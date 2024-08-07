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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// InitConfiguration holds the configuration for initializing a Kubernetes cluster.
type InitConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// GeneralConfig contains the common configuration at initialization.
	GeneralConfig GeneralConfig `yaml:"generalConfig"`

	// CertificateConfig contains configurations related to certificates.
	CertificateConfig CertificateConfig `yaml:"certificateConfig"`

	// EtcdConfig contains configuration for etcd.
	EtcdConfig EtcdConfig `yaml:"etcdConfig"`

	// KarmadaControlPlaneConfig contains control plane configurations.
	KarmadaControlPlaneConfig KarmadaControlPlaneConfig `yaml:"karmadaControlPlaneConfig"`

	// ImageConfig contains image-related configurations.
	ImageConfig ImageConfig `yaml:"imageConfig"`
}

// GeneralConfig contains general configuration parameters.
type GeneralConfig struct {
	Namespace                 string `yaml:"namespace"`
	KubeConfigPath            string `yaml:"kubeConfigPath"`
	KubeImageTag              string `yaml:"kubeImageTag"`
	Context                   string `yaml:"context"`
	PrivateImageRegistry      string `yaml:"privateImageRegistry"`
	WaitComponentReadyTimeout int    `yaml:"waitComponentReadyTimeout"`
	Port                      int    `yaml:"port"`
	StorageClassesName        string `yaml:"storageClassesName"`
}

// CertificateConfig contains certificate-related configuration.
type CertificateConfig struct {
	ExternalDNS    []string `yaml:"externalDNS"`
	ExternalIP     []string `yaml:"externalIP"`
	ValidityPeriod string   `yaml:"validityPeriod"`
	CaCertFile     string   `yaml:"caCertFile"`
	CaCertKeyFile  string   `yaml:"caCertKeyFile"`
	ExtraArgs      []Arg    `yaml:"extraArgs"`
}

// EtcdConfig contains etcd configuration parameters.
type EtcdConfig struct {
	Local    *LocalEtcd    `json:"local,omitempty"`
	External *ExternalEtcd `json:"external,omitempty"`
}

// LocalEtcd contains configuration for a local etcd instance.
type LocalEtcd struct {
	Image              string            `yaml:"image"`
	InitImage          string            `yaml:"initImage"`
	DataDir            string            `yaml:"dataDir"`
	PVCSize            string            `yaml:"pvcSize"`
	NodeSelectorLabels map[string]string `yaml:"nodeSelectorLabels"`
	StorageMode        string            `yaml:"storageMode"`
	Replicas           int32             `yaml:"replicas"`
	ExtraArgs          []Arg             `yaml:"extraArgs"`
}

// ExternalEtcd contains configuration for connecting to an external etcd cluster.
type ExternalEtcd struct {
	ExternalCAPath   string `yaml:"externalCAPath"`
	ExternalCertPath string `yaml:"externalCertPath"`
	ExternalKeyPath  string `yaml:"externalKeyPath"`
	ExternalServers  string `yaml:"externalServers"`
	ExternalPrefix   string `yaml:"externalPrefix"`
	ExtraArgs        []Arg  `yaml:"extraArgs"`
}

// KarmadaControlPlaneConfig contains configuration for the control plane components.
type KarmadaControlPlaneConfig struct {
	APIServer                   APIServerConfig             `yaml:"apiServer"`
	ControllerManager           ControllerManagerConfig     `yaml:"controllerManager"`
	Scheduler                   SchedulerConfig             `yaml:"scheduler"`
	Webhook                     WebhookConfig               `yaml:"webhook"`
	AggregatedAPIServerConfig   AggregatedAPIServerConfig   `yaml:"aggregatedAPIServerConfig"`
	KubeControllerManagerConfig KubeControllerManagerConfig `yaml:"kubeControllerManagerConfig"`
	DataPath                    string                      `yaml:"dataPath"`
	PkiPath                     string                      `yaml:"pkiPath"`
	CRDs                        string                      `yaml:"crds"`
	HostClusterDomain           string                      `yaml:"hostClusterDomain"`
}

// APIServerConfig contains configuration for the API server.
type APIServerConfig struct {
	Image            string `yaml:"image"`
	AdvertiseAddress string `yaml:"advertiseAddress"`
	Replicas         int32  `yaml:"replicas"`
	NodePort         int32  `yaml:"nodePort"`
	ExtraArgs        []Arg  `yaml:"extraArgs"`
}

// ControllerManagerConfig contains configuration for the controller manager.
type ControllerManagerConfig struct {
	Image     string `yaml:"image"`
	Replicas  int32  `yaml:"replicas"`
	ExtraArgs []Arg  `yaml:"extraArgs"`
}

// SchedulerConfig contains configuration for the scheduler.
type SchedulerConfig struct {
	Image     string `yaml:"image"`
	Replicas  int32  `yaml:"replicas"`
	ExtraArgs []Arg  `yaml:"extraArgs"`
}

// WebhookConfig contains configuration for the webhook.
type WebhookConfig struct {
	Image     string `yaml:"image"`
	Replicas  int32  `yaml:"replicas"`
	ExtraArgs []Arg  `yaml:"extraArgs"`
}

// AggregatedAPIServerConfig contains configuration for the aggregated API server.
type AggregatedAPIServerConfig struct {
	Image     string `yaml:"image"`
	Replicas  int32  `yaml:"replicas"`
	ExtraArgs []Arg  `yaml:"extraArgs"`
}

// KubeControllerManagerConfig contains configuration for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	Image     string `yaml:"image"`
	Replicas  int32  `yaml:"replicas"`
	ExtraArgs []Arg  `yaml:"extraArgs"`
}

// ImageConfig contains configuration for images used in the cluster.
type ImageConfig struct {
	KubeImageTag           string   `yaml:"kubeImageTag"`
	KubeImageRegistry      string   `yaml:"kubeImageRegistry"`
	KubeImageMirrorCountry string   `yaml:"kubeImageMirrorCountry"`
	ImagePullPolicy        string   `yaml:"imagePullPolicy"`
	ImagePullSecrets       []string `yaml:"imagePullSecrets"`
}

// Arg represents a name-value pair argument.
type Arg struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
