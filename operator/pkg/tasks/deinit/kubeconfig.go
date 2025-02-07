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

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewCleanupKubeconfigTask init a task to cleanup kubeconfig
func NewCleanupKubeconfigTask() workflow.Task {
	return workflow.Task{
		Name: "cleanup-kubeconfig",
		Run:  runCleanupKubeconfig,
	}
}

func runCleanupKubeconfig(r workflow.RunData) error {
	data, ok := r.(DeInitData)
	if !ok {
		return errors.New("cleanup-karmada-config task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[cleanup-karmada-config] Running cleanup-karmada-config task", "karmada", klog.KObj(data))

	secretNames := generateComponentKubeconfigSecretNames(data)

	for _, secretName := range secretNames {
		err := apiclient.DeleteSecretIfHasLabels(
			data.RemoteClient(),
			secretName,
			data.GetNamespace(),
			constants.KarmadaOperatorLabel,
		)
		if err != nil {
			return fmt.Errorf("failed to cleanup karmada-config secret '%s', err: %w", secretName, err)
		}
	}

	return nil
}

func generateComponentKubeconfigSecretNames(data DeInitData) []string {
	secretNames := []string{
		util.AdminKarmadaConfigSecretName(data.GetName()),
		util.ComponentKarmadaConfigSecretName(util.KarmadaAggregatedAPIServerName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KarmadaControllerManagerName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KubeControllerManagerName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KarmadaSchedulerName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KarmadaDeschedulerName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KarmadaMetricsAdapterName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KarmadaSearchName(data.GetName())),
		util.ComponentKarmadaConfigSecretName(util.KarmadaWebhookName(data.GetName())),
	}

	return secretNames
}
