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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=federatedhpas,scope=Namespaced,shortName=fhpa,categories={karmada-io}
// +kubebuilder:printcolumn:JSONPath=`.spec.scaleTargetRef.kind`,name=`REFERENCE-KIND`,type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.scaleTargetRef.name`,name=`REFERENCE-NAME`,type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.minReplicas`,name=`MINPODS`,type=integer
// +kubebuilder:printcolumn:JSONPath=`.spec.maxReplicas`,name=`MAXPODS`,type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.currentReplicas`,name=`REPLICAS`,type=integer
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name=`AGE`,type=date

// FederatedHPA is centralized HPA that can aggregate the metrics in multiple clusters.
// When the system load increases, it will query the metrics from multiple clusters and scales up the replicas.
// When the system load decreases, it will query the metrics from multiple clusters and scales down the replicas.
// After the replicas are scaled up/down, karmada-scheduler will schedule the replicas based on the policy.
type FederatedHPA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the FederatedHPA.
	// +required
	Spec FederatedHPASpec `json:"spec"`

	// Status is the current status of the FederatedHPA.
	// +optional
	Status autoscalingv2.HorizontalPodAutoscalerStatus `json:"status"`
}

// FederatedHPASpec describes the desired functionality of the FederatedHPA.
type FederatedHPASpec struct {
	// ScaleTargetRef points to the target resource to scale, and is used to
	// the pods for which metrics should be collected, as well as to actually
	// change the replica count.
	// +required
	ScaleTargetRef autoscalingv2.CrossVersionObjectReference `json:"scaleTargetRef"`

	// MinReplicas is the lower limit for the number of replicas to which the
	// autoscaler can scale down.
	// It defaults to 1 pod.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the upper limit for the number of replicas to which the
	// autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas"`

	// Metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used). The desired replica count is calculated multiplying the
	// ratio between the target value and the current value by the current
	// number of pods. Ergo, metrics used must decrease as the pod count is
	// increased, and vice-versa. See the individual metric source types for
	// more information about how each type of metric must respond.
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +optional
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`

	// Behavior configures the scaling behavior of the target
	// in both Up and Down directions (scaleUp and scaleDown fields respectively).
	// If not set, the default HPAScalingRules for scale up and scale down are used.
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FederatedHPAList contains a list of FederatedHPA.
type FederatedHPAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederatedHPA `json:"items"`
}
