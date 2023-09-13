package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewRemoveComponentTask init a remove karmada components task
func NewRemoveComponentTask() workflow.Task {
	return workflow.Task{
		Name:        "remove-component",
		Run:         runRemoveComponent,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newRemoveComponentWithServiceSubTask(constants.KarmadaMetricsAdapterComponent, util.KarmadaMetricsAdapterName),
			newRemoveComponentSubTask(constants.KarmadaDeschedulerComponent, util.KarmadaDeschedulerName),
			newRemoveComponentSubTask(constants.KarmadaSchedulerComponent, util.KarmadaSchedulerName),
			newRemoveComponentSubTask(constants.KarmadaControllerManagerComponent, util.KarmadaControllerManagerName),
			newRemoveComponentSubTask(constants.KubeControllerManagerComponent, util.KubeControllerManagerName),
			newRemoveComponentWithServiceSubTask(constants.KarmadaWebhookComponent, util.KarmadaWebhookName),
			newRemoveComponentWithServiceSubTask(constants.KarmadaAggregatedAPIServerComponent, util.KarmadaAggregatedAPIServerName),
			newRemoveComponentWithServiceSubTask(constants.KarmadaAPIserverComponent, util.KarmadaAPIServerName),
			{
				Name: "remove-etcd",
				Run:  runRemoveEtcd,
			},
		},
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

		// Although we found the workload by name, we cannot be sure that the
		// workload was created by the karmada operator. if the workload exists the
		// label "app.kubernetes.io/managed-by": "karmada-operator", we think it
		// must be created by karmada operator.
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
				return fmt.Errorf("failed to cleanup serivce of component %s, err: %w", component, err)
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
