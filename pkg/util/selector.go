package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// ResourceMatches tells if the specific resource matches the selector.
func ResourceMatches(resource *unstructured.Unstructured, rs v1alpha1.ResourceSelector) bool {
	if resource.GetAPIVersion() != rs.APIVersion ||
		resource.GetKind() != rs.Kind ||
		resource.GetNamespace() != rs.Namespace {
		return false
	}

	// match rules:
	// case ResourceSelector.name   ResourceSelector.labelSelector   Rule
	// 1    not-empty               not-empty                        match name only and ignore selector
	// 2    not-empty               empty                            match name only
	// 3    empty                   not-empty                        match selector only
	// 4    empty                   empty                            match all

	// case 1, 2: name not empty, don't need to consult selector.
	if len(rs.Name) > 0 {
		return rs.Name == resource.GetName()
	}

	// case 4: short path, both name and selector empty, matches all
	if rs.LabelSelector == nil {
		return true
	}

	// case 3: matches with selector
	var s labels.Selector
	var err error
	if s, err = metav1.LabelSelectorAsSelector(rs.LabelSelector); err != nil {
		// should not happen because all resource selector should be fully validated by webhook.
		return false
	}

	return s.Matches(labels.Set(resource.GetLabels()))
}

// ClusterMatches tells if specific cluster matches the affinity.
// TODO(RainbowMango): Now support ClusterAffinity.ClusterNames, ClusterAffinity.ExcludeClusters and ClusterAffinity.LabelSelector. More rules will be implemented later.
func ClusterMatches(cluster *clusterv1alpha1.Cluster, affinity v1alpha1.ClusterAffinity) bool {
	for _, clusterName := range affinity.ExcludeClusters {
		if clusterName == cluster.Name {
			return false
		}
	}

	// match rules:
	// case LabelSelector   ClusterNames   		Rule
	// 1    not-empty        not-empty          match selector and name
	// 2    not-empty        empty              match selector only
	// 3    empty            not-empty          match name only
	// 4    empty            empty              match all

	if affinity.LabelSelector != nil {
		var s labels.Selector
		var err error
		if s, err = metav1.LabelSelectorAsSelector(affinity.LabelSelector); err != nil {
			return false
		}

		if !s.Matches(labels.Set(cluster.GetLabels())) {
			return false
		}
	}

	return ClusterNamesMatches(cluster, affinity.ClusterNames)
}

// ClusterNamesMatches tells if specific cluster matches the clusterNames affinity.
func ClusterNamesMatches(cluster *clusterv1alpha1.Cluster, clusterNames []string) bool {
	if len(clusterNames) == 0 {
		return true
	}

	for _, clusterName := range clusterNames {
		if clusterName == cluster.Name {
			return true
		}
	}
	return false
}
