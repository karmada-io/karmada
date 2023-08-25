package detector

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/util"
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

		if err := d.preemptPropagationPolicy(resourceTemplate, policy); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := d.preemptClusterPropagationPolicyDirectly(resourceTemplate, policy); err != nil {
			errs = append(errs, err)
		}
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

		if err := d.preemptClusterPropagationPolicy(resourceTemplate, policy); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

// preemptPropagationPolicy preempts resource template that is claimed by PropagationPolicy.
func (d *ResourceDetector) preemptPropagationPolicy(resourceTemplate *unstructured.Unstructured, policy *policyv1alpha1.PropagationPolicy) (err error) {
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

	defer func() {
		metrics.CountPolicyPreemption(err)
		if err != nil {
			d.EventRecorder.Eventf(resourceTemplate, corev1.EventTypeWarning, events.EventReasonPreemptPolicyFailed,
				"Propagation policy(%s/%s) failed to preempt propagation policy(%s/%s): %v", policy.Namespace, policy.Name, claimedPolicyNamespace, claimedPolicyName, err)
			return
		}
		d.EventRecorder.Eventf(resourceTemplate, corev1.EventTypeNormal, events.EventReasonPreemptPolicySucceed,
			"Propagation policy(%s/%s) preempted propagation policy(%s/%s) successfully", policy.Namespace, policy.Name, claimedPolicyNamespace, claimedPolicyName)
	}()

	if err = d.ClaimPolicyForObject(resourceTemplate, policy.Namespace, policy.Name, string(policy.UID)); err != nil {
		klog.Errorf("Failed to claim new propagation policy(%s/%s) on resource template(%s, kind=%s, %s): %v.", policy.Namespace, policy.Name,
			resourceTemplate.GetAPIVersion(), resourceTemplate.GetKind(), names.NamespacedKey(resourceTemplate.GetNamespace(), resourceTemplate.GetName()), err)
		return err
	}
	klog.V(4).Infof("Propagation policy(%s/%s) has preempted another propagation policy(%s/%s).",
		policy.Namespace, policy.Name, claimedPolicyNamespace, claimedPolicyName)
	return nil
}

// preemptClusterPropagationPolicyDirectly directly preempts resource template claimed by ClusterPropagationPolicy regardless of priority.
func (d *ResourceDetector) preemptClusterPropagationPolicyDirectly(resourceTemplate *unstructured.Unstructured, policy *policyv1alpha1.PropagationPolicy) (err error) {
	claimedPolicyName := util.GetLabelValue(resourceTemplate.GetLabels(), policyv1alpha1.ClusterPropagationPolicyLabel)
	if claimedPolicyName == "" {
		return nil
	}

	defer func() {
		metrics.CountPolicyPreemption(err)
		if err != nil {
			d.EventRecorder.Eventf(resourceTemplate, corev1.EventTypeWarning, events.EventReasonPreemptPolicyFailed,
				"Propagation policy(%s/%s) failed to preempt cluster propagation policy(%s): %v", policy.Namespace, policy.Name, claimedPolicyName, err)
			return
		}
		d.EventRecorder.Eventf(resourceTemplate, corev1.EventTypeNormal, events.EventReasonPreemptPolicySucceed,
			"Propagation policy(%s/%s) preempted cluster propagation policy(%s) successfully", policy.Namespace, policy.Name, claimedPolicyName)
	}()

	if err = d.ClaimPolicyForObject(resourceTemplate, policy.Namespace, policy.Name, string(policy.UID)); err != nil {
		klog.Errorf("Failed to claim new propagation policy(%s/%s) on resource template(%s, kind=%s, %s) directly: %v.", policy.Namespace, policy.Name,
			resourceTemplate.GetAPIVersion(), resourceTemplate.GetKind(), names.NamespacedKey(resourceTemplate.GetNamespace(), resourceTemplate.GetName()), err)
		return err
	}
	klog.V(4).Infof("Propagation policy(%s/%s) has preempted another cluster propagation policy(%s).",
		policy.Namespace, policy.Name, claimedPolicyName)
	return nil
}

