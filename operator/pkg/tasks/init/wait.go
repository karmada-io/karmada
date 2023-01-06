package tasks

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

var (
	etcdLabels                       = labels.Set{"karmada-app": constants.Etcd}
	karmadaApiserverLabels           = labels.Set{"karmada-app": constants.KarmadaAPIServer}
	karmadaAggregatedAPIServerLabels = labels.Set{"karmada-app": constants.KarmadaAggregatedAPIServer}
	kubeControllerManagerLabels      = labels.Set{"karmada-app": constants.KubeControllerManager}
	karmadaControllerManagerLabels   = labels.Set{"karmada-app": constants.KarmadaControllerManager}
	karmadaSchedulerLablels          = labels.Set{"karmada-app": constants.KarmadaScheduler}
	karmadaWebhookLabels             = labels.Set{"karmada-app": constants.KarmadaWebhook}
)

// NewWaitApiserverTask init wait-apiserver task
func NewWaitApiserverTask() workflow.Task {
	return workflow.Task{
		Name: "wait-apiserver",
		Run:  runWaitApiserver,
	}
}

func runWaitApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return fmt.Errorf("wait-aipserver task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[wait-aipserver] Running task", "karmada", klog.KObj(data))

	waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConifg(), data.RemoteClient(), time.Second*30)

	// wait etcd, karmada apiserver and aggregated apiserver to ready
	// as long as a replica of pod is ready, we consider the service available.
	if err := waiter.WaitForSomePods(etcdLabels.String(), data.GetNamespace(), 1); err != nil {
		return fmt.Errorf("waiting for etcd to ready timeout, err: %w", err)
	}
	if err := waiter.WaitForSomePods(karmadaApiserverLabels.String(), data.GetNamespace(), 1); err != nil {
		return fmt.Errorf("waiting for karmada apiserver to ready timeout, err: %w", err)
	}
	err := waiter.WaitForSomePods(karmadaAggregatedAPIServerLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("waiting for karmada aggregated apiserver to ready timeout, err: %w", err)
	}

	// check whether the karmada apiserver is running and health.
	if err := apiclient.TryRunCommand(waiter.WaitForAPI, 3); err != nil {
		return fmt.Errorf("the karmada apiserver is unhealth, err: %w", err)
	}

	klog.V(2).InfoS("[wait-aipserver] the etcd, karmada apiserver and aggregated apiserver is ready", "karmada", klog.KObj(data))
	return nil
}

// NewWaitControlPlaneTask init wait-controlPlane task
func NewWaitControlPlaneTask() workflow.Task {
	return workflow.Task{
		Name:        "wait-controlPlane",
		Run:         runWaitControlPlane,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newWaitControlPlaneSubTask("KubeControllerManager", kubeControllerManagerLabels),
			newWaitControlPlaneSubTask("KarmadaControllerManager", karmadaControllerManagerLabels),
			newWaitControlPlaneSubTask("KarmadaScheduler", karmadaSchedulerLablels),
			newWaitControlPlaneSubTask("KarmadaWebhook", karmadaWebhookLabels),
		},
	}
}

func runWaitControlPlane(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-controlPlane task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[wait-controlPlane] Running wait-controlPlane task", "karmada", klog.KObj(data))
	return nil
}

func newWaitControlPlaneSubTask(component string, lables labels.Set) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runWaitControlPlaneSubTask(component, lables),
	}
}

func runWaitControlPlaneSubTask(component string, lables labels.Set) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("wait-controlPlane task invoked with an invalid data struct")
		}

		waiter := apiclient.NewKarmadaWaiter(nil, data.RemoteClient(), time.Second*120)
		if err := waiter.WaitForSomePods(lables.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[wait-ControlPlane] component status is ready", "component", component, "karmada", klog.KObj(data))
		return nil
	}
}
