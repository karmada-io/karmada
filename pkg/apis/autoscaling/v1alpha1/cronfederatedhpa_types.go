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
// +kubebuilder:resource:shortName=cronfhpa,categories={karmada-io}
// +kubebuilder:printcolumn:JSONPath=`.spec.scaleTargetRef.kind`,name=`REFERENCE-KIND`,type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.scaleTargetRef.name`,name=`REFERENCE-NAME`,type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name=`AGE`,type=date

// CronFederatedHPA represents a collection of repeating schedule to scale
// replica number of a specific workload. It can scale any resource implementing
// the scale subresource as well as FederatedHPA.
type CronFederatedHPA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the CronFederatedHPA.
	// +required
	Spec CronFederatedHPASpec `json:"spec"`

	// Status is the current status of the CronFederatedHPA.
	// +optional
	Status CronFederatedHPAStatus `json:"status"`
}

// CronFederatedHPASpec is the specification of the CronFederatedHPA.
type CronFederatedHPASpec struct {
	// ScaleTargetRef points to the target resource to scale.
	// Target resource could be any resource that implementing the scale
	// subresource like Deployment, or FederatedHPA.
	// +required
	ScaleTargetRef autoscalingv2.CrossVersionObjectReference `json:"scaleTargetRef"`

	// Rules contains a collection of schedules that declares when and how
	// the referencing target resource should be scaled.
	// +required
	Rules []CronFederatedHPARule `json:"rules"`
}

// CronFederatedHPARule declares a schedule as well as scale actions.
type CronFederatedHPARule struct {
	// Name of the rule.
	// Each rule in a CronFederatedHPA must have a unique name.
	//
	// Note: the name will be used as an identifier to record its execution
	// history. Changing the name will be considered as deleting the old rule
	// and adding a new rule, that means the original execution history will be
	// discarded.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +required
	Name string `json:"name"`

	// Schedule is the cron expression that represents a periodical time.
	// The syntax follows https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#schedule-syntax.
	// +required
	Schedule string `json:"schedule"`

	// TargetReplicas is the target replicas to be scaled for resources
	// referencing by ScaleTargetRef of this CronFederatedHPA.
	// Only needed when referencing resource is not FederatedHPA.
	// +optional
	TargetReplicas *int32 `json:"targetReplicas,omitempty"`

	// TargetMinReplicas is the target MinReplicas to be set for FederatedHPA.
	// Only needed when referencing resource is FederatedHPA.
	// TargetMinReplicas and TargetMaxReplicas can be specified together or
	// either one can be specified alone.
	// nil means the MinReplicas(.spec.minReplicas) of the referencing FederatedHPA
	// will not be updated.
	// +optional
	TargetMinReplicas *int32 `json:"targetMinReplicas,omitempty"`

	// TargetMaxReplicas is the target MaxReplicas to be set for FederatedHPA.
	// Only needed when referencing resource is FederatedHPA.
	// TargetMinReplicas and TargetMaxReplicas can be specified together or
	// either one can be specified alone.
	// nil means the MaxReplicas(.spec.maxReplicas) of the referencing FederatedHPA
	// will not be updated.
	// +optional
	TargetMaxReplicas *int32 `json:"targetMaxReplicas,omitempty"`

	// Suspend tells the controller to suspend subsequent executions.
	// Defaults to false.
	//
	// +kubebuilder:default=false
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// TimeZone for the giving schedule.
	// If not specified, this will default to the time zone of the
	// karmada-controller-manager process.
	// Invalid TimeZone will be rejected when applying by karmada-webhook.
	// see https://en.wikipedia.org/wiki/List_of_tz_database_time_zones for the
	// all timezones.
	// +optional
	TimeZone *string `json:"timeZone,omitempty"`

	// SuccessfulHistoryLimit represents the count of successful execution items
	// for each rule.
	// The value must be a positive integer. It defaults to 3.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	// +optional
	SuccessfulHistoryLimit *int32 `json:"successfulHistoryLimit,omitempty"`

	// FailedHistoryLimit represents the count of failed execution items for
	// each rule.
	// The value must be a positive integer. It defaults to 3.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	FailedHistoryLimit *int32 `json:"failedHistoryLimit,omitempty"`
}

// CronFederatedHPAStatus represents the current status of a CronFederatedHPA.
type CronFederatedHPAStatus struct {
	// ExecutionHistories record the execution histories of CronFederatedHPARule.
	// +optional
	ExecutionHistories []ExecutionHistory `json:"executionHistories,omitempty"`
}

// ExecutionHistory records the execution history of specific CronFederatedHPARule.
type ExecutionHistory struct {
	// RuleName is the name of the CronFederatedHPARule.
	// +required
	RuleName string `json:"ruleName"`

	// NextExecutionTime is the next time to execute.
	// Nil means the rule has been suspended.
	// +optional
	NextExecutionTime *metav1.Time `json:"nextExecutionTime,omitempty"`

	// SuccessfulExecutions records successful executions.
	// +optional
	SuccessfulExecutions []SuccessfulExecution `json:"successfulExecutions,omitempty"`

	// FailedExecutions records failed executions.
	// +optional
	FailedExecutions []FailedExecution `json:"failedExecutions,omitempty"`
}

// SuccessfulExecution records a successful execution.
type SuccessfulExecution struct {
	// ScheduleTime is the expected execution time declared in CronFederatedHPARule.
	// +required
	ScheduleTime *metav1.Time `json:"scheduleTime"`

	// ExecutionTime is the actual execution time of CronFederatedHPARule.
	// Tasks may not always be executed at ScheduleTime. ExecutionTime is used
	// to evaluate the efficiency of the controller's execution.
	// +required
	ExecutionTime *metav1.Time `json:"executionTime"`

	// AppliedReplicas is the replicas have been applied.
	// It is required if .spec.rules[*].targetReplicas is not empty.
	// +optional
	AppliedReplicas *int32 `json:"appliedReplicas,omitempty"`

	// AppliedMaxReplicas is the MaxReplicas have been applied.
	// It is required if .spec.rules[*].targetMaxReplicas is not empty.
	// +optional
	AppliedMaxReplicas *int32 `json:"appliedMaxReplicas,omitempty"`

	// AppliedMinReplicas is the MinReplicas have been applied.
	// It is required if .spec.rules[*].targetMinReplicas is not empty.
	// +optional
	AppliedMinReplicas *int32 `json:"appliedMinReplicas,omitempty"`
}

// FailedExecution records a failed execution.
type FailedExecution struct {
	// ScheduleTime is the expected execution time declared in CronFederatedHPARule.
	// +required
	ScheduleTime *metav1.Time `json:"scheduleTime"`

	// ExecutionTime is the actual execution time of CronFederatedHPARule.
	// Tasks may not always be executed at ScheduleTime. ExecutionTime is used
	// to evaluate the efficiency of the controller's execution.
	// +required
	ExecutionTime *metav1.Time `json:"executionTime"`

	// Message is the human-readable message indicating details about the failure.
	// +required
	Message string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CronFederatedHPAList contains a list of CronFederatedHPA.
type CronFederatedHPAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronFederatedHPA `json:"items"`
}
