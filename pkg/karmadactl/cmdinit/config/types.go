/*
Copyright 2024 The Karmada Authors.

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

package config

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the group name use in this package
const GroupName = "config.karmada.io"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

// KarmadaInitConfig defines the configuration for initializing Karmada
type KarmadaInitConfig struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`

	// Spec defines the desired state for initializing Karmada
	// +optional
	Spec KarmadaInitSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// KarmadaInitSpec is the specification part of KarmadaInitConfig, containing all configurable options
type KarmadaInitSpec struct {
	// Certificates configures the certificate information required by Karmada
	// +optional
	Certificates Certificates `json:"certificates,omitempty" yaml:"certificates,omitempty"`

	// Etcd configures the information of the Etcd cluster
	// +optional
	Etcd Etcd `json:"etcd,omitempty" yaml:"etcd,omitempty"`

	// HostCluster configures the information of the host cluster
	// +optional
	HostCluster HostCluster `json:"hostCluster,omitempty" yaml:"hostCluster,omitempty"`

	// Images configures image-related information
	// +optional
	Images Images `json:"images,omitempty" yaml:"images,omitempty"`

	// Components configures information about Karmada components
	// +optional
	Components KarmadaComponents `json:"components,omitempty" yaml:"components,omitempty"`

	// KarmadaCRDs configures the Karmada CRDs to be installed
	// +optional
	KarmadaCRDs string `json:"karmadaCRDs,omitempty" yaml:"karmadaCRDs,omitempty"`

	// KarmadaDataPath configures the data directory for Karmada
	// +optional
	KarmadaDataPath string `json:"karmadaDataPath,omitempty" yaml:"karmadaDataPath,omitempty"`

	// KarmadaPKIPath configures the PKI directory for Karmada
	// +optional
	KarmadaPKIPath string `json:"karmadaPKIPath,omitempty" yaml:"karmadaPKIPath,omitempty"`

	// WaitComponentReadyTimeout configures the timeout (in seconds) for waiting for components to be ready
	// +optional
	WaitComponentReadyTimeout int `json:"waitComponentReadyTimeout,omitempty" yaml:"waitComponentReadyTimeout,omitempty"`
}

// Certificates defines the configuration related to certificates
type Certificates struct {
	// CACertFile is the path to the root CA certificate file
	// +optional
	CACertFile string `json:"caCertFile,omitempty" yaml:"caCertFile,omitempty"`

	// CAKeyFile is the path to the root CA key file
	// +optional
	CAKeyFile string `json:"caKeyFile,omitempty" yaml:"caKeyFile,omitempty"`

	// ExternalDNS is the list of external DNS names for the certificate
	// +optional
	ExternalDNS []string `json:"externalDNS,omitempty" yaml:"externalDNS,omitempty"`

	// ExternalIP is the list of external IPs for the certificate
	// +optional
	ExternalIP []string `json:"externalIP,omitempty" yaml:"externalIP,omitempty"`

	// ValidityPeriod is the validity period of the certificate
	// +optional
	ValidityPeriod metav1.Duration `json:"validityPeriod,omitempty" yaml:"validityPeriod,omitempty"`
}

// Etcd defines the configuration of the Etcd cluster
type Etcd struct {
	// Local indicates using a local Etcd cluster
	// +optional
	Local *LocalEtcd `json:"local,omitempty" yaml:"local,omitempty"`

	// External indicates using an external Etcd cluster
	// +optional
	External *ExternalEtcd `json:"external,omitempty" yaml:"external,omitempty"`
}

// LocalEtcd defines the configuration of a local Etcd cluster
type LocalEtcd struct {
	// CommonSettings contains common settings like image and resources
	CommonSettings `json:",inline" yaml:",inline"`

	// DataPath is the data storage path for Etcd
	// +optional
	DataPath string `json:"dataPath,omitempty" yaml:"dataPath,omitempty"`

	// InitImage is the image for the Etcd init container
	// +optional
	InitImage Image `json:"initImage,omitempty" yaml:"initImage,omitempty"`

	// NodeSelectorLabels are the node selector labels for the Etcd pods
	// +optional
	NodeSelectorLabels map[string]string `json:"nodeSelectorLabels,omitempty" yaml:"nodeSelectorLabels,omitempty"`

	// PVCSize is the size of the PersistentVolumeClaim for Etcd
	// +optional
	PVCSize string `json:"pvcSize,omitempty" yaml:"pvcSize,omitempty"`

	// StorageMode is the storage mode for Etcd (e.g., emptyDir, hostPath, PVC)
	// +optional
	StorageMode string `json:"storageMode,omitempty" yaml:"storageMode,omitempty"`

	// StorageClassesName is the name of the storage class for the Etcd PVC
	// +optional
	StorageClassesName string `json:"storageClassesName,omitempty" yaml:"storageClassesName,omitempty"`
}

// ExternalEtcd defines the configuration of an external Etcd cluster
type ExternalEtcd struct {
	// Endpoints are the server addresses of the external Etcd cluster
	// +required
	Endpoints []string `json:"endpoints" yaml:"endpoints"`

	// CAFile is the path to the CA certificate for the external Etcd cluster
	// +optional
	CAFile string `json:"caFile,omitempty" yaml:"caFile,omitempty"`

	// CertFile is the path to the client certificate for the external Etcd cluster
	// +optional
	CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`

	// KeyFile is the path to the client key for the external Etcd cluster
	// +optional
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`

	// KeyPrefix is the key prefix used in the external Etcd cluster
	// +optional
	KeyPrefix string `json:"keyPrefix,omitempty" yaml:"keyPrefix,omitempty"`
}

// HostCluster defines the configuration of the host cluster
type HostCluster struct {
	// APIEndpoint is the API server address of the host cluster
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`

	// Kubeconfig is the path to the kubeconfig file for the host cluster
	// +optional
	Kubeconfig string `json:"kubeconfig,omitempty" yaml:"kubeconfig,omitempty"`

	// Context is the context name in the kubeconfig for the host cluster
	// +optional
	Context string `json:"context,omitempty" yaml:"context,omitempty"`

	// Domain is the domain name of the host cluster
	// +optional
	Domain string `json:"domain,omitempty" yaml:"domain,omitempty"`

	// SecretRef refers to the credentials needed to access the host cluster
	// +optional
	SecretRef *LocalSecretReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}

// Images defines the configuration related to images
type Images struct {
	// ImagePullPolicy is the pull policy for images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`

	// ImagePullSecrets are the secrets used for pulling images
	// +optional
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`

	// KubeImageMirrorCountry is the country code for the Kubernetes image mirror
	// +optional
	KubeImageMirrorCountry string `json:"kubeImageMirrorCountry,omitempty" yaml:"kubeImageMirrorCountry,omitempty"`

	// KubeImageRegistry is the registry for Kubernetes images
	// +optional
	KubeImageRegistry string `json:"kubeImageRegistry,omitempty" yaml:"kubeImageRegistry,omitempty"`

	// KubeImageTag is the tag for Kubernetes images
	// +optional
	KubeImageTag string `json:"kubeImageTag,omitempty" yaml:"kubeImageTag,omitempty"`

	// PrivateRegistry is the private image registry
	// +optional
	PrivateRegistry *ImageRegistry `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
}

// KarmadaComponents defines the configuration for all Karmada components
type KarmadaComponents struct {
	// KarmadaAPIServer is the configuration for the Karmada API Server
	// +optional
	KarmadaAPIServer *KarmadaAPIServer `json:"karmadaAPIServer,omitempty" yaml:"karmadaAPIServer,omitempty"`

	// KarmadaAggregatedAPIServer is the configuration for the Karmada Aggregated API Server
	// +optional
	KarmadaAggregatedAPIServer *KarmadaAggregatedAPIServer `json:"karmadaAggregatedAPIServer,omitempty" yaml:"karmadaAggregatedAPIServer,omitempty"`

	// KubeControllerManager is the configuration for the Kube Controller Manager
	// +optional
	KubeControllerManager *KubeControllerManager `json:"kubeControllerManager,omitempty" yaml:"kubeControllerManager,omitempty"`

	// KarmadaControllerManager is the configuration for the Karmada Controller Manager
	// +optional
	KarmadaControllerManager *KarmadaControllerManager `json:"karmadaControllerManager,omitempty" yaml:"karmadaControllerManager,omitempty"`

	// KarmadaScheduler is the configuration for the Karmada Scheduler
	// +optional
	KarmadaScheduler *KarmadaScheduler `json:"karmadaScheduler,omitempty" yaml:"karmadaScheduler,omitempty"`

	// KarmadaWebhook is the configuration for the Karmada Webhook
	// +optional
	KarmadaWebhook *KarmadaWebhook `json:"karmadaWebhook,omitempty" yaml:"karmadaWebhook,omitempty"`
}

// Networking defines network-related configuration
type Networking struct {
	// Namespace is the Kubernetes namespace where Karmada is deployed
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Port is the port number for the Karmada API Server
	// +optional
	Port int32 `json:"port,omitempty" yaml:"port,omitempty"`
}

// CommonSettings defines common settings for components
type CommonSettings struct {
	// Image specifies the image to use for the component
	Image `json:",inline" yaml:",inline"`

	// Replicas is the number of replicas for the component
	// +optional
	Replicas int32 `json:"replicas,omitempty" yaml:"replicas,omitempty"`

	// Resources defines resource requests and limits for the component
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`

	// NodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`

	// Tolerations define pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`

	// Affinity defines pod affinity rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty" yaml:"affinity,omitempty"`
}

// Image defines image information
type Image struct {
	// Repository is the repository for the image
	// +optional
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`

	// Tag is the tag for the image
	// +optional
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

// KarmadaAPIServer defines the configuration for the Karmada API Server
type KarmadaAPIServer struct {
	CommonSettings `json:",inline" yaml:",inline"`

	// AdvertiseAddress is the address advertised by the API server
	// +optional
	AdvertiseAddress string `json:"advertiseAddress,omitempty" yaml:"advertiseAddress,omitempty"`

	// Networking configures network-related information
	// +optional
	Networking Networking `json:"networking,omitempty" yaml:"networking,omitempty"`

	// ServiceAnnotations are annotations added to the API server service
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty" yaml:"serviceAnnotations,omitempty"`
}

// KarmadaAggregatedAPIServer defines the configuration for the Karmada Aggregated API Server
type KarmadaAggregatedAPIServer struct {
	CommonSettings `json:",inline" yaml:",inline"`
}

// KubeControllerManager defines the configuration for the Kube Controller Manager
type KubeControllerManager struct {
	CommonSettings `json:",inline" yaml:",inline"`
}

// KarmadaControllerManager defines the configuration for the Karmada Controller Manager
type KarmadaControllerManager struct {
	CommonSettings `json:",inline" yaml:",inline"`
}

// KarmadaScheduler defines the configuration for the Karmada Scheduler
type KarmadaScheduler struct {
	CommonSettings `json:",inline" yaml:",inline"`
}

// KarmadaWebhook defines the configuration for the Karmada Webhook
type KarmadaWebhook struct {
	CommonSettings `json:",inline" yaml:",inline"`
}

// LocalSecretReference is a reference to a secret within the same namespace
type LocalSecretReference struct {
	// Name is the name of the referenced secret
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// ImageRegistry represents an image registry
type ImageRegistry struct {
	// Registry is the hostname of the image registry
	// +required
	Registry string `json:"registry" yaml:"registry"`
}

// GetImage generates the full image string in the format "Repository:Tag"
// by combining the image repository and tag fields.
func (i *Image) GetImage() string {
	if i.Tag == "" || i.Repository == "" {
		return ""
	}
	return i.Repository + ":" + i.Tag
}
