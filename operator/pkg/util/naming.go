package util

import (
	"fmt"
	"strings"
)

// Namefunc defines a function to generate resource name according to karmada resource name.
type Namefunc func(karmada string) string

// AdminKubeconfigSercretName returns secret name of karmada-admin kubeconfig
func AdminKubeconfigSercretName(karmada string) string {
	return generateResourceName(karmada, "admin-config")
}

// KarmadaCertSecretName returns secret name of karmada certs
func KarmadaCertSecretName(karmada string) string {
	return generateResourceName(karmada, "cert")
}

// EtcdCertSecretName returns secret name of etcd cert
func EtcdCertSecretName(karmada string) string {
	return generateResourceName(karmada, "etcd-cert")
}

// WebhookCertSecretName returns secret name of karmada-webhook cert
func WebhookCertSecretName(karmada string) string {
	return generateResourceName(karmada, "webhook-cert")
}

// KarmadaAPIServerName returns secret name of karmada-apiserver
func KarmadaAPIServerName(karmada string) string {
	return generateResourceName(karmada, "apiserver")
}

// KarmadaAggregatedAPIServerName returns secret name of karmada-aggregated-apiserver
func KarmadaAggregatedAPIServerName(karmada string) string {
	return generateResourceName(karmada, "aggregated-apiserver")
}

// KarmadaEtcdName returns name of karmada-etcd
func KarmadaEtcdName(karmada string) string {
	return generateResourceName(karmada, "etcd")
}

// KarmadaEtcdClientName returns name of karmada-etcd client
func KarmadaEtcdClientName(karmada string) string {
	return generateResourceName(karmada, "etcd-client")
}

// KubeControllerManagerName returns name of kube-controller-manager
func KubeControllerManagerName(karmada string) string {
	return generateResourceName(karmada, "kube-controller-manager")
}

// KarmadaControllerManagerName returns name of karmada-controller-manager
func KarmadaControllerManagerName(karmada string) string {
	return generateResourceName(karmada, "controller-manager")
}

// KarmadaSchedulerName returns name of karmada-scheduler
func KarmadaSchedulerName(karmada string) string {
	return generateResourceName(karmada, "scheduler")
}

// KarmadaWebhookName returns name of karmada-webhook
func KarmadaWebhookName(karmada string) string {
	return generateResourceName(karmada, "webhook")
}

// KarmadaDeschedulerName returns name of karmada-descheduler
func KarmadaDeschedulerName(karmada string) string {
	return generateResourceName(karmada, "descheduler")
}

func generateResourceName(karmada, suffix string) string {
	if strings.Contains(karmada, "karmada") {
		return fmt.Sprintf("%s-%s", karmada, suffix)
	}

	return fmt.Sprintf("%s-karmada-%s", karmada, suffix)
}
