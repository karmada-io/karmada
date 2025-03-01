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

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	// The timeout for wait each component be ready
	// It includes the time for pulling the component image.
	componentBeReadyTimeout = 120 * time.Second

	// failureThreshold represents the maximum number of retries allowed for
	// waiting for a component to be ready. If the threshold is exceeded,
	// the process will stop and return an error.
	failureThreshold = 3

	etcdLabels                       = labels.Set{constants.AppNameLabel: constants.Etcd}
	karmadaApiserverLabels           = labels.Set{constants.AppNameLabel: constants.KarmadaAPIServer}
	karmadaAggregatedAPIServerLabels = labels.Set{constants.AppNameLabel: names.KarmadaAggregatedAPIServerComponentName}
	kubeControllerManagerLabels      = labels.Set{constants.AppNameLabel: constants.KubeControllerManager}
	karmadaControllerManagerLabels   = labels.Set{constants.AppNameLabel: names.KarmadaControllerManagerComponentName}
	karmadaSchedulerLabels           = labels.Set{constants.AppNameLabel: names.KarmadaSchedulerComponentName}
	karmadaWebhookLabels             = labels.Set{constants.AppNameLabel: names.KarmadaWebhookComponentName}
	karmadaMetricAdapterLabels       = labels.Set{constants.AppNameLabel: names.KarmadaMetricsAdapterComponentName}
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
	if err := apiclient.TryRunCommand(waiter.WaitForAPI, failureThreshold); err != nil {
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

		ls[constants.AppInstanceLabel] = data.GetName()
		waiter := apiclient.NewKarmadaWaiter(nil, data.RemoteClient(), componentBeReadyTimeout)
		if err := waiter.WaitForSomePods(ls.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[wait-ControlPlane] component status is ready", "component", component, "karmada", klog.KObj(data))
		return nil
	}
}
