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

const (
	// ResourceKindFederatedResourceQuota is kind name of FederatedResourceQuota.
	ResourceKindFederatedResourceQuota = "FederatedResourceQuota"
	// ResourceSingularFederatedResourceQuota is singular name of FederatedResourceQuota.
	ResourceSingularFederatedResourceQuota = "federatedresourcequota"
	// ResourcePluralFederatedResourceQuota is plural name of FederatedResourceQuota.
	ResourcePluralFederatedResourceQuota = "federatedresourcequotas"
	// ResourceNamespaceScopedFederatedResourceQuota indicates if FederatedResourceQuota is NamespaceScoped.
	ResourceNamespaceScopedFederatedResourceQuota = true
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=federatedresourcequotas,scope=Namespaced,categories={karmada-io}
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:JSONPath=`.status.overall`,name=`OVERALL`,type=string
// +kubebuilder:printcolumn:JSONPath=`.status.overallUsed`,name=`OVERALL_USED`,type=string

// FederatedResourceQuota sets aggregate quota restrictions enforced per namespace across all clusters.
type FederatedResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired quota.
	// +required
	Spec FederatedResourceQuotaSpec `json:"spec"`

	// Status defines the actual enforced quota and its current usage.
	// +optional
	Status FederatedResourceQuotaStatus `json:"status,omitempty"`
}

// FederatedResourceQuotaSpec defines the desired hard limits to enforce for Quota.
type FederatedResourceQuotaSpec struct {
	// Overall is the set of desired hard limits for each named resource.
	// +required
	Overall corev1.ResourceList `json:"overall"`

	// StaticAssignments specifies ResourceQuota settings for specific clusters.
	// If non-empty, Karmada will create ResourceQuotas in the corresponding clusters.
	// Clusters not listed here or when StaticAssignments is empty will have no ResourceQuotas created.
	//
	// This field addresses multi-cluster configuration management challenges by allowing centralized
	// control over ResourceQuotas across clusters.
	//
	// Note: The Karmada scheduler currently does NOT use this configuration for scheduling decisions.
	// Future updates may integrate it into the scheduling logic.
	//
	// +optional
	StaticAssignments []StaticClusterAssignment `json:"staticAssignments,omitempty"`

	// DynamicAssignments represents the rule about how to assign and ensure the quota for each cluster.
	// E.g.
	//   - The rule about how to assign the total amount of quotas(specified by Overall) to Clusters.
	//   - The rule about how to balance the quota between clusters in a dynamic way.
	//   - The rule about how to prevent workload from scheduling to cluster without quota.
	//   - The rule about how to prevent workload from creating to Karmada control plane.
	//   - The rule about how to cooperate with FederatedHPA to scale workloads.
	//
	// Note: We are in the progress of gathering user cases, if you are interested in
	//  this feature/capacity, please feel free to let us know.
	//
	// +optional
	// DynamicAssignments []DynamicClusterAssignment `json:"dynamicAssignments,omitempty"`
}

// StaticClusterAssignment represents the set of desired hard limits for a specific cluster.
type StaticClusterAssignment struct {
	// ClusterName is the name of the cluster the limits enforce to.
	// +required
	ClusterName string `json:"clusterName"`

	// Hard is the set of desired hard limits for each named resource.
	// +required
	Hard corev1.ResourceList `json:"hard"`
}

// FederatedResourceQuotaStatus defines the enforced hard limits and observed use.
type FederatedResourceQuotaStatus struct {
	// Overall is the set of enforced hard limits for each named resource.
	// +optional
	Overall corev1.ResourceList `json:"overall,omitempty"`

	// OverallUsed is the current observed total usage of the resource in the namespace.
	// +optional
	OverallUsed corev1.ResourceList `json:"overallUsed,omitempty"`

	// AggregatedStatus is the observed quota usage of each cluster.
	// +optional
	AggregatedStatus []ClusterQuotaStatus `json:"aggregatedStatus,omitempty"`
}

// ClusterQuotaStatus represents the set of desired limits and observed usage for a specific cluster.
type ClusterQuotaStatus struct {
	// ClusterName is the name of the cluster the limits enforce to.
	// +required
	ClusterName string `json:"clusterName"`

	corev1.ResourceQuotaStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FederatedResourceQuotaList contains a list of FederatedResourceQuota.
type FederatedResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederatedResourceQuota `json:"items"`
}
