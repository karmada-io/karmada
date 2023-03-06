package util

import (
	"fmt"
	"strings"
)

// Namefunc defines a function to generate resource name according to karmada resource name.
type Namefunc func(karmada string) string

// AdminKubeconfigSercretName return a secret name of karmada admin kubeconfig
func AdminKubeconfigSercretName(karmada string) string {
	return generateResourceName(karmada, "admin-config")
}

// KarmadaCertSecretName return a secret name of karmada certs
func KarmadaCertSecretName(karmada string) string {
	return generateResourceName(karmada, "cert")
}

// EtcdCertSecretName return a secret name of etcd cert
func EtcdCertSecretName(karmada string) string {
	return generateResourceName(karmada, "etcd-cert")
}

// WebhookCertSecretName return secret name of karmada webhook cert
func WebhookCertSecretName(karmada string) string {
	return generateResourceName(karmada, "webhook-cert")
}

// KarmadaAPIServerName return secret name of karmada apiserver
func KarmadaAPIServerName(karmada string) string {
	return generateResourceName(karmada, "apiserver")
}

// KarmadaAggratedAPIServerName return secret name of karmada aggregated apiserver
func KarmadaAggratedAPIServerName(karmada string) string {
	return generateResourceName(karmada, "aggregated-apiserver")
}

// KarmadaEtcdName return karmada etcd name
func KarmadaEtcdName(karmada string) string {
	return generateResourceName(karmada, "etcd")
}

// KarmadaEtcdClientName return karmada etcd client name
func KarmadaEtcdClientName(karmada string) string {
	return generateResourceName(karmada, "etcd-client")
}

// KubeControllerManagerName return name of kube controller manager name of karmada
func KubeControllerManagerName(karmada string) string {
	return generateResourceName(karmada, "kube-controller-manager")
}

// KarmadaControllerManagerName return karmada controller manager name
func KarmadaControllerManagerName(karmada string) string {
	return generateResourceName(karmada, "controller-manager")
}

// KarmadaSchedulerName return karmada scheduler name
func KarmadaSchedulerName(karmada string) string {
	return generateResourceName(karmada, "scheduler")
}

// KarmadaWebhookName return karmada webhook name
func KarmadaWebhookName(karmada string) string {
	return generateResourceName(karmada, "webhook")
}

func generateResourceName(karmada, suffix string) string {
	if strings.Contains(karmada, "karmada") {
		return fmt.Sprintf("%s-%s", karmada, suffix)
	}

	return fmt.Sprintf("%s-karmada-%s", karmada, suffix)
}
