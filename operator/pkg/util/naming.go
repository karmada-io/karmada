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

package util

import (
	"fmt"
	"strings"
)

// Namefunc defines a function that generates a resource name based on the Karmada resource name.
type Namefunc func(karmada string) string

// AdminKarmadaConfigSecretName returns the secret name for the Karmada admin kubeconfig
// for the specified Karmada instance.
func AdminKarmadaConfigSecretName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "admin-config")
}

// ComponentKarmadaConfigSecretName returns the secret name of a Karmada component's kubeconfig
func ComponentKarmadaConfigSecretName(karmadaComponent string) string {
	return fmt.Sprintf("%s-config", karmadaComponent)
}

// KarmadaCertSecretName returns the secret name of Karmada certificate
// for the specified Karmada instance.
func KarmadaCertSecretName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "cert")
}

// EtcdCertSecretName returns the secret name for etcd certificates
// for the specified Karmada instance.
func EtcdCertSecretName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "etcd-cert")
}

// WebhookCertSecretName returns the secret name for Karmada webhook certificates
// for the specified Karmada instance.
func WebhookCertSecretName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "webhook-cert")
}

// KarmadaAPIServerName returns the name of the Karmada API server
// for the specified Karmada instance.
func KarmadaAPIServerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "apiserver")
}

// KarmadaAggregatedAPIServerName returns the name of the Karmada aggregated API server
// for the specified Karmada instance.
func KarmadaAggregatedAPIServerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "aggregated-apiserver")
}

// KarmadaSearchAPIServerName returns the name of the Karmada search API server
// for the specified Karmada instance.
func KarmadaSearchAPIServerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "search")
}

// KarmadaEtcdName returns the name of the Karmada etcd instance
// for the specified Karmada instance.
func KarmadaEtcdName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "etcd")
}

// KarmadaEtcdClientName returns the name of the Karmada etcd client
// for the specified Karmada instance.
func KarmadaEtcdClientName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "etcd-client")
}

// KubeControllerManagerName returns the name of the kube-controller-manager
// for the specified Karmada instance.
func KubeControllerManagerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "kube-controller-manager")
}

// KarmadaControllerManagerName returns the name of the Karmada controller manager
// for the specified Karmada instance.
func KarmadaControllerManagerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "controller-manager")
}

// KarmadaSchedulerName returns the name of the Karmada scheduler
// for the specified Karmada instance.
func KarmadaSchedulerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "scheduler")
}

// KarmadaWebhookName returns the name of the Karmada webhook
// for the specified Karmada instance.
func KarmadaWebhookName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "webhook")
}

// KarmadaDeschedulerName returns the name of the Karmada descheduler
// for the specified Karmada instance.
func KarmadaDeschedulerName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "descheduler")
}

// KarmadaMetricsAdapterName returns the name of the Karmada metrics adapter
// for the specified Karmada instance.
func KarmadaMetricsAdapterName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "metrics-adapter")
}

// KarmadaSearchName returns the name of the Karmada search component
// for the specified Karmada instance.
func KarmadaSearchName(karmadaInstanceName string) string {
	return generateResourceName(karmadaInstanceName, "search")
}

// generateResourceName generates a resource name for a Karmada resource with
// the given suffix. If the provided Karmada instance name already contains "karmada",
// the suffix is appended directly. Otherwise, "karmada" is prefixed before the suffix.
func generateResourceName(karmadaInstanceName, suffix string) string {
	if strings.Contains(karmadaInstanceName, "karmada") {
		return fmt.Sprintf("%s-%s", karmadaInstanceName, suffix)
	}

	return fmt.Sprintf("%s-karmada-%s", karmadaInstanceName, suffix)
}
