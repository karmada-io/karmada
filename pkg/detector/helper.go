package detector

import (
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

var (
	propagationPolicyRefLabels = []string{policyv1alpha1.PropagationPolicyNamespaceLabel, policyv1alpha1.PropagationPolicyNameLabel, policyv1alpha1.PropagationPolicyPermanentIDLabel}

	clusterPropagationPolicyRefLabels = []string{policyv1alpha1.ClusterPropagationPolicyLabel, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel}
)

// RemoveMapKeysFunc defines the func to remove some keys.
type RemoveMapKeysFunc func(map[string]string) map[string]string
