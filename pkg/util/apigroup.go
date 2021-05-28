package util

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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
	return r
}

// Parse to parse tokens to SkippedResourceConfig
func (r *SkippedResourceConfig) Parse(c string) {
	// v1/Node,Pod;networking.k8s.io/v1beta1/Ingress,IngressClass
	// group networking.k8s.io
	// group version networking.k8s.io/v1
	// group version kind networking.k8s.io/v1/Ingress
	// corev1 has no group
	if c == "" {
		return
	}

	tokens := strings.Split(c, ";")
	for _, token := range tokens {
		r.parseSingle(token)
	}
}

func (r *SkippedResourceConfig) parseSingle(token string) {
	switch strings.Count(token, "/") {
	case 0:
		// Group
		r.Groups[token] = struct{}{}
	case 1:
		if strings.HasPrefix(token, "v1") {
			// core v1
			kinds := []string{}
			for _, k := range strings.Split(token, ",") {
				if strings.Contains(k, "/") {
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
		} else {
			// GroupVersion
			i := strings.Index(token, "/")
			gv := schema.GroupVersion{
				Group:   token[:i],
				Version: token[i+1:],
			}
			r.GroupVersions[gv] = struct{}{}
		}
	case 2:
		// GroupVersionKind
		g := ""
		v := ""
		kinds := []string{}
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
		klog.Error("Unsupported SkippedPropagatingAPIs: ", token)
	}
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
