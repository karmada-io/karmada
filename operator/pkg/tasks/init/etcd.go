package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/controlplane/etcd"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewEtcdTask init a etcd task to install etcd component
func NewEtcdTask() workflow.Task {
	return workflow.Task{
		Name:        "Etcd",
		Run:         runEtcd,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-etcd",
				Run:  runDeployEtcd,
			},
			{
				Name: "wait-etcd",
				Run:  runWaitEtcd,
			},
		},
	}
}

func runEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("etcd task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[etcd] Running etcd task", "karmada", klog.KObj(data))
	return nil
}

func runDeployEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("deploy-etcd task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.Etcd.External != nil {
		klog.V(2).InfoS("[etcd] use external etcd, skip install etcd job", "karmada", data.GetName())
		return nil
	}

	if cfg.Etcd.Local == nil {
		return errors.New("unexpect empty etcd local configuration")
	}

	err := etcd.EnsureKarmadaEtcd(data.RemoteClient(), cfg.Etcd.Local, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install etcd component, err: %w", err)
	}

	klog.V(2).InfoS("[deploy-etcd] Successfully installed etcd component", "karmada", klog.KObj(data))
	return nil
}

func runWaitEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-etcd task invoked with an invalid data struct")
	}

	waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConfig(), data.RemoteClient(), componentBeReadyTimeout)

	// wait etcd, karmada apiserver and aggregated apiserver to ready
	// as long as a replica of pod is ready, we consider the service available.
	if err := waiter.WaitForSomePods(etcdLabels.String(), data.GetNamespace(), 1); err != nil {
		return fmt.Errorf("waiting for karmada-etcd to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[wait-etcd] the etcd pods is ready", "karmada", klog.KObj(data))
	return nil
}
