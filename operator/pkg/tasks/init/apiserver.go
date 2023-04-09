package tasks

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/apiserver"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewKarmadaApiserverTask inits a task to install karmada-apiserver component
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
				Name: fmt.Sprintf("%s-%s", "wait", constants.KarmadaAPIserverComponent),
				Run:  runWaitKarmadaAPIServer,
			},
		},
	}
}

// NewKarmadaAggregatedApiserverTask inits a task to install karmada-aggregated-apiserver component
func NewKarmadaAggregatedApiserverTask() workflow.Task {
	return workflow.Task{
		Name:        "aggregated-apiserver",
		Run:         runAggregatedApiserver,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: constants.KarmadaAggregatedAPIServerComponent,
				Run:  runKarmadaAggregatedAPIServer,
			},
			{
				Name: fmt.Sprintf("%s-%s", "wait", constants.KarmadaAggregatedAPIServerComponent),
				Run:  runWaitKarmadaAggregatedAPIServer,
			},
		},
	}
}

func runAggregatedApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("aggregated-apiserver task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[aggregated-apiserver] Running aggregated apiserver task", "karmada", klog.KObj(data))
	return nil
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
		return errors.New("KarmadaApiserver task invoked with an invalid data struct")
	}

	cfg := data.Components()

	if cfg.KarmadaAPIServer == nil {
		klog.V(2).InfoS("[KarmadaApiserver] Skip install karmada-apiserver component")
		return nil
	}

	err := apiserver.EnsureKarmadaAPIServer(data.RemoteClient(), cfg, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install karmada apiserver component, err: %w", err)
	}

	klog.V(2).InfoS("[KarmadaApiserver] Successfully installed karmada-apiserver component", "karmada", klog.KObj(data))
	return nil
}

func runWaitKarmadaAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-KarmadaAPIServer task invoked with an invalid data struct")
	}

	waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConifg(), data.RemoteClient(), time.Second*30)

	err := waiter.WaitForSomePods(karmadaApiserverLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("waiting for karmada-apiserver to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[wait-KarmadaAPIServer] the karmada-apiserver is ready", "karmada", klog.KObj(data))
	return nil
}

func runKarmadaAggregatedAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("KarmadaAggregatedAPIServer task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaAggregatedAPIServer == nil {
		klog.V(2).InfoS("[KarmadaAggregatedAPIServer] Skip install karmada-aggregated-apiserver component")
		return nil
	}

	err := apiserver.EnsureKarmadaAggregatedAPIServer(data.RemoteClient(), cfg, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install karmada aggregated apiserver, err: %w", err)
	}

	klog.V(2).InfoS("[KarmadaAggregatedApiserver] Successfully installed karmada-aggregated-apiserver component", "karmada", klog.KObj(data))
	return nil
}

func runWaitKarmadaAggregatedAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-KarmadaAggregatedAPIServer task invoked with an invalid data struct")
	}

	waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConifg(), data.RemoteClient(), time.Second*30)

	err := waiter.WaitForSomePods(karmadaAggregatedAPIServerLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("waiting for karmada-apiserver to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[wait-KarmadaAggregatedAPIServer] the karmada-aggregated-apiserver is ready", "karmada", klog.KObj(data))
	return nil
}
