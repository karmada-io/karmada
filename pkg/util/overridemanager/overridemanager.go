/*
Copyright 2021 The Karmada Authors.

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

package overridemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-openapi/jsonpointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// OverrideManagerName is the manager name that will be used when reporting events.
	OverrideManagerName = "override-manager"
)

// OverrideManager managers override policies operation
type OverrideManager interface {
	// ApplyOverridePolicies overrides the object if one or more override policies exist and matches the target cluster.
	// For cluster scoped resource:
	// - Apply ClusterOverridePolicy by policies name in ascending
	// For namespaced scoped resource, apply order is:
	// - First apply ClusterOverridePolicy;
	// - Then apply OverridePolicy;
	ApplyOverridePolicies(rawObj *unstructured.Unstructured, cluster string) (appliedClusterPolicies *AppliedOverrides, appliedNamespacedPolicies *AppliedOverrides, err error)
}

// GeneralOverridePolicy is an abstract object of ClusterOverridePolicy and OverridePolicy
type GeneralOverridePolicy interface {
	// GetName returns the name of OverridePolicy
	GetName() string
	// GetNamespace returns the namespace of OverridePolicy
	GetNamespace() string
	// GetOverrideSpec returns the OverrideSpec of OverridePolicy
	GetOverrideSpec() policyv1alpha1.OverrideSpec
}

// overrideOption define the JSONPatch operator
type overrideOption struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type policyOverriders struct {
	name       string
	namespace  string
	overriders policyv1alpha1.Overriders
}

type overrideManagerImpl struct {
	client.Client
	record.EventRecorder
}

// New builds an OverrideManager instance.
func New(client client.Client, eventRecorder record.EventRecorder) OverrideManager {
	return &overrideManagerImpl{
		Client:        client,
		EventRecorder: eventRecorder,
	}
}

func (o *overrideManagerImpl) ApplyOverridePolicies(rawObj *unstructured.Unstructured, clusterName string) (*AppliedOverrides, *AppliedOverrides, error) {
	clusterObj := &clusterv1alpha1.Cluster{}
	if err := o.Client.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
		klog.Errorf("Failed to get member cluster: %s, error: %v", clusterName, err)
		return nil, nil, err
	}

	var appliedClusterOverrides *AppliedOverrides
	var appliedNamespacedOverrides *AppliedOverrides
	var err error

	// Apply cluster scoped override policies
	appliedClusterOverrides, err = o.applyClusterOverrides(rawObj, clusterObj)
	if err != nil {
		klog.Errorf("Failed to apply cluster override policies. error: %v", err)
		return nil, nil, err
	}

	// For namespace scoped resources, should apply override policies under the same namespace.
	// No matter the resources propagated by ClusterPropagationPolicy or PropagationPolicy.
	if len(rawObj.GetNamespace()) > 0 {
		// Apply namespace scoped override policies
		appliedNamespacedOverrides, err = o.applyNamespacedOverrides(rawObj, clusterObj)
		if err != nil {
			klog.Errorf("Failed to apply namespaced override policies. error: %v", err)
			return nil, nil, err
		}
	}

	return appliedClusterOverrides, appliedNamespacedOverrides, nil
}

// applyClusterOverrides will apply overrides according to ClusterOverridePolicy instructions.
func (o *overrideManagerImpl) applyClusterOverrides(rawObj *unstructured.Unstructured, cluster *clusterv1alpha1.Cluster) (*AppliedOverrides, error) {
	// get all cluster-scoped override policies
	policyList := &policyv1alpha1.ClusterOverridePolicyList{}
	if err := o.Client.List(context.TODO(), policyList, &client.ListOptions{UnsafeDisableDeepCopy: ptr.To(true)}); err != nil {
		klog.Errorf("Failed to list cluster override policies, error: %v", err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	items := make([]GeneralOverridePolicy, 0, len(policyList.Items))
	for i := range policyList.Items {
		items = append(items, &policyList.Items[i])
	}
	matchingPolicyOverriders := o.getOverridersFromOverridePolicies(items, rawObj, cluster)
	if len(matchingPolicyOverriders) == 0 {
		klog.V(2).Infof("No cluster override policy for resource: %s/%s", rawObj.GetNamespace(), rawObj.GetName())
		return nil, nil
	}

	appliedList := &AppliedOverrides{}
	for _, p := range matchingPolicyOverriders {
		if err := applyPolicyOverriders(rawObj, p.overriders); err != nil {
			klog.Errorf("Failed to apply cluster overrides(%s) for resource(%s/%s), error: %v", p.name, rawObj.GetNamespace(), rawObj.GetName(), err)
			o.EventRecorder.Eventf(rawObj, corev1.EventTypeWarning, events.EventReasonApplyOverridePolicyFailed, "Apply cluster override policy(%s) for cluster(%s) failed.", p.name, cluster.Name)
			return nil, err
		}
		klog.V(2).Infof("Applied cluster overrides(%s) for resource(%s/%s)", p.name, rawObj.GetNamespace(), rawObj.GetName())
		o.EventRecorder.Eventf(rawObj, corev1.EventTypeNormal, events.EventReasonApplyOverridePolicySucceed, "Apply cluster override policy(%s) for cluster(%s) succeed.", p.name, cluster.Name)
		appliedList.Add(p.name, p.overriders)
	}

	return appliedList, nil
}

// applyNamespacedOverrides will apply overrides according to OverridePolicy instructions.
func (o *overrideManagerImpl) applyNamespacedOverrides(rawObj *unstructured.Unstructured, cluster *clusterv1alpha1.Cluster) (*AppliedOverrides, error) {
	// get all namespace-scoped override policies
	policyList := &policyv1alpha1.OverridePolicyList{}
	if err := o.Client.List(context.TODO(), policyList, &client.ListOptions{Namespace: rawObj.GetNamespace(), UnsafeDisableDeepCopy: ptr.To(true)}); err != nil {
		klog.Errorf("Failed to list override policies from namespace: %s, error: %v", rawObj.GetNamespace(), err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	items := make([]GeneralOverridePolicy, 0, len(policyList.Items))
	for i := range policyList.Items {
		items = append(items, &policyList.Items[i])
	}
	matchingPolicyOverriders := o.getOverridersFromOverridePolicies(items, rawObj, cluster)
	if len(matchingPolicyOverriders) == 0 {
		klog.V(2).Infof("No override policy for resource(%s/%s)", rawObj.GetNamespace(), rawObj.GetName())
		return nil, nil
	}

	appliedList := &AppliedOverrides{}
	for _, p := range matchingPolicyOverriders {
		if err := applyPolicyOverriders(rawObj, p.overriders); err != nil {
			klog.Errorf("Failed to apply overrides(%s/%s) for resource(%s/%s), error: %v", p.namespace, p.name, rawObj.GetNamespace(), rawObj.GetName(), err)
			o.EventRecorder.Eventf(rawObj, corev1.EventTypeWarning, events.EventReasonApplyOverridePolicyFailed, "Apply override policy(%s/%s) for cluster(%s) failed.", p.namespace, p.name, cluster.Name)
			return nil, err
		}
		klog.V(2).Infof("Applied overrides(%s/%s) for resource(%s/%s)", p.namespace, p.name, rawObj.GetNamespace(), rawObj.GetName())
		o.EventRecorder.Eventf(rawObj, corev1.EventTypeNormal, events.EventReasonApplyOverridePolicySucceed, "Apply override policy(%s/%s) for cluster(%s) succeed.", p.namespace, p.name, cluster.Name)
		appliedList.Add(p.name, p.overriders)
	}

	return appliedList, nil
}

func (o *overrideManagerImpl) getOverridersFromOverridePolicies(policies []GeneralOverridePolicy, resource *unstructured.Unstructured, cluster *clusterv1alpha1.Cluster) []policyOverriders {
	resourceMatchingPolicies := make([]GeneralOverridePolicy, 0)
	for _, policy := range policies {
		if len(policy.GetOverrideSpec().ResourceSelectors) == 0 {
			resourceMatchingPolicies = append(resourceMatchingPolicies, policy)
			continue
		}

		if util.ResourceMatchSelectors(resource, policy.GetOverrideSpec().ResourceSelectors...) {
			resourceMatchingPolicies = append(resourceMatchingPolicies, policy)
		}
	}
	sort.SliceStable(resourceMatchingPolicies, func(i, j int) bool {
		implicitPriorityI := util.ResourceMatchSelectorsPriority(resource, resourceMatchingPolicies[i].GetOverrideSpec().ResourceSelectors...)
		if len(resourceMatchingPolicies[i].GetOverrideSpec().ResourceSelectors) == 0 {
			implicitPriorityI = util.PriorityMatchAll
		}
		implicitPriorityJ := util.ResourceMatchSelectorsPriority(resource, resourceMatchingPolicies[j].GetOverrideSpec().ResourceSelectors...)
		if len(resourceMatchingPolicies[j].GetOverrideSpec().ResourceSelectors) == 0 {
			implicitPriorityJ = util.PriorityMatchAll
		}
		if implicitPriorityI != implicitPriorityJ {
			return implicitPriorityI < implicitPriorityJ
		}
		return resourceMatchingPolicies[i].GetName() < resourceMatchingPolicies[j].GetName()
	})
	clusterMatchingPolicyOverriders := make([]policyOverriders, 0)
	for _, policy := range resourceMatchingPolicies {
		overrideRules := policy.GetOverrideSpec().OverrideRules
		// Since the tuple of '.spec.TargetCluster' and '.spec.Overriders' can not co-exist with '.spec.OverrideRules'
		// (guaranteed by webhook), so we only look '.spec.OverrideRules' here.
		if len(overrideRules) == 0 {
			overrideRules = []policyv1alpha1.RuleWithCluster{
				{
					//nolint:staticcheck
					// disable `deprecation` check for backward compatibility.
					TargetCluster: policy.GetOverrideSpec().TargetCluster,
					//nolint:staticcheck
					// disable `deprecation` check for backward compatibility.
					Overriders: policy.GetOverrideSpec().Overriders,
				},
			}
		}
		for _, rule := range overrideRules {
			if rule.TargetCluster == nil || (rule.TargetCluster != nil && util.ClusterMatches(cluster, *rule.TargetCluster)) {
				clusterMatchingPolicyOverriders = append(clusterMatchingPolicyOverriders, policyOverriders{
					name:       policy.GetName(),
					namespace:  policy.GetNamespace(),
					overriders: rule.Overriders,
				})
			}
		}
	}

	// select policy in which at least one PlaintextOverrider matches target resource.
	// TODO(RainbowMango): check if the overrider instructions can be applied to target resource.

	return clusterMatchingPolicyOverriders
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

// applyRawJSONPatch applies the override on to the given raw json object.
func applyRawJSONPatch(raw []byte, overrides []overrideOption) ([]byte, error) {
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return nil, err
	}

	return patch.Apply(raw)
}

func applyRawYAMLPatch(raw []byte, overrides []overrideOption) ([]byte, error) {
	rawJSON, err := yaml.YAMLToJSON(raw)
	if err != nil {
		klog.ErrorS(err, "Failed to convert yaml to json")
		return nil, err
	}

	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return nil, err
	}

	rawJSON, err = patch.Apply(rawJSON)
	if err != nil {
		return nil, err
	}

	rawYAML, err := yaml.JSONToYAML(rawJSON)
	if err != nil {
		klog.Errorf("Failed to convert json to yaml, error: %v", err)
		return nil, err
	}

	return rawYAML, nil
}

// applyPolicyOverriders applies OverridePolicy/ClusterOverridePolicy overriders to target object
func applyPolicyOverriders(rawObj *unstructured.Unstructured, overriders policyv1alpha1.Overriders) error {
	err := applyImageOverriders(rawObj, overriders.ImageOverrider)
	if err != nil {
		return err
	}
	// patch command
	if err := applyCommandOverriders(rawObj, overriders.CommandOverrider); err != nil {
		return err
	}
	// patch args
	if err := applyArgsOverriders(rawObj, overriders.ArgsOverrider); err != nil {
		return err
	}
	if err := applyLabelsOverriders(rawObj, overriders.LabelsOverrider); err != nil {
		return err
	}
	if err := applyAnnotationsOverriders(rawObj, overriders.AnnotationsOverrider); err != nil {
		return err
	}
	if err := applyFieldOverriders(rawObj, overriders.FieldOverrider); err != nil {
		return err
	}
	return applyJSONPatch(rawObj, parseJSONPatchesByPlaintext(overriders.Plaintext))
}

func applyImageOverriders(rawObj *unstructured.Unstructured, imageOverriders []policyv1alpha1.ImageOverrider) error {
	for index := range imageOverriders {
		patches, err := buildPatches(rawObj, &imageOverriders[index])
		if err != nil {
			klog.Errorf("Build patches with imageOverrides err: %v", err)
			return err
		}

		klog.V(4).Infof("Parsed JSON patches by imageOverriders(%+v): %+v", imageOverriders[index], patches)
		if err = applyJSONPatch(rawObj, patches); err != nil {
			return err
		}
	}

	return nil
}

func applyCommandOverriders(rawObj *unstructured.Unstructured, commandOverriders []policyv1alpha1.CommandArgsOverrider) error {
	for index := range commandOverriders {
		patches, err := buildCommandArgsPatches(CommandString, rawObj, &commandOverriders[index])
		if err != nil {
			return err
		}

		klog.V(4).Infof("Parsed JSON patches by commandOverriders(%+v): %+v", commandOverriders[index], patches)
		if err = applyJSONPatch(rawObj, patches); err != nil {
			return err
		}
	}

	return nil
}

func applyArgsOverriders(rawObj *unstructured.Unstructured, argsOverriders []policyv1alpha1.CommandArgsOverrider) error {
	for index := range argsOverriders {
		patches, err := buildCommandArgsPatches(ArgsString, rawObj, &argsOverriders[index])
		if err != nil {
			return err
		}

		klog.V(4).Infof("Parsed JSON patches by argsOverriders(%+v): %+v", argsOverriders[index], patches)
		if err = applyJSONPatch(rawObj, patches); err != nil {
			return err
		}
	}

	return nil
}

func applyFieldOverriders(rawObj *unstructured.Unstructured, FieldOverriders []policyv1alpha1.FieldOverrider) error {
	if len(FieldOverriders) == 0 {
		return nil
	}
	for index := range FieldOverriders {
		pointer, err := jsonpointer.New(FieldOverriders[index].FieldPath)
		if err != nil {
			klog.Errorf("Build jsonpointer with overrider's path err: %v", err)
			return err
		}
		res, kind, err := pointer.Get(rawObj.Object)
		if err != nil {
			klog.Errorf("Get value by overrider's path err: %v", err)
			return err
		}
		if kind != reflect.String {
			errMsg := fmt.Sprintf("Get object's value by overrider's path(%s) is not string", FieldOverriders[index].FieldPath)
			klog.Error(errMsg)
			return errors.New(errMsg)
		}
		dataBytes := []byte(res.(string))
		klog.V(4).Infof("Parsed JSON patches by FieldOverriders[%d](%+v)", index, FieldOverriders[index])
		var appliedRawData []byte
		if len(FieldOverriders[index].YAML) > 0 {
			appliedRawData, err = applyRawYAMLPatch(dataBytes, parseYAMLPatchesByField(FieldOverriders[index].YAML))
			if err != nil {
				klog.Errorf("Error applying raw JSON patch: %v", err)
				return err
			}
		} else if len(FieldOverriders[index].JSON) > 0 {
			appliedRawData, err = applyRawJSONPatch(dataBytes, parseJSONPatchesByField(FieldOverriders[index].JSON))
			if err != nil {
				klog.Errorf("Error applying raw YAML patch: %v", err)
				return err
			}
		}
		_, err = pointer.Set(rawObj.Object, string(appliedRawData))
		if err != nil {
			return err
		}
	}
	return nil
}

func parseJSONPatchesByPlaintext(overriders []policyv1alpha1.PlaintextOverrider) []overrideOption {
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

func parseYAMLPatchesByField(overriders []policyv1alpha1.YAMLPatchOperation) []overrideOption {
	patches := make([]overrideOption, 0, len(overriders))
	for i := range overriders {
		patches = append(patches, overrideOption{
			Op:    string(overriders[i].Operator),
			Path:  overriders[i].SubPath,
			Value: overriders[i].Value,
		})
	}
	return patches
}

func parseJSONPatchesByField(overriders []policyv1alpha1.JSONPatchOperation) []overrideOption {
	patches := make([]overrideOption, 0, len(overriders))
	for i := range overriders {
		patches = append(patches, overrideOption{
			Op:    string(overriders[i].Operator),
			Path:  overriders[i].SubPath,
			Value: overriders[i].Value,
		})
	}
	return patches
}
