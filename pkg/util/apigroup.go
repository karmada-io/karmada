package util

import (
	"fmt"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// SkippedResourceConfig represents the configuration that identifies the API resources should be skipped from propagating.
type SkippedResourceConfig struct {
	// Groups holds a collection of API group, all resources under this group will be skipped.
	Groups map[string]struct{}
	// GroupVersions holds a collection of API GroupVersion, all resource under this GroupVersion will be skipped.
	GroupVersions map[schema.GroupVersion]struct{}
	// GroupVersionKinds holds a collection of resource that should be skipped.
	GroupVersionKinds map[schema.GroupVersionKind]struct{}
}

var corev1EventGVK = schema.GroupVersionKind{
	Group:   "",
	Version: "v1",
	Kind:    "Event",
}

// NewSkippedResourceConfig to create SkippedResourceConfig
func NewSkippedResourceConfig() *SkippedResourceConfig {
	r := &SkippedResourceConfig{
		Groups:            map[string]struct{}{},
		GroupVersions:     map[schema.GroupVersion]struct{}{},
		GroupVersionKinds: map[schema.GroupVersionKind]struct{}{},
	}
	// disable karmada group by default
	r.DisableGroup(clusterv1alpha1.GroupVersion.Group)
	r.DisableGroup(policyv1alpha1.GroupVersion.Group)
	r.DisableGroup(workv1alpha1.GroupVersion.Group)
	r.DisableGroup(configv1alpha1.GroupVersion.Group)
	r.DisableGroup(networkingv1alpha1.GroupVersion.Group)
	r.DisableGroup(autoscalingv1alpha1.GroupVersion.Group)
	// disable event by default
	r.DisableGroup(eventsv1.GroupName)
	r.DisableGroupVersionKind(corev1EventGVK)

	// disable Lease by default
	r.DisableGroupVersion(coordinationv1.SchemeGroupVersion)
	return r
}

// Parse parses the --skipped-propagating-apis input.
func (r *SkippedResourceConfig) Parse(c string) error {
	// default(empty) input
	if c == "" {
		return nil
	}

	tokens := strings.Split(c, ";")
	for _, token := range tokens {
		if err := r.parseSingle(token); err != nil {
			return fmt.Errorf("parse --skipped-propagating-apis %w", err)
		}
	}

	return nil
}

func (r *SkippedResourceConfig) parseSingle(token string) error {
	switch strings.Count(token, "/") {
	// Assume user don't want to skip the 'core'(no group name) group.
	// So, it should be the case "<group>".
	case 0:
		r.Groups[token] = struct{}{}
	// it should be the case "<group>/<version>"
	case 1:
		// for core group which don't have the group name, the case should be "v1/<kind>" or "v1/<kind>,<kind>..."
		if strings.HasPrefix(token, "v1") {
			var kinds []string
			for _, k := range strings.Split(token, ",") {
				if strings.Contains(k, "/") { // "v1/<kind>"
					s := strings.Split(k, "/")
					kinds = append(kinds, s[1])
				} else {
					kinds = append(kinds, k)
				}
			}
			for _, k := range kinds {
				gvk := schema.GroupVersionKind{
					Version: "v1",
					Kind:    k,
				}
				r.GroupVersionKinds[gvk] = struct{}{}
			}
		} else { // case "<group>/<version>"
			parts := strings.Split(token, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid token: %s", token)
			}
			gv := schema.GroupVersion{
				Group:   parts[0],
				Version: parts[1],
			}
			r.GroupVersions[gv] = struct{}{}
		}
	// parameter format: "<group>/<version>/<kind>" or "<group>/<version>/<kind>,<kind>..."
	case 2:
		g := ""
		v := ""
		var kinds []string
		for _, k := range strings.Split(token, ",") {
			if strings.Contains(k, "/") {
				s := strings.Split(k, "/")
				g = s[0]
				v = s[1]
				kinds = append(kinds, s[2])
			} else {
				kinds = append(kinds, k)
			}
		}
		for _, k := range kinds {
			gvk := schema.GroupVersionKind{
				Group:   g,
				Version: v,
				Kind:    k,
			}
			r.GroupVersionKinds[gvk] = struct{}{}
		}
	default:
		return fmt.Errorf("invalid parameter: %s", token)
	}

	return nil
}

// GroupVersionDisabled returns whether GroupVersion is disabled.
func (r *SkippedResourceConfig) GroupVersionDisabled(gv schema.GroupVersion) bool {
	if _, ok := r.GroupVersions[gv]; ok {
		return true
	}
	return false
}

// GroupVersionKindDisabled returns whether GroupVersionKind is disabled.
func (r *SkippedResourceConfig) GroupVersionKindDisabled(gvk schema.GroupVersionKind) bool {
	if _, ok := r.GroupVersionKinds[gvk]; ok {
		return true
	}
	return false
}

// GroupDisabled returns whether Group is disabled.
func (r *SkippedResourceConfig) GroupDisabled(g string) bool {
	if _, ok := r.Groups[g]; ok {
		return true
	}
	return false
}

// DisableGroup to disable group.
func (r *SkippedResourceConfig) DisableGroup(g string) {
	r.Groups[g] = struct{}{}
}

// DisableGroupVersion to disable GroupVersion.
func (r *SkippedResourceConfig) DisableGroupVersion(gv schema.GroupVersion) {
	r.GroupVersions[gv] = struct{}{}
}

// DisableGroupVersionKind to disable GroupVersionKind.
func (r *SkippedResourceConfig) DisableGroupVersionKind(gvk schema.GroupVersionKind) {
	r.GroupVersionKinds[gvk] = struct{}{}
}
