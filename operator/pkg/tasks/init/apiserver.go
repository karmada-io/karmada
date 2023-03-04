package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/apiserver"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewKarmadaApiserverTask init apiserver task to install karmada apiserver and
// karmada aggregated apiserver component
func NewKarmadaApiserverTask() workflow.Task {
	return workflow.Task{
		Name:        "apiserver",
		Run:         runApiserver,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: constants.KarmadaAPIserverComponent,
				Run:  runKarmadaAPIServer,
			},
			{
				Name: constants.KarmadaAggregratedAPIServerComponent,
				Run:  runKarmadaAggregratedAPIServer,
			},
		},
	}
}

func runApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("apiserver task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[apiserver] Running apiserver task", "karmada", klog.KObj(data))
	return nil
}

func runKarmadaAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("karmadaApiserver task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaAPIServer == nil {
		klog.V(2).InfoS("[karmadaApiserver] Skip install karmada-apiserver component")
		return nil
	}

	err := apiserver.EnsureKarmadaAPIServer(data.RemoteClient(), cfg, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install karmada apiserver component, err: %w", err)
	}

	klog.V(2).InfoS("[karmadaApiserver] Successfully installed apiserver component", "karmada", klog.KObj(data))
	return nil
}

func runKarmadaAggregratedAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("karmadaAggregratedApiServer task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaAggregratedAPIServer == nil {
		klog.V(2).InfoS("[KarmadaAggregratedApiServer] Skip install karmada-aggregrated-apiserver component")
		return nil
	}

	err := apiserver.EnsureKarmadaAggregratedAPIServer(data.RemoteClient(), cfg, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install karmada aggregrated apiserver, err: %w", err)
	}

	klog.V(2).InfoS("[KarmadaAggregratedApiServer] Successfully installed karmada apiserve component", "karmada", klog.KObj(data))
	return nil
}
