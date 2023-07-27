package detector

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// preemptionEnabled checks if preemption is enabled.
func preemptionEnabled(preemption policyv1alpha1.PreemptionBehavior) bool {
	if preemption != policyv1alpha1.PreemptAlways {
		return false
	}
	if !features.FeatureGate.Enabled(features.PolicyPreemption) {
		klog.Warningf("Cannot handle the preemption process because feature gate %q is not enabled.", features.PolicyPreemption)
		return false
	}
	return true
}

// handleClusterPropagationPolicyPreemption handles the preemption process of PropagationPolicy.
// The preemption rule: high-priority PP > low-priority PP > CPP.
func (d *ResourceDetector) handlePropagationPolicyPreemption(policy *policyv1alpha1.PropagationPolicy) error {
	var errs []error
	for _, rs := range policy.Spec.ResourceSelectors {
		resourceTemplate, err := d.fetchResourceTemplate(rs)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if resourceTemplate == nil {
			continue
		}

		errs = append(errs, d.preemptPropagationPolicy(resourceTemplate, policy))
		// TODO(whitewindmills): PP preempts CPP.
	}

	return utilerrors.NewAggregate(errs)
}

// handleClusterPropagationPolicyPreemption handles the preemption process of ClusterPropagationPolicy.
// The preemption rule: high-priority CPP > low-priority CPP.
func (d *ResourceDetector) handleClusterPropagationPolicyPreemption(policy *policyv1alpha1.ClusterPropagationPolicy) error {
	var errs []error
	for _, rs := range policy.Spec.ResourceSelectors {
		resourceTemplate, err := d.fetchResourceTemplate(rs)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if resourceTemplate == nil {
			continue
		}

		errs = append(errs, d.preemptClusterPropagationPolicy(resourceTemplate, policy))
	}

	return utilerrors.NewAggregate(errs)
}

// preemptPropagationPolicy preempts resource template that is claimed by PropagationPolicy.
func (d *ResourceDetector) preemptPropagationPolicy(resourceTemplate *unstructured.Unstructured, policy *policyv1alpha1.PropagationPolicy) error {
	rtLabels := resourceTemplate.GetLabels()
	claimedPolicyNamespace := util.GetLabelValue(rtLabels, policyv1alpha1.PropagationPolicyNamespaceLabel)
	claimedPolicyName := util.GetLabelValue(rtLabels, policyv1alpha1.PropagationPolicyNameLabel)
	if claimedPolicyName == "" || claimedPolicyNamespace == "" {
		return nil
	}
	// resource template has been claimed by policy itself.
	if claimedPolicyNamespace == policy.Namespace && claimedPolicyName == policy.Name {
		return nil
	}

	claimedPolicyObj, err := d.propagationPolicyLister.ByNamespace(claimedPolicyNamespace).Get(claimedPolicyName)
	if err != nil {
		klog.Errorf("Failed to retrieve claimed propagation policy(%s/%s): %v.", claimedPolicyNamespace, claimedPolicyName, err)
		return err
	}

	claimedPolicy := &policyv1alpha1.PropagationPolicy{}
	if err = helper.ConvertToTypedObject(claimedPolicyObj, claimedPolicy); err != nil {
		klog.Errorf("Failed to convert PropagationPolicy from unstructured object: %v.", err)
		return err
	}

	if policy.ExplicitPriority() <= claimedPolicy.ExplicitPriority() {
		klog.V(2).Infof("Propagation policy(%s/%s) cannot preempt another propagation policy(%s/%s) due to insufficient priority.",
			policy.Namespace, policy.Name, claimedPolicyNamespace, claimedPolicyName)
		return nil
	}

	clusterWideKey, err := keys.ClusterWideKeyFunc(resourceTemplate)
	if err != nil {
		// should not happen.
		return err
	}
	if err := d.ApplyPolicy(resourceTemplate, clusterWideKey, policy); err != nil {
		klog.Errorf("Failed to apply new propagation policy(%s/%s) on resource template(%s, kind=%s, %s): %v.", policy.Namespace, policy.Name,
			resourceTemplate.GetAPIVersion(), resourceTemplate.GetKind(), names.NamespacedKey(resourceTemplate.GetNamespace(), resourceTemplate.GetName()), err)
		return err
	}
	klog.V(4).Infof("Propagation policy(%s/%s) has preempted another propagation policy(%s/%s).",
		policy.Namespace, policy.Name, claimedPolicyNamespace, claimedPolicyName)
	return nil
}

