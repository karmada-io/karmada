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

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewRemoveComponentTask init a remove karmada components task
func NewRemoveComponentTask(karmada *v1alpha1.Karmada) workflow.Task {
	workflowTasks := []workflow.Task{
		newRemoveComponentWithServiceSubTask(constants.KarmadaMetricsAdapterComponent, util.KarmadaMetricsAdapterName),
		newRemoveComponentSubTask(constants.KarmadaDeschedulerComponent, util.KarmadaDeschedulerName),
		newRemoveComponentSubTask(constants.KarmadaSchedulerComponent, util.KarmadaSchedulerName),
		newRemoveComponentSubTask(constants.KarmadaControllerManagerComponent, util.KarmadaControllerManagerName),
		newRemoveComponentSubTask(constants.KubeControllerManagerComponent, util.KubeControllerManagerName),
		newRemoveComponentWithServiceSubTask(constants.KarmadaWebhookComponent, util.KarmadaWebhookName),
		newRemoveComponentWithServiceSubTask(constants.KarmadaSearchComponent, util.KarmadaSearchName),
		newRemoveComponentWithServiceSubTask(constants.KarmadaAggregatedAPIServerComponent, util.KarmadaAggregatedAPIServerName),
		newRemoveComponentWithServiceSubTask(constants.KarmadaAPIserverComponent, util.KarmadaAPIServerName),
	}
	// Required only if local etcd is configured
	if karmada.Spec.Components.Etcd.Local != nil {
		removeEtcdTask := workflow.Task{
			Name: "remove-etcd",
			Run:  runRemoveEtcd,
		}
		workflowTasks = append(workflowTasks, removeEtcdTask)
	}
	return workflow.Task{
		Name:        "remove-component",
		Run:         runRemoveComponent,
		RunSubTasks: true,
		Tasks:       workflowTasks,
	}
}

func runRemoveComponent(r workflow.RunData) error {
	data, ok := r.(DeInitData)
	if !ok {
		return errors.New("remove-component task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[remove-component] Running remove-component task", "karmada", klog.KObj(data))
	return nil
}

func newRemoveComponentSubTask(component string, workloadNameFunc util.Namefunc) workflow.Task {
	return workflow.Task{
		Name: fmt.Sprintf("remove-%s", component),
		Run:  runRemoveComponentSubTask(component, workloadNameFunc, false),
	}
}

func newRemoveComponentWithServiceSubTask(component string, workloadNameFunc util.Namefunc) workflow.Task {
	return workflow.Task{
		Name: fmt.Sprintf("remove-%s", component),
		Run:  runRemoveComponentSubTask(component, workloadNameFunc, true),
	}
}

func runRemoveComponentSubTask(component string, workloadNameFunc util.Namefunc, hasService bool) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(DeInitData)
		if !ok {
			return fmt.Errorf("remove-%s task invoked with an invalid data struct", component)
		}

		// Even though we found the workload by name, we can't be certain that it was
		// created by the Karmada operator. If the workload has the label
		// "app.kubernetes.io/managed-by": "karmada-operator", we can assume it was
		// created by the Karmada operator.
		err := apiclient.DeleteDeploymentIfHasLabels(
			data.RemoteClient(),
			workloadNameFunc(data.GetName()),
			data.GetNamespace(),
			constants.KarmadaOperatorLabel,
		)
		if err != nil {
			return fmt.Errorf("failed to remove component %s, err: %w", component, err)
		}

		if hasService {
			err := apiclient.DeleteServiceIfHasLabels(
				data.RemoteClient(),
				workloadNameFunc(data.GetName()),
				data.GetNamespace(),
				constants.KarmadaOperatorLabel,
			)
			if err != nil {
				return fmt.Errorf("failed to cleanup service of component %s, err: %w", component, err)
			}
		}

		return nil
	}
}

func runRemoveEtcd(r workflow.RunData) error {
	data, ok := r.(DeInitData)
	if !ok {
		return fmt.Errorf("remove-etcd task invoked with an invalid data struct")
	}

	err := apiclient.DeleteStatefulSetIfHasLabels(
		data.RemoteClient(),
		util.KarmadaEtcdName(data.GetName()),
		data.GetNamespace(),
		constants.KarmadaOperatorLabel,
	)
	if err != nil {
		return fmt.Errorf("failed to remove etcd, err: %w", err)
	}

	// cleanup these service of etcd.
	services := []string{util.KarmadaEtcdName(data.GetName()), util.KarmadaEtcdClientName(data.GetName())}
	for _, service := range services {
		err := apiclient.DeleteServiceIfHasLabels(
			data.RemoteClient(),
			service,
			data.GetNamespace(),
			constants.KarmadaOperatorLabel,
		)
		if err != nil {
			return fmt.Errorf("failed to cleanup etcd service %s, err: %w", service, err)
		}
	}

	return nil
}
