package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/controlplane/etcd"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewEtcdTask init a etcd task to install etcd component
func NewEtcdTask() workflow.Task {
	return workflow.Task{
		Name: "Etcd",
		Run:  runEtcd,
	}
}

func runEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("etcd task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[etcd] Running etcd task", "karmada", klog.KObj(data))

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

	klog.V(2).InfoS("[etcd] Successfully installed etcd component", "karmada", klog.KObj(data))
	return nil
}
