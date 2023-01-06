package util

import (
	"fmt"
	"strings"
)

// AdminKubeconfigSercretName return a secret name of karmada admin kubeconfig
func AdminKubeconfigSercretName(name string) string {
	return generateResourceName(name, "admin-config")
}

// KarmadaCertSecretName return a secret name of karmada certs
func KarmadaCertSecretName(name string) string {
	return generateResourceName(name, "cert")
}

// EtcdCertSecretName return a secret name of etcd cert
func EtcdCertSecretName(name string) string {
	return generateResourceName(name, "etcd-cert")
}

// WebhookCertSecretName return secret name of karmada webhook cert
func WebhookCertSecretName(name string) string {
	return generateResourceName(name, "webhook-cert")
}

// KarmadaAPIServerName return secret name of karmada apiserver
func KarmadaAPIServerName(name string) string {
	return generateResourceName(name, "apiserver")
}

// KarmadaAggratedAPIServerName return secret name of karmada aggregated apiserver
func KarmadaAggratedAPIServerName(name string) string {
	return generateResourceName(name, "aggregated-apiserver")
}

// KarmadaEtcdName return karmada etcd name
func KarmadaEtcdName(name string) string {
	return generateResourceName(name, "etcd")
}

// KarmadaEtcdClientName return karmada etcd client name
func KarmadaEtcdClientName(name string) string {
	return generateResourceName(name, "etcd-client")
}

// KubeControllerManagerName return name of kube controller manager name of karmada
func KubeControllerManagerName(name string) string {
	return generateResourceName(name, "kube-controller-manager")
}

// KarmadaControllerManagerName return karmada controller manager name
func KarmadaControllerManagerName(name string) string {
	return generateResourceName(name, "controller-manager")
}

// KarmadaSchedulerName return karmada scheduler name
func KarmadaSchedulerName(name string) string {
	return generateResourceName(name, "scheduler")
}

// KarmadaWebhookName return karmada webhook name
func KarmadaWebhookName(name string) string {
	return generateResourceName(name, "webhook")
}

func generateResourceName(name, suffix string) string {
	if strings.Contains(name, "karmada") {
		return fmt.Sprintf("%s-%s", name, suffix)
	}

	return fmt.Sprintf("%s-karmada-%s", name, suffix)
}
