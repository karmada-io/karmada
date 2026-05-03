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
	// ResourceKindTenantQueue is kind name of TenantQueue.
	ResourceKindTenantQueue = "TenantQueue"
	// ResourceSingularTenantQueue is singular name of TenantQueue.
	ResourceSingularTenantQueue = "tenantqueue"
	// ResourcePluralTenantQueue is plural name of TenantQueue.
	ResourcePluralTenantQueue = "tenantqueues"
	// ResourceNamespaceScopedTenantQueue indicates if TenantQueue is NamespaceScoped.
	ResourceNamespaceScopedTenantQueue = true
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=tenantqueues,scope=Namespaced,shortName=tq,categories={karmada-io}
// +kubebuilder:storageversion

// TenantQueue configures per-tenant scheduling queue settings.
// It is namespace-scoped — one TenantQueue per namespace. ResourceBindings
// in the same namespace are routed to this queue for scheduling.
type TenantQueue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired queue configuration.
	// +required
	Spec TenantQueueSpec `json:"spec"`
}

// TenantQueueSpec defines the configuration for a tenant's scheduling queue.
type TenantQueueSpec struct {
	// QueueingStrategy controls the ordering and blocking behavior of
	// bindings in the active queue.
	// +kubebuilder:default=BestEffortFIFO
	// +kubebuilder:validation:Enum=BestEffortFIFO;StrictFIFO
	// +optional
	QueueingStrategy QueueingStrategy `json:"queueingStrategy,omitempty"`
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TenantQueueList contains a list of TenantQueue.
type TenantQueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantQueue `json:"items"`
}
