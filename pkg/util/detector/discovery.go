package detector

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
)

// GetDeletableResources returns all resources from discoveryClient.
// More specifically, all preferred resources which support the 'delete', 'list', and 'watch' verbs.
//
// All discovery errors are considered temporary. Upon encountering any error,
// GetDeletableResources will log and return any discovered resources it was
// able to process (which may be none).
//
// This code is directly lifted from the Kubernetes codebase.
// For reference: https://github.com/kubernetes/kubernetes/blob/1e11e4a2108024935ecfcb2912226cedeafd99df/pkg/controller/garbagecollector/garbagecollector.go#L638-L667
func GetDeletableResources(discoveryClient discovery.ServerResourcesInterface) map[schema.GroupVersionResource]struct{} {
	preferredResources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			klog.Warningf("failed to discover some groups: %v", err.(*discovery.ErrGroupDiscoveryFailed).Groups)
		} else {
			klog.Warningf("failed to discover preferred resources: %v", err)
		}
	}
	if preferredResources == nil {
		return map[schema.GroupVersionResource]struct{}{}
	}

	// This is extracted from discovery.GroupVersionResources to allow tolerating
	// failures on a per-resource basis.
	deletableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"delete", "list", "watch"}}, preferredResources)
	deletableGroupVersionResources := map[schema.GroupVersionResource]struct{}{}
	for _, rl := range deletableResources {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			klog.Warningf("ignoring invalid discovered resource %q: %v", rl.GroupVersion, err)
			continue
		}
		for i := range rl.APIResources {
			deletableGroupVersionResources[schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: rl.APIResources[i].Name}] = struct{}{}
		}
	}

	return deletableGroupVersionResources
}
