package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindResourceInterpreterWebhookConfiguration is kind name of ResourceInterpreterWebhookConfiguration.
	ResourceKindResourceInterpreterWebhookConfiguration = "ResourceInterpreterWebhookConfiguration"
	// ResourceSingularResourceInterpreterWebhookConfiguration is singular name of ResourceInterpreterWebhookConfiguration.
	ResourceSingularResourceInterpreterWebhookConfiguration = "resourceinterpreterwebhookconfiguration"
	// ResourcePluralResourceInterpreterWebhookConfiguration is plural name of ResourceInterpreterWebhookConfiguration.
	ResourcePluralResourceInterpreterWebhookConfiguration = "resourceinterpreterwebhookconfigurations"
	// ResourceNamespaceScopedResourceInterpreterWebhookConfiguration indicates if ResourceInterpreterWebhookConfiguration is NamespaceScoped.
	ResourceNamespaceScopedResourceInterpreterWebhookConfiguration = false
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:storageversion

// ResourceInterpreterWebhookConfiguration describes the configuration of webhooks which take the responsibility to
// tell karmada the details of the resource object, especially for custom resources.
type ResourceInterpreterWebhookConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Webhooks is a list of webhooks and the affected resources and operations.
	// +required
	Webhooks []ResourceInterpreterWebhook `json:"webhooks"`
}

// ResourceInterpreterWebhook describes the webhook as well as the resources and operations it applies to.
type ResourceInterpreterWebhook struct {
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

	// TimeoutSeconds specifies the timeout for this webhook. After the timeout passes,
	// the webhook call will be ignored or the API call will fail based on the
	// failure policy.
	// The timeout value must be between 1 and 30 seconds.
	// Default to 10 seconds.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// InterpreterContextVersions is an ordered list of preferred `ResourceInterpreterContext`
	// versions the Webhook expects. Karmada will try to use first version in
	// the list which it supports. If none of the versions specified in this list
	// supported by Karmada, validation will fail for this object.
	// If a persisted webhook configuration specifies allowed versions and does not
	// include any versions known to the Karmada, calls to the webhook will fail
	// and be subject to the failure policy.
	InterpreterContextVersions []string `json:"interpreterContextVersions"`
}

// RuleWithOperations is a tuple of Operations and Resources. It is recommended to make
// sure that all the tuple expansions are valid.
type RuleWithOperations struct {
	// Operations is the operations the hook cares about.
	// If '*' is present, the length of the slice must be one.
	// +required
	Operations []InterpreterOperation `json:"operations"`

	// Rule is embedded, it describes other criteria of the rule, like
	// APIGroups, APIVersions, Kinds, etc.
	Rule `json:",inline"`
}

// InterpreterOperation specifies an operation for a request.
type InterpreterOperation string

const (
	// InterpreterOperationAll indicates math all InterpreterOperation.
	InterpreterOperationAll InterpreterOperation = "*"

	// InterpreterOperationInterpretReplica indicates that karmada want to figure out the replica declaration of a specific object.
	// Only necessary for those resource types that have replica declaration, like Deployment or similar custom resources.
	InterpreterOperationInterpretReplica InterpreterOperation = "InterpretReplica"

	// InterpreterOperationReviseReplica indicates that karmada request webhook to modify the replica.
	InterpreterOperationReviseReplica InterpreterOperation = "ReviseReplica"

	// InterpreterOperationInterpretStatus indicates that karmada want to figure out how to get the status.
	// Only necessary for those resource types that define their status in a special path(not '.status').
	InterpreterOperationInterpretStatus InterpreterOperation = "InterpretStatus"

	// InterpreterOperationPrune indicates that karmada want to figure out how to package resource template to Work.
	InterpreterOperationPrune InterpreterOperation = "Prune"

	// InterpreterOperationRetain indicates that karmada request webhook to retain the desired resource template.
	// Only necessary for those resources which specification will be updated by their controllers running in member cluster.
	InterpreterOperationRetain InterpreterOperation = "Retain"

	// InterpreterOperationAggregateStatus indicates that karmada want to figure out how to aggregate status to resource template.
	// Only necessary for those resource types that want to aggregate status to resource template.
	InterpreterOperationAggregateStatus InterpreterOperation = "AggregateStatus"

	// InterpreterOperationInterpretHealth indicates that karmada want to figure out the health status of a specific object.
	// Only necessary for those resource types that have and want to reflect their health status.
	InterpreterOperationInterpretHealth InterpreterOperation = "InterpretHealth"

	// InterpreterOperationInterpretDependency indicates that karmada want to figure out the dependencies of a specific object.
	// Only necessary for those resource types that have dependencies resources and expect the dependencies be propagated
	// together, like Deployment depends on ConfigMap/Secret.
	InterpreterOperationInterpretDependency InterpreterOperation = "InterpretDependency"
)

// Rule is a tuple of APIGroups, APIVersion, and Kinds.
type Rule struct {
	// APIGroups is the API groups the resources belong to. '*' is all groups.
	// If '*' is present, the length of the slice must be one.
	// For example:
	//  ["apps", "batch", "example.io"] means matches 3 groups.
	//  ["*"] means matches all group
	//
	// Note: The group could be empty, e.g the 'core' group of kubernetes, in that case use [""].
	// +required
	APIGroups []string `json:"apiGroups"`

	// APIVersions is the API versions the resources belong to. '*' is all versions.
	// If '*' is present, the length of the slice must be one.
	// For example:
	//  ["v1alpha1", "v1beta1"] means matches 2 versions.
	//  ["*"] means matches all versions.
	// +required
	APIVersions []string `json:"apiVersions"`

	// Kinds is a list of resources this rule applies to.
	// If '*' is present, the length of the slice must be one.
	// For example:
	//  ["Deployment", "Pod"] means matches Deployment and Pod.
	//  ["*"] means apply to all resources.
	// +required
	Kinds []string `json:"kinds"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceInterpreterWebhookConfigurationList contains a list of ResourceInterpreterWebhookConfiguration.
type ResourceInterpreterWebhookConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of ResourceInterpreterWebhookConfiguration.
	Items []ResourceInterpreterWebhookConfiguration `json:"items"`
}
