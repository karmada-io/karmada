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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindMultiClusterService is kind name of MultiClusterService.
	ResourceKindMultiClusterService = "MultiClusterService"
	// ResourceSingularMultiClusterService is singular name of MultiClusterService.
	ResourceSingularMultiClusterService = "multiclusterservice"
	// ResourcePluralMultiClusterService is plural name of MultiClusterService.
	ResourcePluralMultiClusterService = "multiclusterservices"
	// ResourceNamespaceScopedMultiClusterService indicates if MultiClusterService is NamespaceScoped.
	ResourceNamespaceScopedMultiClusterService = true
	// MCSServiceAppliedConditionType is indicates the condition type of mcs service applied.
	MCSServiceAppliedConditionType = "ServiceApplied"
	// EndpointSliceDispatched indicates whether the EndpointSlice is dispatched to consumption clusters
	EndpointSliceDispatched string = "EndpointSliceDispatched"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=multiclusterservices,scope=Namespaced,shortName=mcs,categories={karmada-io}

// MultiClusterService is a named abstraction of multi-cluster software service.
// The name field of MultiClusterService is the same as that of Service name.
// Services with the same name in different clusters are regarded as the same
// service and are associated with the same MultiClusterService.
// MultiClusterService can control the exposure of services to outside multiple
// clusters, and also enable service discovery between clusters.
type MultiClusterService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the MultiClusterService.
	Spec MultiClusterServiceSpec `json:"spec"`

	// Status is the current state of the MultiClusterService.
	// +optional
	Status corev1.ServiceStatus `json:"status,omitempty"`
}

// MultiClusterServiceSpec is the desired state of the MultiClusterService.
type MultiClusterServiceSpec struct {
	// Types specifies how to expose the service referencing by this
	// MultiClusterService.
	// +required
	Types []ExposureType `json:"types"`

	// Ports is the list of ports that are exposed by this MultiClusterService.
	// No specified port will be filtered out during the service
	// exposure and discovery process.
	// All ports in the referencing service will be exposed by default.
	// +optional
	Ports []ExposurePort `json:"ports,omitempty"`

	// Range specifies the ranges where the referencing service should
	// be exposed.
	// Only valid and optional in case of Types contains CrossCluster.
	// If not set and Types contains CrossCluster, all clusters will
	// be selected, that means the referencing service will be exposed
	// across all registered clusters.
	// Deprecated: in favor of ProviderClusters/ConsumerClusters.
	// +optional
	Range ExposureRange `json:"range,omitempty"`

	// ServiceProvisionClusters specifies the clusters which will provision the service backend.
	// If leave it empty, we will collect the backend endpoints from all clusters and sync
	// them to the ServiceConsumptionClusters.
	// Deprecated: in favor of ProviderClusters/ConsumerClusters.
	// +optional
	ServiceProvisionClusters []string `json:"serviceProvisionClusters,omitempty"`

	// ServiceConsumptionClusters specifies the clusters where the service will be exposed, for clients.
	// If leave it empty, the service will be exposed to all clusters.
	// Deprecated: in favor of ProviderClusters/ConsumerClusters.
	// +optional
	ServiceConsumptionClusters []string `json:"serviceConsumptionClusters,omitempty"`

	// ProviderClusters specifies the clusters which will provide the service backend.
	// If leave it empty, we will collect the backend endpoints from all clusters and sync
	// them to the ConsumerClusters.
	// +optional
	ProviderClusters []ClusterSelector `json:"providerClusters,omitempty"`

	// ConsumerClusters specifies the clusters where the service will be exposed, for clients.
	// If leave it empty, the service will be exposed to all clusters.
	// +optional
	ConsumerClusters []ClusterSelector `json:"consumerClusters,omitempty"`
}

// ClusterSelector specifies the cluster to be selected.
type ClusterSelector struct {
	// Name is the name of the cluster to be selected.
	// +required
	Name string `json:"name,omitempty"`
}

// ExposureType describes how to expose the service.
type ExposureType string

const (
	// ExposureTypeCrossCluster means a service will be accessible across clusters.
	ExposureTypeCrossCluster ExposureType = "CrossCluster"

	// ExposureTypeLoadBalancer means a service will be exposed via an external
	// load balancer.
	ExposureTypeLoadBalancer ExposureType = "LoadBalancer"
)

// ExposurePort describes which port will be exposed.
type ExposurePort struct {
	// Name is the name of the port that needs to be exposed within the service.
	// The port name must be the same as that defined in the service.
	// +optional
	Name string `json:"name,omitempty"`

	// Port specifies the exposed service port.
	// +required
	Port int32 `json:"port"`
}

// ExposureRange describes a list of clusters where the service is exposed.
// Now supports selecting cluster by name, leave the room for extend more methods
// such as using label selector.
type ExposureRange struct {
	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterServiceList is a collection of MultiClusterService.
type MultiClusterServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of MultiClusterService.
	Items []MultiClusterService `json:"items"`
}
