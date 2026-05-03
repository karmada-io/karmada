/*
Copyright The Karmada Authors.

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

const (
	// ResourceKindSchedulerQueue is kind name of SchedulerQueue.
	ResourceKindSchedulerQueue = "SchedulerQueue"
	// ResourceSingularSchedulerQueue is singular name of SchedulerQueue.
	ResourceSingularSchedulerQueue = "schedulerqueue"
	// ResourcePluralSchedulerQueue is plural name of SchedulerQueue.
	ResourcePluralSchedulerQueue = "schedulerqueues"
	// ResourceNamespaceScopedSchedulerQueue indicates if SchedulerQueue is NamespaceScoped.
	ResourceNamespaceScopedSchedulerQueue = false
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=schedulerqueues,scope="Cluster",shortName=sq,categories={karmada-io}
// +kubebuilder:storageversion

// SchedulerQueue configures per-tenant scheduling queue settings.
// It is cluster-scoped so that only cluster admins can create it.
// ResourceBindings in namespaces matching the NamespaceSelector are
// routed to this queue for scheduling.
type SchedulerQueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired queue configuration.
	// +required
	Spec SchedulerQueueSpec `json:"spec"`
}

// SchedulerQueueSpec defines the configuration for a tenant's scheduling queue.
type SchedulerQueueSpec struct {
	// NamespaceSelector selects the namespaces whose ResourceBindings
	// this queue governs.
	// +required
	NamespaceSelector NamespaceSelector `json:"namespaceSelector"`

	// QueueingStrategy controls the ordering and blocking behavior of
	// bindings in the active queue.
	// +kubebuilder:default=BestEffortFIFO
	// +kubebuilder:validation:Enum=BestEffortFIFO;StrictFIFO
	// +optional
	QueueingStrategy QueueingStrategy `json:"queueingStrategy,omitempty"`

	// BackoffConfig tunes the retry backoff for this tenant's backoff queue.
	// +optional
	BackoffConfig *BackoffConfig `json:"backoffConfig,omitempty"`

	// UnschedulableConfig tunes how long bindings may sit in the
	// unschedulable set before being flushed back to the active queue.
	// +optional
	UnschedulableConfig *UnschedulableConfig `json:"unschedulableConfig,omitempty"`
}

// NamespaceSelector selects namespaces for a tenant queue.
type NamespaceSelector struct {
	// Names is a list of exact namespace names.
	// +optional
	Names []string `json:"names,omitempty"`

	// MatchLabels selects namespaces by label. Reserved for Phase 2.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// QueueingStrategy determines how bindings are ordered and whether
// head-of-line blocking is applied.
type QueueingStrategy string

const (
	// BestEffortFIFO orders bindings by priority, breaking ties by
	// creation timestamp. When the head of the queue cannot be scheduled,
	// the scheduler skips it and tries the next binding.
	BestEffortFIFO QueueingStrategy = "BestEffortFIFO"

	// StrictFIFO orders bindings by priority, breaking ties by creation
	// timestamp. If the head-of-queue binding cannot be scheduled, no later
	// binding in the same tenant is attempted until the head is resolved
	// (head-of-line blocking).
	StrictFIFO QueueingStrategy = "StrictFIFO"
)

// BackoffConfig controls exponential backoff for failed scheduling attempts.
type BackoffConfig struct {
	// InitialDuration is the backoff duration for the first retry.
	// +kubebuilder:default="1s"
	// +optional
	InitialDuration *metav1.Duration `json:"initialDuration,omitempty"`

	// MaxDuration is the maximum backoff duration.
	// +kubebuilder:default="10s"
	// +optional
	MaxDuration *metav1.Duration `json:"maxDuration,omitempty"`
}

// UnschedulableConfig controls how long a binding waits in the
// unschedulable set before being re-queued.
type UnschedulableConfig struct {
	// MaxDuration is the maximum time a binding may remain in the
	// unschedulable set. After this, it is moved to the backoff or active queue.
	// +kubebuilder:default="5m"
	// +optional
	MaxDuration *metav1.Duration `json:"maxDuration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulerQueueList contains a list of SchedulerQueue.
type SchedulerQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulerQueue `json:"items"`
}
