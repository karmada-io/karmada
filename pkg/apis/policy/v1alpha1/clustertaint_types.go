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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:path=clustertaintpolicies,scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterTaintPolicy defines how Karmada would taint clusters according
// to the conditions on the target clusters.
type ClusterTaintPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterTaintPolicy.
	// +required
	Spec ClusterTaintPolicySpec `json:"spec"`
}

// ClusterTaintPolicySpec represents the desired behavior of ClusterTaintPolicy.
type ClusterTaintPolicySpec struct {
	// TargetClusters specifies the clusters that ClusterTaintPolicy needs
	// to pay attention to.
	// For clusters that meet the MatchConditions, Taints will be added.
	// If targetClusters is not set, any cluster can be selected.
	// +optional
	TargetClusters *ClusterAffinity `json:"targetClusters,omitempty"`

	// MatchConditions indicates the conditions to match for triggering
	// the controller to add taints on the cluster object.
	// The match conditions are ANDed.
	// When the MatchConditions no longer match, the taints will be removed.
	// It can not be empty.
	// +required
	MatchConditions []MatchCondition `json:"matchConditions"`

	// Taints specifies the taints that need to be applied to the clusters
	// which match with TargetClusters.
	// Distinct ClusterTaintPolicy objects are restricted from operating on
	// the same taint.
	// +required
	Taints []Taint `json:"taints"`
}

// MatchCondition represents the condition match detail of activating the failover
// relevant taints on target clusters.
type MatchCondition struct {
	// ConditionType specifies the ClusterStatus condition type.
	// +required
	ConditionType string `json:"conditionType"`

	// Operator represents a relationship to a set of values.
	// Valid operators are In, NotIn.
	// +required
	Operator MatchConditionOperator `json:"operator"`

	// StatusValues is an array of metav1.ConditionStatus values.
	// The item specifies the ClusterStatus condition status.
	// +required
	StatusValues []metav1.ConditionStatus `json:"statusValues"`
}

// A MatchConditionOperator operator is the set of operators that can be used in the match condition.
type MatchConditionOperator string

const (
	// MatchConditionOpIn represents the operator In.
	MatchConditionOpIn MatchConditionOperator = "In"
	// MatchConditionOpNotIn represents the operator NotIn.
	MatchConditionOpNotIn MatchConditionOperator = "NotIn"
)

// Taint describes the taint that needs to be applied to the cluster.
type Taint struct {
	// Key represents the taint key to be applied to a cluster.
	// +required
	Key string `json:"key"`

	// Effect represents the taint effect to be applied to a cluster.
	// +required
	Effect corev1.TaintEffect `json:"effect"`

	// Value represents the taint value corresponding to the taint key.
	// +optional
	Value string `json:"value,omitempty"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterTaintPolicyList contains a list of ClusterTaintPolicy
type ClusterTaintPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTaintPolicy `json:"items"`
}
