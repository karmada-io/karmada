package tasks

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/controlplane"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/metricsadapter"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/webhook"
	"github.com/karmada-io/karmada/operator/pkg/karmadaresource/apiservice"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
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
			newComponentSubTask(constants.KarmadaDeschedulerComponent),
			newKarmadaMetricsAdapterSubTask(),
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
			data.FeatureGates(),
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
		return errors.New("KarmadaWebhook task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaWebhook == nil {
		return errors.New("skip install karmada webhook")
	}

	err := webhook.EnsureKarmadaWebhook(
		data.RemoteClient(),
		cfg.KarmadaWebhook,
		data.GetName(),
		data.GetNamespace(),
		data.FeatureGates(),
	)
	if err != nil {
		return fmt.Errorf("failed to apply karmada webhook, err: %w", err)
	}

	klog.V(2).InfoS("[KarmadaWebhook] Successfully applied karmada webhook component", "karmada", klog.KObj(data))
	return nil
}

func newKarmadaMetricsAdapterSubTask() workflow.Task {
	return workflow.Task{
		Name:        constants.KarmadaMetricsAdapterComponent,
		Run:         runKarmadaMetricsAdapter,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "DeployMetricAdapter",
				Run:  runDeployMetricAdapter,
			},
			{
				Name: "DeployMetricAdapterAPIService",
				Run:  runDeployMetricAdapterAPIService,
			},
		},
	}
}

func runKarmadaMetricsAdapter(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("karmadaMetricsAdapter task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[karmadaMetricsAdapter] Running karmadaMetricsAdapter task", "karmada", klog.KObj(data))
	return nil
}

func runDeployMetricAdapter(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("DeployMetricAdapter task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaMetricsAdapter == nil {
		klog.V(2).InfoS("[karmadaMetricsAdapter] Skip install karmada-metrics-adapter component")
		return nil
	}

	err := metricsadapter.EnsureKarmadaMetricAdapter(
		data.RemoteClient(),
		cfg.KarmadaMetricsAdapter,
		data.GetName(),
		data.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("failed to apply karmada-metrics-adapter, err: %w", err)
	}

	klog.V(2).InfoS("[DeployMetricAdapter] Successfully applied karmada-metrics-adapter component", "karmada", klog.KObj(data))

	if *cfg.KarmadaMetricsAdapter.Replicas != 0 {
		waiter := apiclient.NewKarmadaWaiter(data.ControlplaneConfig(), data.RemoteClient(), time.Second*30)
		if err = waiter.WaitForSomePods(karmadaMetricAdapterLabels.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for karmada-metrics-adapter to ready timeout, err: %w", err)
		}

		klog.V(2).InfoS("[DeployMetricAdapter] the karmada-metrics-adapter is ready", "karmada", klog.KObj(data))
	}

	return nil
}

func runDeployMetricAdapterAPIService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("DeployMetricAdapterAPIService task invoked with an invalid data struct")
	}

	cfg := data.Components()
	if cfg.KarmadaMetricsAdapter == nil {
		klog.V(2).InfoS("[karmadaMetricsAdapter] Skip install karmada-metrics-adapter APIService")
		return nil
	}

	config := data.ControlplaneConfig()
	client, err := apiclient.NewAPIRegistrationClient(config)
	if err != nil {
		return err
	}

	cert := data.GetCert(constants.CaCertAndKeyName)
	if len(cert.CertData()) == 0 {
		return errors.New("unexpected empty ca cert data for aggregatedAPIService")
	}
	caBase64 := base64.StdEncoding.EncodeToString(cert.CertData())

	err = apiservice.EnsureMetricsAdapterAPIService(client, data.KarmadaClient(), data.GetName(), constants.KarmadaSystemNamespace, data.GetName(), data.GetNamespace(), caBase64)
	if err != nil {
		return fmt.Errorf("failed to apply karmada-metrics-adapter APIService resource to karmada controlplane, err: %w", err)
	}

	if *cfg.KarmadaMetricsAdapter.Replicas != 0 {
		waiter := apiclient.NewKarmadaWaiter(config, nil, time.Second*20)
		for _, gv := range constants.KarmadaMetricsAdapterAPIServices {
			apiServiceName := fmt.Sprintf("%s.%s", gv.Version, gv.Group)

			if err := waiter.WaitForAPIService(apiServiceName); err != nil {
				return fmt.Errorf("the APIService %s is unhealthy, err: %w", apiServiceName, err)
			}
		}

		klog.V(2).InfoS("[DeployMetricAdapterAPIService] all karmada-metrics-adapter APIServices status is ready ", "karmada", klog.KObj(data))
	}

	return nil
}
