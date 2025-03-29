/*
Copyright 2023 The Karmada Authors.

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

package detector

import (
	"context"

	pq "github.com/emirpasic/gods/queues/priorityqueue"
	godsutils "github.com/emirpasic/gods/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// PriorityKey is the unique propagation policy key with priority.
type PriorityKey struct {
	runtime.Object
	// Priority is the priority of the propagation policy.
	Priority int32
}

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

// handlePropagationPolicyPreemption  handles the preemption process of PropagationPolicy.
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
	rtAnnotations := resourceTemplate.GetAnnotations()
	claimedPolicyNamespace := util.GetAnnotationValue(rtAnnotations, policyv1alpha1.PropagationPolicyNamespaceAnnotation)
	claimedPolicyName := util.GetAnnotationValue(rtAnnotations, policyv1alpha1.PropagationPolicyNameAnnotation)
	if claimedPolicyName == "" || claimedPolicyNamespace == "" {
		return nil
	}
	// resource template has been claimed by policy itself.
	if claimedPolicyNamespace == policy.Namespace && claimedPolicyName == policy.Name {
		return nil
	}

	claimedPolicy := &policyv1alpha1.PropagationPolicy{}
	err = d.Client.Get(context.TODO(), client.ObjectKey{Namespace: claimedPolicyNamespace, Name: claimedPolicyName}, claimedPolicy)
	if err != nil {
		klog.Errorf("Failed to retrieve claimed propagation policy(%s/%s): %v.", claimedPolicyNamespace, claimedPolicyName, err)
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

	if _, err = d.ClaimPolicyForObject(resourceTemplate, policy); err != nil {
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
	claimedPolicyName := util.GetAnnotationValue(resourceTemplate.GetAnnotations(), policyv1alpha1.ClusterPropagationPolicyAnnotation)
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

	if _, err = d.ClaimPolicyForObject(resourceTemplate, policy); err != nil {
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
	claimedPolicyName := util.GetAnnotationValue(resourceTemplate.GetAnnotations(), policyv1alpha1.ClusterPropagationPolicyAnnotation)
	if claimedPolicyName == "" {
		return nil
	}
	// resource template has been claimed by policy itself.
	if claimedPolicyName == policy.Name {
		return nil
	}

	claimedPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
	err = d.Client.Get(context.TODO(), client.ObjectKey{Name: claimedPolicyName}, claimedPolicy)
	if err != nil {
		klog.Errorf("Failed to retrieve claimed cluster propagation policy(%s): %v.", claimedPolicyName, err)
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

	if _, err = d.ClaimClusterPolicyForObject(resourceTemplate, policy); err != nil {
		klog.Errorf("Failed to claim new cluster propagation policy(%s) on resource template(%s, kind=%s, %s): %v.", policy.Name,
			resourceTemplate.GetAPIVersion(), resourceTemplate.GetKind(), names.NamespacedKey(resourceTemplate.GetNamespace(), resourceTemplate.GetName()), err)
		return err
	}
	klog.V(4).Infof("Cluster propagation policy(%s) has preempted another cluster propagation policy(%s).", policy.Name, claimedPolicyName)
	return nil
}

// fetchResourceTemplate fetches resource template by resource selector, ignore it if not found or deleting.
func (d *ResourceDetector) fetchResourceTemplate(rs policyv1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	resourceTemplate, err := helper.FetchResourceTemplate(context.TODO(), d.DynamicClient, d.InformerManager, d.RESTMapper, helper.ConstructObjectReference(rs))
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
	policyList := &policyv1alpha1.PropagationPolicyList{}
	err := d.Client.List(context.TODO(), policyList, &client.ListOptions{
		Namespace:             newPolicy.GetNamespace(),
		UnsafeDisableDeepCopy: ptr.To(true),
	})
	if err != nil {
		klog.Errorf("Failed to list PropagationPolicy from namespace: %s, error: %v", newPolicy.GetNamespace(), err)
		return
	}
	if len(policyList.Items) == 0 {
		klog.Infof("No PropagationPolicy to preempt the PropagationPolicy(%s/%s).", newPolicy.GetNamespace(), newPolicy.GetName())
	}

	// Use the priority queue to sort the listed policies to ensure the
	// higher priority PropagationPolicy be process first to avoid possible
	// multiple preemption.
	sortedPotentialKeys := pq.NewWith(priorityDescendingComparator)
	for _, potentialPolicy := range policyList.Items {
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
			klog.Infof("Enqueuing PropagationPolicy(%s/%s) in case of PropagationPolicy(%s/%s) priority changes.", potentialPolicy.GetNamespace(), potentialPolicy.GetName(), newPolicy.GetNamespace(), newPolicy.GetName())
			sortedPotentialKeys.Enqueue(&PriorityKey{
				Object:   &potentialPolicy,
				Priority: potentialPolicy.ExplicitPriority(),
			})
		}
	}
	requeuePotentialKeys(sortedPotentialKeys, d.policyReconcileWorker)
}

// HandleDeprioritizedClusterPropagationPolicy responses to priority change of a ClusterPropagationPolicy,
// if the change is from high priority (e.g. 5) to low priority(e.g. 3), it will
// check if there is another ClusterPropagationPolicy could preempt the targeted resource,
// and put the ClusterPropagationPolicy in the queue to trigger preemption.
func (d *ResourceDetector) HandleDeprioritizedClusterPropagationPolicy(oldPolicy policyv1alpha1.ClusterPropagationPolicy, newPolicy policyv1alpha1.ClusterPropagationPolicy) {
	klog.Infof("ClusterPropagationPolicy(%s) priority changed from %d to %d",
		newPolicy.GetName(), *oldPolicy.Spec.Priority, *newPolicy.Spec.Priority)
	policyList := &policyv1alpha1.ClusterPropagationPolicyList{}
	err := d.Client.List(context.TODO(), policyList, &client.ListOptions{
		UnsafeDisableDeepCopy: ptr.To(true),
	})
	if err != nil {
		klog.Errorf("Failed to list ClusterPropagationPolicy, error: %v", err)
		return
	}
	if len(policyList.Items) == 0 {
		klog.Infof("No ClusterPropagationPolicy to preempt the ClusterPropagationPolicy(%s).", newPolicy.GetName())
	}

	// Use the priority queue to sort the listed policies to ensure the
	// higher priority ClusterPropagationPolicy be process first to avoid possible
	// multiple preemption.
	sortedPotentialKeys := pq.NewWith(priorityDescendingComparator)
	for _, potentialPolicy := range policyList.Items {
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
			klog.Infof("Enqueuing ClusterPropagationPolicy(%s) in case of ClusterPropagationPolicy(%s) priority changes.",
				potentialPolicy.GetName(), newPolicy.GetName())
			sortedPotentialKeys.Enqueue(&PriorityKey{
				Object:   &potentialPolicy,
				Priority: potentialPolicy.ExplicitPriority(),
			})
		}
	}
	requeuePotentialKeys(sortedPotentialKeys, d.clusterPolicyReconcileWorker)
}

// requeuePotentialKeys re-queues potential policy keys.
func requeuePotentialKeys(sortedPotentialKeys *pq.Queue, worker util.AsyncWorker) {
	for {
		key, ok := sortedPotentialKeys.Dequeue()
		if !ok {
			break
		}

		worker.Enqueue(key.(*PriorityKey).Object)
	}
}

// priorityDescendingComparator provides a basic descending comparison on policy priority.
func priorityDescendingComparator(a, b interface{}) int {
	aPriority := a.(*PriorityKey).Priority
	bPriority := b.(*PriorityKey).Priority
	return godsutils.Int32Comparator(bPriority, aPriority)
}