// preemptClusterPropagationPolicy preempts resource template that is claimed by ClusterPropagationPolicy.
func (d *ResourceDetector) preemptClusterPropagationPolicy(resourceTemplate *unstructured.Unstructured, policy *policyv1alpha1.ClusterPropagationPolicy) error {
	claimedPolicyName := util.GetLabelValue(resourceTemplate.GetLabels(), policyv1alpha1.ClusterPropagationPolicyLabel)
	if claimedPolicyName == "" {
		return nil
	}
	// resource template has been claimed by policy itself.
	if claimedPolicyName == policy.Name {
		return nil
	}

	claimedPolicyObj, err := d.clusterPropagationPolicyLister.Get(claimedPolicyName)
	if err != nil {
		klog.Errorf("Failed to retrieve claimed cluster propagation policy(%s): %v.", claimedPolicyName, err)
		return err
	}

	claimedPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
	if err = helper.ConvertToTypedObject(claimedPolicyObj, claimedPolicy); err != nil {
		klog.Errorf("Failed to convert ClusterPropagationPolicy from unstructured object: %v.", err)
		return err
	}

	if policy.ExplicitPriority() <= claimedPolicy.ExplicitPriority() {
		klog.V(2).Infof("Cluster propagation policy(%s) cannot preempt another cluster propagation policy(%s) due to insufficient priority.",
			policy.Name, claimedPolicyName)
		return nil
	}

	clusterWideKey, err := keys.ClusterWideKeyFunc(resourceTemplate)
	if err != nil {
		// should not happen.
		return err
	}
	if err := d.ApplyClusterPolicy(resourceTemplate, clusterWideKey, policy); err != nil {
		klog.Errorf("Failed to apply new cluster propagation policy(%s) on resource template(%s, kind=%s, %s): %v.", policy.Name,
			resourceTemplate.GetAPIVersion(), resourceTemplate.GetKind(), names.NamespacedKey(resourceTemplate.GetNamespace(), resourceTemplate.GetName()), err)
		return err
	}
	klog.V(4).Infof("Cluster propagation policy(%s) has preempted another cluster propagation policy(%s).", policy.Name, claimedPolicyName)
	return nil
}

// fetchResourceTemplate fetches resource template by resource selector, ignore it if not found or deleting.
func (d *ResourceDetector) fetchResourceTemplate(rs policyv1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	resourceTemplate, err := helper.FetchResourceTemplate(d.DynamicClient, d.InformerManager, d.RESTMapper, helper.ConstructObjectReference(rs))
	if err != nil {
		// do nothing if resource template not exist, it might has been removed.
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("Resource template(%s, kind=%s, %s) cannot be preempted because it has been deleted.",
				rs.APIVersion, rs.Kind, names.NamespacedKey(rs.Namespace, rs.Name))
			return nil, nil
		}
		klog.Errorf("Failed to fetch resource template(%s, kind=%s, %s): %v.", rs.APIVersion, rs.Kind,
			names.NamespacedKey(rs.Namespace, rs.Name), err)
		return nil, err
	}

	if !resourceTemplate.GetDeletionTimestamp().IsZero() {
		klog.V(2).Infof("Resource template(%s, kind=%s, %s) cannot be preempted because it's being deleted.",
			rs.APIVersion, rs.Kind, names.NamespacedKey(rs.Namespace, rs.Name))
		return nil, nil
	}
	return resourceTemplate, nil
}
