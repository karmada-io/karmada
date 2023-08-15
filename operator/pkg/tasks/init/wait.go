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
	// The timeout for wait each component be ready
	// It includes the time for pulling the component image.
	componentBeReadyTimeout = 120 * time.Second

	etcdLabels                       = labels.Set{"karmada-app": constants.Etcd}
	karmadaApiserverLabels           = labels.Set{"karmada-app": constants.KarmadaAPIServer}
	karmadaAggregatedAPIServerLabels = labels.Set{"karmada-app": constants.KarmadaAggregatedAPIServer}
	kubeControllerManagerLabels      = labels.Set{"karmada-app": constants.KubeControllerManager}
	karmadaControllerManagerLabels   = labels.Set{"karmada-app": constants.KarmadaControllerManager}
	karmadaSchedulerLabels           = labels.Set{"karmada-app": constants.KarmadaScheduler}
	karmadaWebhookLabels             = labels.Set{"karmada-app": constants.KarmadaWebhook}
)

// NewCheckApiserverHealthTask init wait-apiserver task
func NewCheckApiserverHealthTask() workflow.Task {
	return workflow.Task{
		Name: "check-apiserver-health",
		Run:  runWaitApiserver,
	}
}

func runWaitApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return fmt.Errorf("check-apiserver-health task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[check-apiserver-health] Running task", "karmada", klog.KObj(data))

	waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConfig(), data.RemoteClient(), componentBeReadyTimeout)

	// check whether the karmada apiserver is health.
	if err := apiclient.TryRunCommand(waiter.WaitForAPI, 3); err != nil {
		return fmt.Errorf("the karmada apiserver is unhealthy, err: %w", err)
	}
	klog.V(2).InfoS("[check-apiserver-health] the etcd and karmada-apiserver is healthy", "karmada", klog.KObj(data))
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
			newWaitControlPlaneSubTask("KarmadaScheduler", karmadaSchedulerLabels),
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

		waiter := apiclient.NewKarmadaWaiter(nil, data.RemoteClient(), componentBeReadyTimeout)
		if err := waiter.WaitForSomePods(lables.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[wait-ControlPlane] component status is ready", "component", component, "karmada", klog.KObj(data))
		return nil
	}
}
