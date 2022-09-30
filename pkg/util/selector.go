package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ImplicitPriority describes the extent to which a ResourceSelector or a set of
// ResourceSelectors match resources.
type ImplicitPriority int

const (
	// PriorityMisMatch means the ResourceSelector does not match the resource.
	PriorityMisMatch ImplicitPriority = iota
	// PriorityMatchAll means the ResourceSelector whose Name and LabelSelector is empty
	// matches the resource.
	PriorityMatchAll
	// PriorityMatchLabelSelector means the LabelSelector of ResourceSelector matches the resource.
	PriorityMatchLabelSelector
	// PriorityMatchName means the Name of ResourceSelector matches the resource.
	PriorityMatchName
)

// ResourceMatches tells if the specific resource matches the selector.
func ResourceMatches(resource *unstructured.Unstructured, rs policyv1alpha1.ResourceSelector) bool {
	return ResourceSelectorPriority(resource, rs) > PriorityMisMatch
}

// ResourceSelectorPriority tells the priority between the specific resource and the selector.
func ResourceSelectorPriority(resource *unstructured.Unstructured, rs policyv1alpha1.ResourceSelector) ImplicitPriority {
	if resource.GetAPIVersion() != rs.APIVersion ||
		resource.GetKind() != rs.Kind ||
		(len(rs.Namespace) > 0 && resource.GetNamespace() != rs.Namespace) {
		return PriorityMisMatch
	}

	// match rules:
	// case ResourceSelector.name   ResourceSelector.labelSelector   Rule
	// 1    not-empty               not-empty                        match name only and ignore selector
	// 2    not-empty               empty                            match name only
	// 3    empty                   not-empty                        match selector only
	// 4    empty                   empty                            match all

	// case 1, 2: name not empty, don't need to consult selector.
	if len(rs.Name) > 0 {
		if rs.Name == resource.GetName() {
			return PriorityMatchName
		}
		return PriorityMisMatch
	}

	// case 4: short path, both name and selector empty, matches all
	if rs.LabelSelector == nil {
		return PriorityMatchAll
	}

	// case 3: matches with selector
	var s labels.Selector
	var err error
	if s, err = metav1.LabelSelectorAsSelector(rs.LabelSelector); err != nil {
		// should not happen because all resource selector should be fully validated by webhook.
		return PriorityMisMatch
	}

	if s.Matches(labels.Set(resource.GetLabels())) {
		return PriorityMatchLabelSelector
	}
	return PriorityMisMatch
}

// ClusterMatches tells if specific cluster matches the affinity.
func ClusterMatches(cluster *clusterv1alpha1.Cluster, affinity policyv1alpha1.ClusterAffinity) bool {
	for _, clusterName := range affinity.ExcludeClusters {
		if clusterName == cluster.Name {
			return false
		}
	}

	// match rules:
	// case LabelSelector   ClusterNames   	FieldSelector       Rule
	// 1    not-empty        not-empty       not-empty          match selector, name and field
	// 2    not-empty        empty           empty              match selector only
	// 3    not-empty        not-empty       empty              match selector, name
	// 4    not-empty        empty       	 not-empty          match selector, filed
	// 5    empty            not-empty       not-empty          match name, filed
	// 6    empty            not-empty       empty              match name only
	// 7    empty            empty       	 not-empty          match field only
	// 8    empty            empty           empty              match all

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

	if affinity.FieldSelector != nil {
		var matchFields labels.Selector
		var err error
		if matchFields, err = lifted.NodeSelectorRequirementsAsSelector(affinity.FieldSelector.MatchExpressions); err != nil {
			return false
		}
		clusterFields := extractClusterFields(cluster)
		if matchFields != nil && !matchFields.Matches(clusterFields) {
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

// ResourceMatchSelectors tells if the specific resource matches the selectors.
func ResourceMatchSelectors(resource *unstructured.Unstructured, selectors ...policyv1alpha1.ResourceSelector) bool {
	for _, rs := range selectors {
		if ResourceMatches(resource, rs) {
			return true
		}
	}
	return false
}

// ResourceMatchSelectorsPriority returns the highest priority between specific resource and the selectors.
func ResourceMatchSelectorsPriority(resource *unstructured.Unstructured, selectors ...policyv1alpha1.ResourceSelector) ImplicitPriority {
	var priority ImplicitPriority
	for _, rs := range selectors {
		if p := ResourceSelectorPriority(resource, rs); p > priority {
			priority = p
		}
	}
	return priority
}

func extractClusterFields(cluster *clusterv1alpha1.Cluster) labels.Set {
	clusterFieldsMap := make(labels.Set)

	if cluster.Spec.Provider != "" {
		clusterFieldsMap[ProviderField] = cluster.Spec.Provider
	}

	if cluster.Spec.Region != "" {
		clusterFieldsMap[RegionField] = cluster.Spec.Region
	}

	if cluster.Spec.Zone != "" {
		clusterFieldsMap[ZoneField] = cluster.Spec.Zone
	}

	return clusterFieldsMap
}
