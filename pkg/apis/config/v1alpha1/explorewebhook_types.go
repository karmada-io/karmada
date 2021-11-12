package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:storageversion

// ResourceExploringWebhookConfiguration describes the configuration of webhooks which take the responsibility to
// tell karmada the details of the resource object, especially for custom resources.
type ResourceExploringWebhookConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Webhooks is a list of webhooks and the affected resources and operations.
	// +required
	Webhooks []ResourceExploringWebhook `json:"webhooks"`
}

// ResourceExploringWebhook describes the webhook as well as the resources and operations it applies to.
type ResourceExploringWebhook struct {
	// Name is the full-qualified name of the webhook.
	// +required
	Name string `json:"name"`

	// ClientConfig defines how to communicate with the hook.
	// +required
	ClientConfig admissionregistrationv1.WebhookClientConfig `json:"clientConfig"`

	// Rules describes what operations on what resources the webhook cares about.
	// The webhook cares about an operation if it matches any Rule.
	// +optional
	Rules []RuleWithOperations `json:"rules,omitempty"`

	// FailurePolicy defines how unrecognized errors from the webhook are handled,
	// allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	FailurePolicy *admissionregistrationv1.FailurePolicyType `json:"failurePolicy,omitempty"`

	// TimeoutSeconds specifies the timeout for this webhook. After the timeout passes,
	// the webhook call will be ignored or the API call will fail based on the
	// failure policy.
	// The timeout value must be between 1 and 30 seconds.
	// Default to 10 seconds.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// ExploreReviewVersions is an ordered list of preferred `ExploreReview`
	// versions the Webhook expects. Karmada will try to use first version in
	// the list which it supports. If none of the versions specified in this list
	// supported by Karmada, validation will fail for this object.
	// If a persisted webhook configuration specifies allowed versions and does not
	// include any versions known to the Karmada, calls to the webhook will fail
	// and be subject to the failure policy.
	ExploreReviewVersions []string `json:"exploreReviewVersions"`
}

// RuleWithOperations is a tuple of Operations and Resources. It is recommended to make
// sure that all the tuple expansions are valid.
type RuleWithOperations struct {
	// Operations is the operations the hook cares about.
	// If '*' is present, the length of the slice must be one.
	// +required
	Operations []OperationType `json:"operations"`

	// Rule is embedded, it describes other criteria of the rule, like
	// APIGroups, APIVersions, Resources, etc.
	admissionregistrationv1.Rule `json:",inline"`
}

// OperationType specifies an operation for a request.
type OperationType string

const (
	// OperationAll indicates math all OperationType.
	OperationAll OperationType = "*"

	// ExploreReplica indicates that karmada want to figure out the replica declaration of a specific object.
	// Only necessary for those resource types that have replica declaration, like Deployment or similar custom resources.
	ExploreReplica OperationType = "ExploreReplica"

	// ExploreStatus indicates that karmada want to figure out how to get the status.
	// Only necessary for those resource types that define their status in a special path(not '.status').
	ExploreStatus OperationType = "ExploreStatus"

	// ExplorePacking indicates that karmada want to figure out how to package resource template to Work.
	ExplorePacking OperationType = "ExplorePacking"

	// ExploreReplicaRevising indicates that karmada request webhook to modify the replica.
	ExploreReplicaRevising OperationType = "ExploreReplicaRevising"

	// ExploreRetaining indicates that karmada request webhook to retain the desired resource template.
	// Only necessary for those resources which specification will be updated by their controllers running in member cluster.
	ExploreRetaining OperationType = "ExploreRetaining"

	// ExploreStatusAggregating indicates that karmada want to figure out how to aggregate status to resource template.
	// Only necessary for those resource types that want to aggregate status to resource template.
	ExploreStatusAggregating OperationType = "ExploreStatusAggregating"

	// ExploreHealthy indicates that karmada want to figure out the healthy status of a specific object.
	// Only necessary for those resource types that have and want to reflect their healthy status.
	ExploreHealthy OperationType = "ExploreHealthy"

	// ExploreDependencies indicates that karmada want to figure out the dependencies of a specific object.
	// Only necessary for those resource types that have dependencies resources and expect the dependencies be propagated
	// together, like Deployment depends on ConfigMap/Secret.
	ExploreDependencies OperationType = "ExploreDependencies"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceExploringWebhookConfigurationList contains a list of ResourceExploringWebhookConfiguration.
type ResourceExploringWebhookConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of ResourceExploringWebhookConfiguration.
	Items []ResourceExploringWebhookConfiguration `json:"items"`
}
