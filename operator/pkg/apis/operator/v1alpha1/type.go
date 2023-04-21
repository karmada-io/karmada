/*
Copyright 2022 The Karmada Authors.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:path=karmadas,scope=Namespaced,categories={karmada-io}
// +kubebuilder:printcolumn:JSONPath=`.status.controlPlaneReady`,name="Status",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// Karmada enables declarative installation of karmada.
type Karmada struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired behavior of the Karmada.
	// +optional
	Spec KarmadaSpec `json:"spec,omitempty"`

	// Most recently observed status of the Karmada.
	// +optional
	Status KarmadaStatus `json:"status,omitempty"`
}

// KarmadaSpec is the specification of the desired behavior of the Karmada.
type KarmadaSpec struct {
	// HostCluster represents the cluster where to install the Karmada control plane.
	// If not set, the control plane will be installed on the cluster where
	// running the operator.
	// +optional
	HostCluster *HostCluster `json:"hostCluster,omitempty"`

	// PrivateRegistry is the global image registry.
	// If set, the operator will pull all required images from it, no matter
	// the image is maintained by Karmada or Kubernetes.
	// It will be useful for offline installation to specify an accessible registry.
	// +optional
	PrivateRegistry *ImageRegistry `json:"privateRegistry,omitempty"`

	// Components define all of karmada components.
	// not all of these components need to be installed.
	// +optional
	Components *KarmadaComponents `json:"components,omitempty"`

	// FeatureGates enabled by the user.
	// - Failover: https://karmada.io/docs/userguide/failover/#failover
	// - GragscefulEviction: https://karmada.io/docs/userguide/failover/#graceful-eviction-feature
	// - PropagateDeps: https://karmada.io/docs/userguide/scheduling/propagate-dependencies
	// - CustomizedClusterResourceModeling: https://karmada.io/docs/userguide/scheduling/cluster-resources#start-to-use-cluster-resource-models
	// More info: https://github.com/karmada-io/karmada/blob/master/pkg/features/features.go
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// ImageRegistry represents an image registry as well as the
// necessary credentials to access with.
// Note: Postpone define the credentials to the next release.
type ImageRegistry struct {
	// Registry is the image registry hostname, like:
	// - docker.io
	// - fictional.registry.example
	// +required
	Registry string `json:"registry"`
}

// KarmadaComponents define all of karmada components.
type KarmadaComponents struct {
	// Etcd holds configuration for etcd.
	// +required
	Etcd *Etcd `json:"etcd,omitempty"`

	// KarmadaAPIServer holds settings to kube-apiserver component. Currently, kube-apiserver
	// is used as the apiserver of karmada. we had the same experience as k8s apiserver.
	// +optional
	KarmadaAPIServer *KarmadaAPIServer `json:"karmadaAPIServer,omitempty"`

	// KarmadaAggregatedAPIServer holds settings to karmada-aggregated-apiserver component of the karmada.
	// +optional
	KarmadaAggregatedAPIServer *KarmadaAggregatedAPIServer `json:"karmadaAggregatedAPIServer,omitempty"`

	// KubeControllerManager holds settings to kube-controller-manager component of the karmada.
	// +optional
	KubeControllerManager *KubeControllerManager `json:"kubeControllerManager,omitempty"`

	// KarmadaControllerManager holds settings to karmada-controller-manager component of the karmada.
	// +optional
	KarmadaControllerManager *KarmadaControllerManager `json:"karmadaControllerManager,omitempty"`

	// KarmadaScheduler holds settings to karmada-scheduler component of the karmada.
	// +optional
	KarmadaScheduler *KarmadaScheduler `json:"karmadaScheduler,omitempty"`

	// KarmadaWebhook holds settings to karmada-webook component of the karmada.
	// +optional
	KarmadaWebhook *KarmadaWebhook `json:"karmadaWebhook,omitempty"`

	// KarmadaDescheduler holds settings to karmada-descheduler component of the karmada.
	// +optional
	KarmadaDescheduler *KarmadaDescheduler `json:"KarmadaDescheduler,omitempty"`

	// KarmadaSearch holds settings to karmada search component of the karmada.
	// +optional
	KarmadaSearch *KarmadaSearch `json:"karmadaSearch,omitempty"`
}

// Networking contains elements describing cluster's networking configuration
type Networking struct {
	// DNSDomain is the dns domain used by k8s services. Defaults to "cluster.local".
	// +optional
	DNSDomain *string `json:"dnsDomain,omitempty"`
}

// Etcd contains elements describing Etcd configuration.
type Etcd struct {
	// Local provides configuration knobs for configuring the built-in etcd instance
	// Local and External are mutually exclusive
	// +optional
	Local *LocalEtcd `json:"local,omitempty"`

	// External describes how to connect to an external etcd cluster
	// Local and External are mutually exclusive
	// +optional
	External *ExternalEtcd `json:"external,omitempty"`
}

// LocalEtcd describes that operator should run an etcd cluster in a host cluster.
type LocalEtcd struct {
	// CommonSettings holds common settings to etcd.
	CommonSettings `json:",inline"`

	// VolumeData describes the settings of etcd data store.
	// We will support 3 modes: emtydir, hostPath, PVC. default by hostPath.
	// +optional
	VolumeData *VolumeData `json:"volumeData,omitempty"`

	// ServerCertSANs sets extra Subject Alternative Names for the etcd server signing cert.
	// +optional
	ServerCertSANs []string `json:"serverCertSANs,omitempty"`

	// PeerCertSANs sets extra Subject Alternative Names for the etcd peer signing cert.
	// +optional
	PeerCertSANs []string `json:"peerCertSANs,omitempty"`
}

// VolumeData describes the settings of etcd data store.
type VolumeData struct {
	// The specification for the PersistentVolumeClaim. The entire content is
	// copied unchanged into the PVC that gets created from this
	// template. The same fields as in a PersistentVolumeClaim
	// are also valid here.
	// +optional
	VolumeClaim *corev1.PersistentVolumeClaimTemplate `json:"volumeClaim,omitempty"`

	// HostPath represents a pre-existing file or directory on the host
	// machine that is directly exposed to the container. This is generally
	// used for system agents or other privileged things that are allowed
	// to see the host machine. Most containers will NOT need this.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	// ---
	// TODO(jonesdl) We need to restrict who can use host directory mounts and who can/can not
	// mount host directories as read/write.
	// +optional
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`

	// EmptyDir represents a temporary directory that shares a pod's lifetime.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
}

// ExternalEtcd describes an external etcd cluster.
// operator has no knowledge of where certificate files live and they must be supplied.
type ExternalEtcd struct {
	// Endpoints of etcd members. Required for ExternalEtcd.
	Endpoints []string `json:"endpoints"`

	// CAData is an SSL Certificate Authority file used to secure etcd communication.
	// Required if using a TLS connection.
	CAData []byte `json:"caData"`

	// CertData is an SSL certification file used to secure etcd communication.
	// Required if using a TLS connection.
	CertData []byte `json:"certData"`

	// KeyData is an SSL key file used to secure etcd communication.
	// Required if using a TLS connection.
	KeyData []byte `json:"keyData"`
}

// KarmadaAPIServer holds settings to kube-apiserver component of the kubernetes.
// Karmada uses it as it's own apiserver in order to provide Kubernetes-native APIs.
type KarmadaAPIServer struct {
	// CommonSettings holds common settings to kubernetes api server.
	CommonSettings `json:",inline"`

	// ServiceSubnet is the subnet used by k8s services. Defaults to "10.96.0.0/12".
	// +optional
	ServiceSubnet *string `json:"serviceSubnet,omitempty"`

	// ServiceType represents the service type of karmada apiserver.
	// it is Nodeport by default.
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// ExtraArgs is an extra set of flags to pass to the kube-apiserver component or
	// override. A key in this map is the flag name as it appears on the command line except
	// without leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the
	// kube-apiserver component. In the future, we will provide a more structured way
	// to configure the component. Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// CertSANs sets extra Subject Alternative Names for the API Server signing cert.
	// +optional
	CertSANs []string `json:"certSANs,omitempty"`

	// FeatureGates enabled by the user.
	// More info: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KarmadaAggregatedAPIServer holds settings to karmada-aggregated-apiserver component of the karmada.
type KarmadaAggregatedAPIServer struct {
	// CommonSettings holds common settings to karmada apiServer.
	CommonSettings `json:",inline"`

	// ExtraArgs is an extra set of flags to pass to the karmada-aggregated-apiserver component or
	// override. A key in this map is the flag name as it appears on the command line except
	// without leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the
	// karmada-aggregated-apiserver component. In the future, we will provide a more structured way
	// to configure the component. Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://github.com/karmada-io/karmada/blob/master/cmd/aggregated-apiserver/app/options/options.go
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// CertSANs sets extra Subject Alternative Names for the API Server signing cert.
	// +optional
	CertSANs []string `json:"certSANs,omitempty"`

	// FeatureGates enabled by the user.
	// - CustomizedClusterResourceModeling: https://karmada.io/docs/userguide/scheduling/cluster-resources#start-to-use-cluster-resource-models
	// More info: https://github.com/karmada-io/karmada/blob/master/pkg/features/features.go
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KubeControllerManager holds settings to kube-controller-manager component of the kubernetes.
// Karmada uses it to manage the lifecycle of the federated resources. An especial case is the garbage
// collection of the orphan resources in your karmada.
type KubeControllerManager struct {
	// CommonSettings holds common settings to kubernetes controller manager.
	CommonSettings `json:",inline"`

	// A list of controllers to enable. '*' enables all on-by-default controllers,
	// 'foo' enables the controller named 'foo', '-foo' disables the controller named
	// 'foo'.
	//
	// All controllers: attachdetach, bootstrapsigner, cloud-node-lifecycle,
	// clusterrole-aggregation, cronjob, csrapproving, csrcleaner, csrsigning,
	// daemonset, deployment, disruption, endpoint, endpointslice,
	// endpointslicemirroring, ephemeral-volume, garbagecollector,
	// horizontalpodautoscaling, job, namespace, nodeipam, nodelifecycle,
	// persistentvolume-binder, persistentvolume-expander, podgc, pv-protection,
	// pvc-protection, replicaset, replicationcontroller, resourcequota,
	// root-ca-cert-publisher, route, service, serviceaccount, serviceaccount-token,
	// statefulset, tokencleaner, ttl, ttl-after-finished
	// Disabled-by-default controllers: bootstrapsigner, tokencleaner (default [*])
	// Actual Supported controllers depend on the version of Kubernetes. See
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/
	// for details.
	//
	// However, Karmada uses Kubernetes Native API definitions for federated resource template,
	// so it doesn't need enable some resource related controllers like daemonset, deployment etc.
	// On the other hand, Karmada leverages the capabilities of the Kubernetes controller to
	// manage the lifecycle of the federated resource, so it needs to enable some controllers.
	// For example, the `namespace` controller is used to manage the lifecycle of the namespace
	// and the `garbagecollector` controller handles automatic clean-up of redundant items in
	// your karmada.
	//
	// According to the user feedback and karmada requirements, the following controllers are
	// enabled by default: namespace, garbagecollector, serviceaccount-token, ttl-after-finished,
	// bootstrapsigner,csrapproving,csrcleaner,csrsigning. See
	// https://karmada.io/docs/administrator/configuration/configure-controllers#kubernetes-controllers
	//
	// Others are disabled by default. If you want to enable or disable other controllers, you
	// have to explicitly specify all the controllers that kube-controller-manager shoud enable
	// at startup phase.
	// +optional
	Controllers []string `json:"controllers,omitempty"`

	// ExtraArgs is an extra set of flags to pass to the kube-controller-manager component or
	// override. A key in this map is the flag name as it appears on the command line except
	// without leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the
	// kube-controller-manager component. In the future, we will provide a more structured way
	// to configure the component. Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// FeatureGates enabled by the user.
	// More info: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KarmadaControllerManager holds settings to the karmada-controller-manager component of the karmada.
type KarmadaControllerManager struct {
	// CommonSettings holds common settings to karmada controller manager.
	CommonSettings `json:",inline"`

	// A list of controllers to enable. '*' enables all on-by-default controllers,
	// 'foo' enables the controller named 'foo', '-foo' disables the controller named
	// 'foo'.
	//
	// All controllers: binding, cluster, clusterStatus, endpointSlice, execution,
	// federatedResourceQuotaStatus, federatedResourceQuotaSync, hpa, namespace,
	// serviceExport, serviceImport, unifiedAuth, workStatus.
	// Disabled-by-default controllers: hpa (default [*])
	// Actual Supported controllers depend on the version of Karmada. See
	// https://karmada.io/docs/administrator/configuration/configure-controllers#configure-karmada-controllers
	// for details.
	//
	// +optional
	Controllers []string `json:"controllers,omitempty"`

	// ExtraArgs is an extra set of flags to pass to the karmada-controller-manager component or
	// override. A key in this map is the flag name as it appears on the command line except
	// without leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the
	// karmada-controller-manager component. In the future, we will provide a more structured way
	// to configure the component. Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://github.com/karmada-io/karmada/blob/master/cmd/controller-manager/app/options/options.go
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// FeatureGates enabled by the user.
	// - Failover: https://karmada.io/docs/userguide/failover/#failover
	// - GracefulEviction: https://karmada.io/docs/userguide/failover/#graceful-eviction-feature
	// - PropagateDeps: https://karmada.io/docs/userguide/scheduling/propagate-dependencies
	// - CustomizedClusterResourceModeling: https://karmada.io/docs/userguide/scheduling/cluster-resources#start-to-use-cluster-resource-models
	// More info: https://github.com/karmada-io/karmada/blob/master/pkg/features/features.go
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KarmadaScheduler holds settings to karmada-scheduler component of the karmada.
type KarmadaScheduler struct {
	// CommonSettings holds common settings to karmada scheduler.
	CommonSettings `json:",inline"`

	// ExtraArgs is an extra set of flags to pass to the karmada-scheduler component or override.
	// A key in this map is the flag name as it appears on the command line except without
	// leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the karmada-scheduler
	// component. In the future, we will provide a more structured way to configure the component.
	// Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://github.com/karmada-io/karmada/blob/master/cmd/scheduler/app/options/options.go
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// FeatureGates enabled by the user.
	// - CustomizedClusterResourceModeling: https://karmada.io/docs/userguide/scheduling/cluster-resources#start-to-use-cluster-resource-models
	// More info: https://github.com/karmada-io/karmada/blob/master/pkg/features/features.go
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KarmadaDescheduler holds settings to karmada-descheduler component of the karmada.
type KarmadaDescheduler struct {
	// CommonSettings holds common settings to karmada descheduler.
	CommonSettings `json:",inline"`

	// ExtraArgs is an extra set of flags to pass to the karmada-descheduler component or override.
	// A key in this map is the flag name as it appears on the command line except without
	// leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the karmada-descheduler
	// component. In the future, we will provide a more structured way to configure the component.
	// Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://github.com/karmada-io/karmada/blob/master/cmd/descheduler/app/options/options.go
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// KarmadaSearch holds settings to karmada-search component of the karmada.
type KarmadaSearch struct {
	// CommonSettings holds common settings to karmada search.
	CommonSettings `json:",inline"`

	// ExtraArgs is an extra set of flags to pass to the karmada-descheduler component or override.
	// A key in this map is the flag name as it appears on the command line except without
	// leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the karmada-descheduler
	// component. In the future, we will provide a more structured way to configure the component.
	// Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://github.com/karmada-io/karmada/blob/master/cmd/descheduler/app/options/options.go
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// KarmadaWebhook holds settings to karmada-webhook component of the karmada.
type KarmadaWebhook struct {
	// CommonSettings holds common settings to karmada webhook.
	CommonSettings `json:",inline"`

	// ExtraArgs is an extra set of flags to pass to the karmada-webhook component or
	// override. A key in this map is the flag name as it appears on the command line except
	// without leading dash(es).
	//
	// Note: This is a temporary solution to allow for the configuration of the
	// karmada-webhook component. In the future, we will provide a more structured way
	// to configure the component. Once that is done, this field will be discouraged to be used.
	// Incorrect settings on this feild maybe lead to the corresponding component in an unhealthy
	// state. Before you do it, please confirm that you understand the risks of this configuration.
	//
	// For supported flags, please see
	// https://github.com/karmada-io/karmada/blob/master/cmd/webhook/app/options/options.go
	// for details.
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// CommonSettings describes the common settings of all karmada Components.
type CommonSettings struct {
	// Image allows to customize the image used for the component.
	Image `json:",inline"`

	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Compute Resources required by this component.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// Image allows to customize the image used for components.
type Image struct {
	// ImageRepository sets the container registry to pull images from.
	// if not set, the ImageRepository defined in KarmadaSpec will be used instead.
	// +optional
	ImageRepository string `json:"imageRepository,omitempty"`

	// ImageTag allows to specify a tag for the image.
	// In case this value is set, operator does not change automatically the version
	// of the above components during upgrades.
	// +optional
	ImageTag string `json:"imageTag,omitempty"`
}

// HostCluster represents the cluster where to install the Karmada control plane.
type HostCluster struct {
	// APIEndpoint is the API endpoint of the cluster where deploy Karmada
	// control plane on.
	// This can be a hostname, hostname:port, IP or IP:port.
	// +optional
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// SecretRef represents the secret contains mandatory credentials to
	// access the cluster.
	// The secret should hold credentials as follows:
	// - secret.data.token
	// - secret.data.caBundle
	// +optional
	SecretRef *LocalSecretReference `json:"secretRef,omitempty"`

	// Networking holds configuration for the networking topology of the cluster.
	// +optional
	Networking *Networking `json:"networking,omitempty"`
}

// ConditionType declarative karmada condition type of karmada installtion.
type ConditionType string

const (
	// Unknown represent a condition type the karmada not be reconciled by operator
	// or unpredictable condition.
	Unknown ConditionType = "Unknown"

	// Ready represent a condition type the all installtion process to karmada have compaleted.
	Ready ConditionType = "Ready"
)

// KarmadaStatus difine the most recently observed status of the Karmada.
type KarmadaStatus struct {
	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// after the karmada installed, restore the kubeconfig to secret.
	// +optional
	SecretRef *LocalSecretReference `json:"secretRef,omitempty"`

	// KarmadaVersion represente the karmada version.
	// +optional
	KarmadaVersion string `json:"karmadaVersion,omitempty"`

	// KubernetesVersion represente the karmada-apiserver version.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// Conditions represents the latest available observations of a karmada's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// LocalSecretReference is a reference to a secret within the enclosing
// namespace.
type LocalSecretReference struct {
	// Namespace is the namespace for the resource being referenced.
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of resource being referenced.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KarmadaList is a list of Karmadas.
type KarmadaList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Karmada `json:"items"`
}
