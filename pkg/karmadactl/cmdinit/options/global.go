/*
Copyright 2021 The Karmada Authors.

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

package options

const (
	// EtcdCaCertAndKeyName etcd ca certificate key name
	EtcdCaCertAndKeyName = "etcd-ca"
	// EtcdServerCertAndKeyName etcd server certificate key name
	EtcdServerCertAndKeyName = "etcd-server"
	// EtcdClientCertAndKeyName etcd client certificate key name
	EtcdClientCertAndKeyName = "etcd-client"
	// KarmadaCertAndKeyName karmada certificate key name
	KarmadaCertAndKeyName = "karmada"
	// ApiserverCertAndKeyName karmada apiserver certificate key name
	ApiserverCertAndKeyName = "apiserver"
	// FrontProxyCaCertAndKeyName front-proxy-client  certificate key name
	FrontProxyCaCertAndKeyName = "front-proxy-ca"
	// FrontProxyClientCertAndKeyName front-proxy-client  certificate key name
	FrontProxyClientCertAndKeyName = "front-proxy-client"
	// ClusterName karmada cluster name
	ClusterName = "karmada-apiserver"
	// UserName karmada cluster user name
	UserName = "karmada-admin"
	// KarmadaKubeConfigName karmada kubeconfig name
	KarmadaKubeConfigName = "karmada-apiserver.config"
	// WaitComponentReadyTimeout wait component ready time
	WaitComponentReadyTimeout = 120
)

const (
	// DefaultKubeImageTag is the default tag for the Kubernetes image used by the Karmada control plane.
	DefaultKubeImageTag = "v1.30.4"
	// DefaultKarmadaKubeNamespace is the default namespace in which the Karmada control plane components will be deployed.
	DefaultKarmadaKubeNamespace = "karmada-system"
	// DefaultKarmadaAPIServerNodePort is the default node port used by the Karmada API server.
	DefaultKarmadaAPIServerNodePort = 32443
	// DefaultEtcdStorageMode is the default storage mode for etcd data. The default mode is 'hostPath'.
	DefaultEtcdStorageMode = "hostPath"
	// DefaultEtcdHostDataPath is the default file system path where etcd data is stored when the storage mode is 'hostPath'.
	DefaultEtcdHostDataPath = "/var/lib/karmada-etcd"
	// DefaultKarmadaDataPath is the default directory where Karmada stores its configuration, certificates, and other data.
	DefaultKarmadaDataPath = "/etc/karmada"
	// DefaultKarmadaPkiPath is the default directory where Karmada stores its PKI (Public Key Infrastructure) files.
	DefaultKarmadaPkiPath = "/etc/karmada/pki"
)
