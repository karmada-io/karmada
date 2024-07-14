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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindMultiClusterIngress is kind name of MultiClusterIngress.
	ResourceKindMultiClusterIngress = "MultiClusterIngress"
	// ResourceSingularMultiClusterIngress is singular name of MultiClusterIngress.
	ResourceSingularMultiClusterIngress = "multiclusteringress"
	// ResourcePluralMultiClusterIngress is plural name of MultiClusterIngress.
	ResourcePluralMultiClusterIngress = "multiclusteringresses"
	// ResourceNamespaceScopedMultiClusterIngress indicates if MultiClusterIngress is NamespaceScoped.
	ResourceNamespaceScopedMultiClusterIngress = true
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=multiclusteringresses,scope=Namespaced,shortName=mci,categories={karmada-io}

// MultiClusterIngress is a collection of rules that allow inbound connections to reach the
// endpoints defined by a backend. The structure of MultiClusterIngress is same as Ingress,
// indicates the Ingress in multi-clusters.
type MultiClusterIngress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the MultiClusterIngress.
	// +optional
	Spec networkingv1.IngressSpec `json:"spec,omitempty"`

	// Status is the current state of the MultiClusterIngress.
	// +optional
	Status MultiClusterIngressStatus `json:"status,omitempty"`
}

// MultiClusterIngressStatus is the current state of the MultiClusterIngress.
type MultiClusterIngressStatus struct {
	networkingv1.IngressStatus `json:",inline"`

	// TrafficBlockClusters records the cluster name list that needs to perform traffic block.
	// When the cloud provider implements its multicluster-cloud-provider and refreshes
	// the service backend address to the LoadBalancer Service, it needs to filter out
	// the backend addresses in these clusters.
	// +optional
	TrafficBlockClusters []string `json:"trafficBlockClusters,omitempty"`

	// ServiceLocations records the locations of MulticlusterIngress's backend
	// Service resources. It will be set by the system controller.
	// +optional
	ServiceLocations []ServiceLocation `json:"serviceLocations,omitempty"`
}

// ServiceLocation records the locations of MulticlusterIngress's backend Service resources.
type ServiceLocation struct {
	// name is the referenced service. The service must exist in
	// the same namespace as the MultiClusterService object.
	Name string `json:"name"`

	// Clusters records the cluster list where the Service is located.
	// +optional
	Clusters []string `json:"clusters,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterIngressList is a collection of MultiClusterIngress.
type MultiClusterIngressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of MultiClusterIngress.
	Items []MultiClusterIngress `json:"items"`
}
