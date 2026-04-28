/*
Copyright 2019 The Kubernetes Authors.

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

package secret

const (
	// KubeconfigDataName is the key used to store a Kubeconfig in the secret's data field.
	KubeconfigDataName = "value"

	// TLSKeyDataName is the key used to store a TLS private key in the secret's data field.
	TLSKeyDataName = "tls.key"

	// TLSCrtDataName is the key used to store a TLS certificate in the secret's data field.
	TLSCrtDataName = "tls.crt"
)

// Purpose is the name to append to the secret generated for a cluster.
type Purpose string

const (
	// Kubeconfig is the secret name suffix storing the Cluster Kubeconfig.
	Kubeconfig = Purpose("kubeconfig")

	// ClusterCA is the secret name suffix for APIServer CA.
	ClusterCA = Purpose("ca")

	// EtcdCA is the secret name suffix for the Etcd CA.
	EtcdCA = Purpose("etcd")

	// ServiceAccount is the secret name suffix for the Service Account keys.
	ServiceAccount = Purpose("sa")

	// FrontProxyCA is the secret name suffix for Front Proxy CA.
	FrontProxyCA = Purpose("proxy")

	// APIServerEtcdClient is the secret name of user-supplied secret containing the apiserver-etcd-client key/cert.
	APIServerEtcdClient = Purpose("apiserver-etcd-client")
)

var (
	// allSecretPurposes defines a lists with all the secret suffix used by Cluster API.
	allSecretPurposes = []Purpose{Kubeconfig, ClusterCA, EtcdCA, ServiceAccount, FrontProxyCA, APIServerEtcdClient}
)
