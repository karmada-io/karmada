package overridemanager

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// OverrideManager managers override policies operation
type OverrideManager interface {
	ApplyOverridePolicies(rawObj *unstructured.Unstructured, cluster string) error
}

type overrideOption struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type overrideManagerImpl struct {
	client.Client
}

// New builds an OverrideManager instance.
func New(client client.Client) OverrideManager {
	return &overrideManagerImpl{
		Client: client,
	}
}

func (o *overrideManagerImpl) ApplyOverridePolicies(rawObj *unstructured.Unstructured, clusterName string) error {
	clusterObj := &clusterv1alpha1.Cluster{}
	if err := o.Client.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
		klog.Errorf("Failed to get member cluster: %s, error: %v", clusterName, err)
		return err
	}

	// Apply cluster scoped override policies
	if err := o.applyClusterOverrides(rawObj, clusterObj); err != nil {
		klog.Errorf("Failed to apply cluster override policies. error: %v", err)
		return err
	}

	// Apply namespace scoped override policies
	if len(rawObj.GetNamespace()) > 0 {
		if err := o.applyNamespacedOverrides(rawObj, clusterObj); err != nil {
			klog.Errorf("Failed to apply namespaced override policies. error: %v", err)
			return err
		}
	}

	return nil
}

// applyClusterOverrides will apply overrides according to ClusterOverridePolicy instructions.
func (o *overrideManagerImpl) applyClusterOverrides(rawObj *unstructured.Unstructured, cluster *clusterv1alpha1.Cluster) error {

	// TODO(RainbowMango): implements later after ClusterOverridePolicy get on board.

	return nil
}

// applyNamespacedOverrides will apply overrides according to OverridePolicy instructions.
func (o *overrideManagerImpl) applyNamespacedOverrides(rawObj *unstructured.Unstructured, cluster *clusterv1alpha1.Cluster) error {
	// get all namespace-scoped override policies
	policyList := &policyv1alpha1.OverridePolicyList{}
	if err := o.Client.List(context.TODO(), policyList, &client.ListOptions{Namespace: rawObj.GetNamespace()}); err != nil {
		klog.Errorf("Failed to list override policies from namespace: %s, error: %v", rawObj.GetNamespace(), err)
		return err
	}

	if len(policyList.Items) == 0 {
		return nil
	}

	matchedPolicies := o.getMatchedOverridePolicy(policyList.Items, rawObj, cluster)
	if len(matchedPolicies) == 0 {
		klog.V(2).Infof("No override policy for resource: %s/%s", rawObj.GetNamespace(), rawObj.GetName())
		return nil
	}

	var appliedList []string
	for _, p := range matchedPolicies {
		klog.Infof("Applying overrides(%s/%s) for %s/%s", p.Namespace, p.Name, rawObj.GetNamespace(), rawObj.GetName())
		if err := applyJSONPatch(rawObj, parseJSONPatch(p.Spec.Overriders.Plaintext)); err != nil {
			return err
		}
		appliedList = append(appliedList, p.Name)
	}
	util.MergeAnnotation(rawObj, util.AppliedOverrideKey, strings.Join(appliedList, ","))

	return nil
}

func (o *overrideManagerImpl) getMatchedOverridePolicy(policies []policyv1alpha1.OverridePolicy, resource *unstructured.Unstructured, cluster *clusterv1alpha1.Cluster) []policyv1alpha1.OverridePolicy {
	// select policy in which at least one resource selector matches target resource.
	resourceMatches := make([]policyv1alpha1.OverridePolicy, 0)
	for _, policy := range policies {
		if OverridePolicyMatches(resource, &policy) {
			resourceMatches = append(resourceMatches, policy)
		}
	}

	// select policy which cluster selector matches target resource.
	clusterMatches := make([]policyv1alpha1.OverridePolicy, 0)
	for _, policy := range resourceMatches {
		if util.ClusterMatches(cluster, policy.Spec.TargetCluster) {
			clusterMatches = append(clusterMatches, policy)
		}
	}

	// select policy in which at least one PlaintextOverrider matches target resource.
	// TODO(RainbowMango): check if the overrider instructions can be applied to target resource.

	// TODO(RainbowMango): Sort by policy names.

	return clusterMatches
}

func parseJSONPatch(overriders []policyv1alpha1.PlaintextOverrider) []overrideOption {
	patches := make([]overrideOption, 0, len(overriders))
	for i := range overriders {
		patches = append(patches, overrideOption{
			Op:    string(overriders[i].Operator),
			Path:  overriders[i].Path,
			Value: overriders[i].Value,
		})
	}
	return patches
}

// applyJSONPatch applies the override on to the given unstructured object.
func applyJSONPatch(obj *unstructured.Unstructured, overrides []overrideOption) error {
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return err
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return err
	}

	objectJSONBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}

	patchedObjectJSONBytes, err := patch.Apply(objectJSONBytes)
	if err != nil {
		return err
	}

	err = obj.UnmarshalJSON(patchedObjectJSONBytes)
	return err
}

// OverridePolicyMatches tells if the override policy should be applied to the resource.
func OverridePolicyMatches(resource *unstructured.Unstructured, policy *policyv1alpha1.OverridePolicy) bool {
	for _, rs := range policy.Spec.ResourceSelectors {
		if util.ResourceMatches(resource, rs) {
			return true
		}
	}

	return false
}
