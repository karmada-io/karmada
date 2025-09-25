/*
Copyright 2025 The Karmada Authors.

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

package cert

// Package cert centralizes TLS-related constants (secret names and key names)
// so they can be shared by cmdinit and future operator implementations.

// Secret names for split-layout TLS materials
// nolint:gosec // These are Kubernetes Secret resource names, not credentials.
const (
	// apiserver
	SecretApiserverServer             = "karmada-apiserver-cert"
	SecretApiserverEtcdClient         = "karmada-apiserver-etcd-client-cert"
	SecretApiserverFrontProxyClient   = "karmada-apiserver-front-proxy-client-cert"
	SecretApiserverServiceAccountKeys = "karmada-apiserver-service-account-key-pair"

	// aggregated apiserver
	SecretAggregatedAPIServerServer     = "karmada-aggregated-apiserver-cert"
	SecretAggregatedAPIServerEtcdClient = "karmada-aggregated-apiserver-etcd-client-cert"

	// kube-controller-manager
	SecretKubeControllerManagerCA     = "kube-controller-manager-ca-cert"
	SecretKubeControllerManagerSAKeys = "kube-controller-manager-service-account-key-pair"

	// scheduler(estimator) clients
	SecretSchedulerEstimatorClient   = "karmada-scheduler-scheduler-estimator-client-cert"
	SecretDeschedulerEstimatorClient = "karmada-descheduler-scheduler-estimator-client-cert"

	// etcd (internal)
	SecretEtcdServer = "etcd-cert"
	SecretEtcdClient = "etcd-etcd-client-cert"

	// webhook serving cert
	SecretWebhook = "karmada-webhook-cert"
)

// PEM key names used inside TLS secrets
const (
	KeyTLSCrt    = "tls.crt"
	KeyTLSKey    = "tls.key"
	KeyCACrt     = "ca.crt"
	KeySAPrivate = "sa.key"
	KeySAPublic  = "sa.pub"
)
