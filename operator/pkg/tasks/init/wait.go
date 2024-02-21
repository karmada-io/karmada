/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
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
	karmadaMetricAdapterLabels       = labels.Set{"karmada-app": constants.KarmadaMetricsAdapter}

	KarmadaDeschedulerLabels = labels.Set{"karmada-app": constants.KarmadaDescheduler}
	KarmadaSearchLabels      = labels.Set{"karmada-app": constants.KarmadaSearch}
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

// NewWaitAddonTask init wait-addon task
func NewWaitAddonTask() workflow.Task {
	return workflow.Task{
		Name:        "wait-addons",
		Run:         runWaitAddon,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newWaitAddonSubTask(constants.KarmadaDeschedulerComponent, KarmadaDeschedulerLabels),
			newWaitAddonSubTask(constants.KarmadaSearchComponent, KarmadaSearchLabels),
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

func runWaitAddon(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-addon task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[wait-addon] Running wait-addon task", "karmada", klog.KObj(data))
	return nil
}

func newWaitControlPlaneSubTask(component string, ls labels.Set) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runWaitControlPlaneSubTask(component, ls),
	}
}

func runWaitControlPlaneSubTask(component string, ls labels.Set) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("wait-controlPlane task invoked with an invalid data struct")
		}

		waiter := apiclient.NewKarmadaWaiter(nil, data.RemoteClient(), componentBeReadyTimeout)
		if err := waiter.WaitForSomePods(ls.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[wait-ControlPlane] component status is ready", "component", component, "karmada", klog.KObj(data))
		return nil
	}
}

func newWaitAddonSubTask(component string, ls labels.Set) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runWaitAddonSubTask(component, ls),
	}
}

func runWaitAddonSubTask(component string, ls labels.Set) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("wait-addons task invoked with an invalid data struct")
		}
		// Addon is not installed by default, so if it is not installed, just skip it.
		if !isAddonInstalled(component, data.Components()) {
			klog.V(2).Infof("skip wait for addon %s/%s", data.GetNamespace(), component)
			return nil
		}
		waiter := apiclient.NewKarmadaWaiter(nil, data.RemoteClient(), componentBeReadyTimeout)
		if err := waiter.WaitForSomePods(ls.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[wait-addons] component status is ready", "component", component, "karmada", klog.KObj(data))
		return nil
	}
}

func isAddonInstalled(name string, components *v1alpha1.KarmadaComponents) bool {
	switch name {
	case constants.KarmadaDeschedulerComponent:
		return components.KarmadaDescheduler != nil
	case constants.KarmadaSearchComponent:
		return components.KarmadaSearch != nil
	default:
		return false
	}
}
