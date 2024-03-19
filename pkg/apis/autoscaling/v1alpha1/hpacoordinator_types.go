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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// HPACoordinator is a cross-cluster HPA coordinator that can manages the replica
// number of a specific application based on corresponding metrics.
// When the system load increases, it relies on the HPA controller running in the
// cluster to scale up first, when resource in that cluster are limited, it will
// attempt to scale up in other available clusters. When system pressure decreases,
// it will also reclaim replicas from the cluster to reduce resource costs.
type HPACoordinator struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec defines the desired behavior of HPACoordinator.
	// +required
	Spec HPACoordinatorSpec `json:"spec"`

	// Status represents the most recently observed status.
	// +optional
	Status HPACoordinatorStatus `json:"status,omitempty"`
}

// HPACoordinatorSpec describes the desired functionality of the HPACoordinator.
type HPACoordinatorSpec struct {
	// ScaleTargetRef points to the target resource to scale, and is used to
	// the pods for which metrics should be collected, as well as to actually
	// change the replica count.
	// +required
	ScaleTargetRef CrossVersionObjectReference `json:"scaleTargetRef"`

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

	// ClusterScalePreferences configures the preferences for collaborative
	// clusters as well as customized behaviors.
	//
	// The preferences are described in the order they appear in the spec.
	// When the system load increase, the new replicas will be scaled up on these
	// clusters according to this order. When the system load decreases, the
	// replicas will be scaled down from these clusters in reverse order.
	//
	// The candidate clusters will be preliminarily selected by the PropagationPolicy
	// (or ClusterPropagationPolicy) and the scheduler's strategy, while ClusterScalePreferences
	// further specifies the available cluster. If a cluster in the ClusterScalePreferences
	// list but is excluded by the scheduler, it will be ignored.
	// +required
	ClusterScalePreferences []ClusterScalePreference `json:"clusterScalePreference"`
}

// CrossVersionObjectReference contains enough information to let you identify
// the referred resource.
type CrossVersionObjectReference struct {
	// APIVersion represents the API version of the referent.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the referent.
	// +required
	Kind string `json:"kind"`

	// Name represents the name of the referent.
	// +required
	Name string `json:"name"`
}

// ClusterScalePreference describes customized behavior for specific cluster.
type ClusterScalePreference struct {
	// ClusterName is the name of the member cluster.
	// +required
	ClusterName string `json:"clusterName"`

	// MinReplicas is the minimum number of replicas on the current cluster,
	// Karmada will ensure that the number of replicas running on this cluster
	// is not less than this value.
	//
	// Note that this only valid when at least one replica is running in the
	// current cluster. See InitialReplicas for more details.
	//
	// It defaults to 1 pod.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas allowed on the current
	// cluster, Karmada will ensure that the number of running replicas does not
	// exceed this value during coordination.
	//
	// Karmada suggests setting this value to ensure that replicas do not scale
	// indefinitely, thus affecting other applications in the system.
	//
	// It cannot be less than MinReplicas.
	// +required
	MaxReplicas int32 `json:"maxReplicas"`

	// InitialReplicas is the number of replicas to be placed in the current
	// cluster during the initial stage.
	//
	// If the number is greater than zero, Karmada will place specified amount
	// of replicas in the cluster and then delegate the replica management to
	// the HPA running within the cluster.
	//
	// If the number is not set or set to 0, the cluster will act as a backup
	// cluster and no replicas will be placed during the initial stage. Furthermore,
	// the cluster might be activated when the load raised on other clusters, see
	// Expansion for more details.
	//
	// The value must be greater than or equal to MinReplicas and less than or
	// equal to MaxReplicas.
	// +optional
	InitialReplicas *int32 `json:"initialReplicas,omitempty"`

	// Expansion is the threshold of when to expand the system capacities to
	// get ready for the traffic peak. When the threshold is reached, Karmada
	// will attempt to scale up replicas on the backup clusters.
	//
	// Value can be an absolute number(ex: 10) or a percentage of MaxReplicas
	// (ex: 80%). Absolute number is calculated from percentage by rounding down.
	// It defaults to 80%. The number must be less than or equal to MaxReplicas,
	// and greater than Shrink.
	//
	// Note that Expansion only responsible for scale up replicas based on system
	// load, and does not automatically add the endpoint of the replica to
	// specific load balancer that requires another mechanism(like MultiClusterIngress)
	// to ensure that, otherwise the newly scaled replicas will be idle and
	// unable to reduce the upcoming traffic.
	// +optional
	Expansion *intstr.IntOrString `json:"scaleUpToRelayThreshold,omitempty"`

	// Shrink is the threshold of when to reduce the system capacities to release
	// the redundant replicas. When the threshold is reached, Karmada will attempt
	// to scale down replicas from the backup clusters.
	//
	// Value can be an absolute number(ex: 10) or a percentage of MaxReplicas
	// (ex: 50%). Absolute number is calculated from percentage by rounding down.
	// It defaults to 50%. Value must be greater than or equal to MinReplicas
	// and less than Expansion.
	//
	// Note that Shrink only responsible for scale down replicas based on system
	// load, and does not automatically remove the endpoint of the replica from
	// specific load balancer that requires another mechanism(like MultiClusterIngress)
	// to ensure that, otherwise the endpoint of released replicas will still
	// remain in load balancer which might lead to request failures.
	// +optional
	Shrink *intstr.IntOrString `json:"scaleDownFromRelayThreshold,omitempty"`
}

// HPACoordinatorStatus represents the most recently observed status.
type HPACoordinatorStatus struct {
	// ScaleStatus represents the list of HPAs running in each member
	// cluster.
	// +optional
	ScaleStatus []ScaleStatusItem `json:"scaleStatus,omitempty"`
}

// ScaleStatusItem represents the status of the HPA in specific cluster.
type ScaleStatusItem struct {
	// ClusterName represents the member cluster name.
	// +required
	ClusterName string `json:"clusterName"`

	// CurrentReplicas is current number of replicas of pods managed by this
	// HPA, as last seen by the HPA.
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas of pods managed by this
	// HPA, as last calculated by the HPA.
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// CurrentMetrics is the last read state of the metrics used by this HPA.
	// +optional
	CurrentMetrics []autoscalingv2.MetricStatus `json:"currentMetrics,omitempty"`

	// Conditions is the set of conditions required for this HPA to scale
	// its target, and indicates whether those conditions are met.
	// +optional
	Conditions []autoscalingv2.HorizontalPodAutoscalerCondition `json:"conditions,omitempty"`
}
