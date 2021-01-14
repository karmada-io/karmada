package overridemanager

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	propagationstrategy "github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// OverrideManager managers override policies operation
type OverrideManager interface {
	ApplyOverridePolicies(rawObj *unstructured.Unstructured, cluster string) error
}

type overrideOptions struct {
	Op    string      `json:"op,omitempty"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type overridePolicyMap map[string][]overrideOptions

type overrideManagerImpl struct {
	client.Client
}

// NewOverrideManager returns a instance of OverrideManager
func NewOverrideManager(client client.Client) OverrideManager {
	return &overrideManagerImpl{
		Client: client,
	}
}

func (o *overrideManagerImpl) ApplyOverridePolicies(rawObj *unstructured.Unstructured, cluster string) error {
	overridePolices, err := o.findOverridePoliciesThatFit(rawObj, cluster)
	if err != nil {
		klog.Errorf("Failed to find override policies for object %v/%v", rawObj.GetNamespace(), rawObj.GetName())
		return err
	}

	if overridePolices == nil {
		return nil
	}

	// sort matched override policies
	sortedOverridePolices := make([]string, len(overridePolices))
	for overridePolicy := range overridePolices {
		sortedOverridePolices = append(sortedOverridePolices, overridePolicy)
	}
	sort.Strings(sortedOverridePolices)

	var appliedOverride string
	for _, overridePolicy := range sortedOverridePolices {
		applied, err := o.applyOverrideOptions(rawObj, overridePolices[overridePolicy], overridePolicy)
		if err != nil {
			return err
		}
		if applied {
			appliedOverride += fmt.Sprintf("%s, ", overridePolicy)
		}
	}

	if appliedOverride != "" {
		util.MergeAnnotation(rawObj, util.OverrideClaimKey, strings.TrimSuffix(appliedOverride, ", "))
	}

	return nil
}

func (o *overrideManagerImpl) findOverridePoliciesThatFit(rawObj *unstructured.Unstructured, cluster string) (overridePolicyMap, error) {
	overridePolicyList := &propagationstrategy.OverridePolicyList{}
	err := o.Client.List(context.TODO(), overridePolicyList, &client.ListOptions{Namespace: rawObj.GetNamespace()})
	if err != nil {
		klog.Errorf("Failed to list override policies in namespace %s", rawObj.GetNamespace())
		return nil, err
	}

	if len(overridePolicyList.Items) == 0 {
		return nil, nil
	}

	matchedOverridePolicies := make(overridePolicyMap)

	for _, overridePolicy := range overridePolicyList.Items {
		if o.matchOverridePolicy(rawObj, overridePolicy, cluster) {
			for _, rawOverride := range overridePolicy.Spec.Overriders.Plaintext {
				// validate override path
				if !o.validatePath(rawObj, rawOverride, overridePolicy.Name) {
					klog.Infof("Drop override operation %v of override policy %s/%s because it contains invalid path", rawOverride, overridePolicy.Namespace, overridePolicy.Name)
					continue
				}
				targetOverrideItem := overrideOptions{Path: rawOverride.Path, Op: string(rawOverride.Operator), Value: rawOverride.Value}
				matchedOverridePolicies[overridePolicy.Name] = append(matchedOverridePolicies[overridePolicy.Name], targetOverrideItem)
			}
		}
	}

	return matchedOverridePolicies, nil
}

func (o *overrideManagerImpl) matchOverridePolicy(rawObj *unstructured.Unstructured, overridePolicy propagationstrategy.OverridePolicy, cluster string) bool {
	if !o.matchResourceSelector(rawObj, overridePolicy) {
		return false
	}

	if !o.matchClusterSelector(cluster, overridePolicy) {
		return false
	}

	return true
}

func (o *overrideManagerImpl) matchResourceSelector(rawObj *unstructured.Unstructured, overridePolicy propagationstrategy.OverridePolicy) bool {
	for _, resourceSelector := range overridePolicy.Spec.ResourceSelectors {
		if resourceSelector.Namespace != "" && resourceSelector.Namespace != rawObj.GetNamespace() {
			continue
		}
		if resourceSelector.Name != "" && resourceSelector.Name != rawObj.GetName() {
			continue
		}
		if resourceSelector.APIVersion != rawObj.GetAPIVersion() || resourceSelector.Kind != rawObj.GetKind() {
			continue
		}

		if resourceSelector.LabelSelector != nil {
			labelSelector, err := metav1.LabelSelectorAsSelector(resourceSelector.LabelSelector)
			if err != nil {
				return false
			}
			if !labelSelector.Matches(labels.Set(rawObj.GetLabels())) {
				continue
			}
		}
		return true
	}
	return false
}

func (o *overrideManagerImpl) matchClusterSelector(cluster string, overridePolicy propagationstrategy.OverridePolicy) bool {
	// todo: support LabelSelector and FieldSelector
	for _, clusterName := range overridePolicy.Spec.TargetCluster.ExcludeClusters {
		if clusterName == cluster {
			return false
		}
	}

	for _, clusterName := range overridePolicy.Spec.TargetCluster.ClusterNames {
		if clusterName == cluster {
			return true
		}
	}
	return false
}

func (o *overrideManagerImpl) validatePath(rawObj *unstructured.Unstructured, override propagationstrategy.PlaintextOverrider, overridePolicyName string) bool {
	if !strings.HasPrefix(override.Path, "/") {
		klog.Errorf("Invalid path %s/%s/%s, path must start with a leading / and entries must be separated by /", rawObj.GetNamespace(), overridePolicyName, override.Path)
		return false
	}
	// todo: validate path exist
	return true
}

func (o *overrideManagerImpl) applyOverrideOptions(rawObj *unstructured.Unstructured, overrideItems []overrideOptions, overridePoliceName string) (bool, error) {
	applied := false
	for _, overrideOption := range overrideItems {
		originJSONBytes, err := rawObj.MarshalJSON()
		if err != nil {
			return false, err
		}

		patchJSONBytes, err := json.Marshal([]overrideOptions{overrideOption})
		if err != nil {
			klog.Errorf("Failed to marshal override option %s/%s/%v: %v", rawObj.GetNamespace(), overridePoliceName, overrideOption, err)
			continue
		}

		newJSONBytes, err := o.applyJSONPatch(originJSONBytes, patchJSONBytes)
		if err != nil {
			klog.Errorf("Failed to apply json patch %s/%s/%v: %v", rawObj.GetNamespace(), overridePoliceName, overrideOption, err)
			continue
		}

		err = rawObj.UnmarshalJSON(newJSONBytes)
		if err != nil {
			klog.Errorf("Failed to unmarshal raw object %s/%s: %v", rawObj.GetNamespace(), rawObj.GetName(), err)
			continue
		}
		applied = true
	}

	if applied {
		return true, nil
	}
	return false, nil
}

func (o *overrideManagerImpl) applyJSONPatch(original, patchJSON []byte) ([]byte, error) {
	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		return nil, err
	}

	return patch.Apply(original)
}