// preemptClusterPropagationPolicy preempts resource template that is claimed by ClusterPropagationPolicy.
func (d *ResourceDetector) preemptClusterPropagationPolicy(resourceTemplate *unstructured.Unstructured, policy *policyv1alpha1.ClusterPropagationPolicy) (err error) {
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

	defer func() {
		metrics.CountPolicyPreemption(err)
		if err != nil {
			d.EventRecorder.Eventf(resourceTemplate, corev1.EventTypeWarning, events.EventReasonPreemptPolicyFailed,
				"Cluster propagation policy(%s) failed to preempt cluster propagation policy(%s): %v", policy.Name, claimedPolicyName, err)
			return
		}
		d.EventRecorder.Eventf(resourceTemplate, corev1.EventTypeNormal, events.EventReasonPreemptPolicySucceed,
			"Cluster propagation policy(%s) preempted cluster propagation policy(%s) successfully", policy.Name, claimedPolicyName)
	}()

	if err = d.ClaimClusterPolicyForObject(resourceTemplate, policy.Name, string(policy.UID)); err != nil {
		klog.Errorf("Failed to claim new cluster propagation policy(%s) on resource template(%s, kind=%s, %s): %v.", policy.Name,
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

// HandleDeprioritizedPropagationPolicy responses to priority change of a PropagationPolicy,
// if the change is from high priority (e.g. 5) to low priority(e.g. 3), it will
// check if there is another PropagationPolicy could preempt the targeted resource,
// and put the PropagationPolicy in the queue to trigger preemption.
func (d *ResourceDetector) HandleDeprioritizedPropagationPolicy(oldPolicy policyv1alpha1.PropagationPolicy, newPolicy policyv1alpha1.PropagationPolicy) {
	klog.Infof("PropagationPolicy(%s/%s) priority changed from %d to %d", newPolicy.GetNamespace(), newPolicy.GetName(), *oldPolicy.Spec.Priority, *newPolicy.Spec.Priority)
	policies, err := d.propagationPolicyLister.ByNamespace(newPolicy.GetNamespace()).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list PropagationPolicy from namespace: %s, error: %v", newPolicy.GetNamespace(), err)
		return
	}

	// TODO(@RainbowMango): Should sort the listed policies to ensure the
	// higher priority PropagationPolicy be process first to avoid possible
	// multiple preemption.

	for i := range policies {
		var potentialPolicy policyv1alpha1.PropagationPolicy
		if err = helper.ConvertToTypedObject(policies[i], &potentialPolicy); err != nil {
			klog.Errorf("Failed to convert typed PropagationPolicy: %v", err)
			continue
		}
		// Re-queue the polies that enables preemption and with the priority
		// in range (new priority, old priority).
		// For the polices with higher priority than old priority, it can
		// perform preempt automatically and don't need to re-queue here.
		// For the polices with lower priority than new priority, it can't
		// perform preempt as insufficient priority.
		if potentialPolicy.Spec.Priority != nil &&
			potentialPolicy.Spec.Preemption == policyv1alpha1.PreemptAlways &&
			potentialPolicy.ExplicitPriority() > newPolicy.ExplicitPriority() &&
			potentialPolicy.ExplicitPriority() < oldPolicy.ExplicitPriority() {
			var potentialKey util.QueueKey
			potentialKey, err = ClusterWideKeyFunc(&potentialPolicy)
			if err != nil {
				return
			}
			klog.Infof("Enqueuing PropagationPolicy(%s/%s) in case of PropagationPolicy(%s/%s) priority changes", potentialPolicy.GetNamespace(), potentialPolicy.GetName(), newPolicy.GetNamespace(), newPolicy.GetName())
			d.policyReconcileWorker.Add(potentialKey)
		}
	}
}

// HandleDeprioritizedClusterPropagationPolicy responses to priority change of a ClusterPropagationPolicy,
// if the change is from high priority (e.g. 5) to low priority(e.g. 3), it will
// check if there is another ClusterPropagationPolicy could preempt the targeted resource,
// and put the ClusterPropagationPolicy in the queue to trigger preemption.
func (d *ResourceDetector) HandleDeprioritizedClusterPropagationPolicy(oldPolicy policyv1alpha1.ClusterPropagationPolicy, newPolicy policyv1alpha1.ClusterPropagationPolicy) {
	klog.Infof("ClusterPropagationPolicy(%s/%s) priority changed from %d to %d",
		newPolicy.GetNamespace(), newPolicy.GetName(), *oldPolicy.Spec.Priority, *newPolicy.Spec.Priority)

	policies, err := d.clusterPropagationPolicyLister.ByNamespace(newPolicy.GetNamespace()).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list ClusterPropagationPolicy from namespace: %s, error: %v", newPolicy.GetNamespace(), err)
		return
	}

	// TODO(@RainbowMango): Should sort the listed policies to ensure the
	// higher priority ClusterPropagationPolicy be process first to avoid possible
	// multiple preemption.

	for i := range policies {
		var potentialPolicy policyv1alpha1.ClusterPropagationPolicy
		if err = helper.ConvertToTypedObject(policies[i], &potentialPolicy); err != nil {
			klog.Errorf("Failed to convert typed ClusterPropagationPolicy: %v", err)
			continue
		}
		// Re-queue the polies that enables preemption and with the priority
		// in range (new priority, old priority).
		// For the polices with higher priority than old priority, it can
		// perform preempt automatically and don't need to re-queue here.
		// For the polices with lower priority than new priority, it can't
		// perform preempt as insufficient priority.
		if potentialPolicy.Spec.Priority != nil &&
			potentialPolicy.Spec.Preemption == policyv1alpha1.PreemptAlways &&
			potentialPolicy.ExplicitPriority() > newPolicy.ExplicitPriority() &&
			potentialPolicy.ExplicitPriority() < oldPolicy.ExplicitPriority() {
			var potentialKey util.QueueKey
			potentialKey, err = ClusterWideKeyFunc(&potentialPolicy)
			if err != nil {
				return
			}
			klog.Infof("Enqueuing ClusterPropagationPolicy(%s/%s) in case of ClusterPropagationPolicy(%s/%s) priority changes",
				potentialPolicy.GetNamespace(), potentialPolicy.GetName(), newPolicy.GetNamespace(), newPolicy.GetName())
			d.clusterPolicyReconcileWorker.Add(potentialKey)
		}
	}
}
