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
				Name: fmt.Sprintf("%s-%s", "wait", constants.KarmadaAPIserverComponent),
				Run:  runWaitKarmadaAPIServer,
			},
			{
				Name: constants.KarmadaAggregratedAPIServerComponent,
				Run:  runKarmadaAggregratedAPIServer,
			},
			{
				Name: fmt.Sprintf("%s-%s", "wait", constants.KarmadaAggregratedAPIServerComponent),
				Run:  runWaitKarmadaAggregratedAPIServer,
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

func runKarmadaAggregratedAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("KarmadaAggregratedAPIServer task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaAggregratedAPIServer == nil {
		klog.V(2).InfoS("[KarmadaAggregratedAPIServer] Skip install karmada-aggregrated-apiserver component")
		return nil
	}

	err := apiserver.EnsureKarmadaAggregratedAPIServer(data.RemoteClient(), cfg, data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install karmada aggregrated apiserver, err: %w", err)
	}

	klog.V(2).InfoS("[KarmadaAggregratedApiserve] Successfully installed karmada-aggregrated-apiserver component", "karmada", klog.KObj(data))
	return nil
}

func runWaitKarmadaAggregratedAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-KarmadaAggregratedAPIServer task invoked with an invalid data struct")
	}

	waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConifg(), data.RemoteClient(), time.Second*30)

	err := waiter.WaitForSomePods(karmadaAggregatedAPIServerLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("waiting for karmada-apiserver to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[wait-KarmadaAggregratedAPIServer] the karmada-aggregated-apiserver is ready", "karmada", klog.KObj(data))
	return nil
}
