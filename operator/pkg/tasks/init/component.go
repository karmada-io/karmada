package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/controlplane"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/webhook"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewComponentTask init a components task
func NewComponentTask() workflow.Task {
	return workflow.Task{
		Name:        "components",
		Run:         runComponents,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newComponentSubTask(constants.KubeControllerManagerComponent),
			newComponentSubTask(constants.KarmadaControllerManagerComponent),
			newComponentSubTask(constants.KarmadaSchedulerComponent),
			{
				Name: "KarmadaWebhook",
				Run:  runKarmadaWebhook,
			},
		},
	}
}

func runComponents(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("components task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[components] Running components task", "karmada", klog.KObj(data))
	return nil
}

func newComponentSubTask(component string) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runComponentSubTask(component),
	}
}

func runComponentSubTask(component string) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("components task invoked with an invalid data struct")
		}

		err := controlplane.EnsureControlPlaneComponent(
			component,
			data.GetName(),
			data.GetNamespace(),
			data.RemoteClient(),
			data.Components(),
		)
		if err != nil {
			return fmt.Errorf("failed to apply component %s, err: %w", component, err)
		}

		klog.V(2).InfoS("[components] Successfully applied component", "component", component, "karmada", klog.KObj(data))
		return nil
	}
}

func runKarmadaWebhook(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("certs task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaWebhook == nil {
		return errors.New("skip install karmada webhook")
	}

	err := webhook.EnsureKarmadaWebhook(data.RemoteClient(), cfg.KarmadaWebhook, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to apply karmada webhook, err: %w", err)
	}

	klog.V(2).InfoS("[KarmadaWebhook] Successfully applied karmada webhook component", "karmada", klog.KObj(data))
	return nil
}
