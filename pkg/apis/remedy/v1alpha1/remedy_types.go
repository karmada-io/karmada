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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=remedies,scope="Cluster",categories={karmada-io}

// Remedy represents the cluster-level management strategies based on cluster conditions.
type Remedy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of Remedy.
	// +required
	Spec RemedySpec `json:"spec"`
}

// RemedySpec represents the desired behavior of Remedy.
type RemedySpec struct {
	// ClusterAffinity specifies the clusters that Remedy needs to pay attention to.
	// For clusters that meet the DecisionConditions, Actions will be preformed.
	// If empty, all clusters will be selected.
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

	// DecisionMatches indicates the decision matches of triggering the remedy
	// system to perform the actions. As long as any one DecisionMatch matches,
	// the Actions will be preformed.
	// If empty, the Actions will be performed immediately.
	// +optional
	DecisionMatches []DecisionMatch `json:"decisionMatches,omitempty"`

	// Actions specifies the actions that remedy system needs to perform.
	// If empty, no action will be performed.
	// +optional
	Actions []RemedyAction `json:"actions,omitempty"`
}

// DecisionMatch represents the decision match detail of activating the remedy system.
type DecisionMatch struct {
	// ClusterConditionMatch describes the cluster condition requirement.
	// +optional
	ClusterConditionMatch *ClusterConditionRequirement `json:"clusterConditionMatch,omitempty"`
}

// ClusterConditionRequirement describes the Cluster condition requirement details.
type ClusterConditionRequirement struct {
	// ConditionType specifies the ClusterStatus condition type.
	// +required
	ConditionType ConditionType `json:"conditionType"`
	// Operator represents a conditionType's relationship to a conditionStatus.
	// Valid operators are Equal, NotEqual.
	//
	// +kubebuilder:validation:Enum=Equal;NotEqual
	// +required
	Operator ClusterConditionOperator `json:"operator"`
	// ConditionStatus specifies the ClusterStatue condition status.
	// +required
	ConditionStatus string `json:"conditionStatus"`
}

// ConditionType represents the detection ClusterStatus condition type.
type ConditionType string

const (
	// ServiceDomainNameResolutionReady expresses the detection of the domain name resolution
	// function of Service in the Kubernetes cluster.
	ServiceDomainNameResolutionReady ConditionType = "ServiceDomainNameResolutionReady"
)

// ClusterConditionOperator is the set of operators that can be used in the cluster condition requirement.
type ClusterConditionOperator string

const (
	// ClusterConditionEqual means equal match.
	ClusterConditionEqual ClusterConditionOperator = "Equal"
	// ClusterConditionNotEqual means not equal match.
	ClusterConditionNotEqual ClusterConditionOperator = "NotEqual"
)

// ClusterAffinity represents the filter to select clusters.
type ClusterAffinity struct {
	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// RemedyAction represents the action type the remedy system needs to preform.
type RemedyAction string

const (
	// TrafficControl indicates that the cluster requires traffic control.
	TrafficControl RemedyAction = "TrafficControl"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemedyList contains a list of Remedy.
type RemedyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Remedy `json:"items"`
}
