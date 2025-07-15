/*
Copyright 2025 The Karmada Authors.

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

package taint

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "cluster-taint-policy-controller"

// ClusterTaintPolicyController is to sync ClusterTaintPolicy.
type ClusterTaintPolicyController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ClusterTaintPolicyController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling Cluster", "cluster", req.Name)

	clusterObj := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(ctx, req.NamespacedName, clusterObj); err != nil {
		klog.ErrorS(err, "Failed to get Cluster", "cluster", req.Name)
		return controllerruntime.Result{}, client.IgnoreNotFound(err)
	}

	clusterTaintPolicyList := &policyv1alpha1.ClusterTaintPolicyList{}
	listOption := &client.ListOptions{UnsafeDisableDeepCopy: ptr.To(true)}
	if err := c.Client.List(ctx, clusterTaintPolicyList, listOption); err != nil {
		klog.ErrorS(err, "Failed to list ClusterTaintPolicy")
		return controllerruntime.Result{}, err
	}

	// Sorting ensures a deterministic processing order and ensures consistent taint generation for Clusters.
	// This prevents repeated taint addition/removal cycles when conflicting policies exist.
	policies := clusterTaintPolicyList.Items
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreationTimestamp.Before(&policies[j].CreationTimestamp)
	})

	clusterCopyObj := clusterObj.DeepCopy()
	for _, policy := range policies {
		if policy.Spec.TargetClusters != nil && !util.ClusterMatches(clusterCopyObj, *policy.Spec.TargetClusters) {
			continue
		}

		if conditionMatches(clusterCopyObj.Status.Conditions, policy.Spec.AddOnConditions) {
			clusterCopyObj.Spec.Taints = addTaintsOnCluster(clusterCopyObj, policy.Spec.Taints, metav1.Now())
		}

		if conditionMatches(clusterCopyObj.Status.Conditions, policy.Spec.RemoveOnConditions) {
			clusterCopyObj.Spec.Taints = removeTaintsFromCluster(clusterCopyObj, policy.Spec.Taints)
		}
	}

	if !reflect.DeepEqual(clusterObj.Spec.Taints, clusterCopyObj.Spec.Taints) {
		objPatch := client.MergeFrom(clusterObj)
		err := c.Client.Patch(ctx, clusterCopyObj, objPatch)
		if err != nil {
			klog.ErrorS(err, "Failed to patch Cluster", "cluster", req.Name)
			c.EventRecorder.Event(clusterCopyObj, corev1.EventTypeWarning, events.EventReasonTaintClusterFailed, err.Error())
			return controllerruntime.Result{}, err
		}

		msg := fmt.Sprintf("Update Cluster(%s) with taints(%v) succeed", req.Name,
			utilhelper.GenerateTaintsMessage(clusterCopyObj.Spec.Taints))
		klog.InfoS("Successfully updated cluster taints", "cluster", req.Name, "taints", clusterCopyObj.Spec.Taints)
		c.EventRecorder.Event(clusterCopyObj, corev1.EventTypeNormal, events.EventReasonTaintClusterSucceed, msg)
		return controllerruntime.Result{}, nil
	}
	klog.V(4).InfoS("Cluster taints are up to date.", "cluster", req.Name)
	return controllerruntime.Result{}, nil
}

// conditionMatches returns true if the conditions matches the matchConditions.
// The match conditions are ANDed.
// If the match conditions are empty, it will return false.
func conditionMatches(conditions []metav1.Condition, matchConditions []policyv1alpha1.MatchCondition) bool {
	if len(matchConditions) == 0 {
		return false
	}

	matches := true
	for _, matchCondition := range matchConditions {
		match := false
		for _, clusterCondition := range conditions {
			if clusterCondition.Type != matchCondition.ConditionType {
				continue
			}

			switch matchCondition.Operator {
			case policyv1alpha1.MatchConditionOpIn:
				for _, value := range matchCondition.StatusValues {
					if clusterCondition.Status == value {
						match = true
						break
					}
				}
			case policyv1alpha1.MatchConditionOpNotIn:
				match = true
				for _, value := range matchCondition.StatusValues {
					if clusterCondition.Status == value {
						match = false
						break
					}
				}
			default:
				err := fmt.Errorf("unsupported MatchCondition operator")
				klog.ErrorS(err, "operator", matchCondition.Operator)
				return false
			}
		}

		if !match {
			matches = false
			break
		}
	}
	return matches
}

// addTaintsOnCluster adds taints on the cluster.
// If the taint already exists, it will not be added.
// Use now to set the TimeAdded field of the taint.
// It's convenient for testing.
func addTaintsOnCluster(cluster *clusterv1alpha1.Cluster, taints []policyv1alpha1.Taint, now metav1.Time) []corev1.Taint {
	clusterTaints := cluster.Spec.Taints
	for _, taint := range taints {
		exist := false
		for _, clusterTaint := range clusterTaints {
			if clusterTaint.Effect == taint.Effect &&
				clusterTaint.Key == taint.Key {
				exist = true
				break
			}
		}

		if !exist {
			clusterTaints = append(clusterTaints, corev1.Taint{
				Key:       taint.Key,
				Value:     taint.Value,
				Effect:    taint.Effect,
				TimeAdded: &now,
			})
		}
	}

	return clusterTaints
}

// removeTaintsFromCluster removes taints from the cluster.
// If the taint does not exist, it will not be removed.
func removeTaintsFromCluster(cluster *clusterv1alpha1.Cluster, taints []policyv1alpha1.Taint) []corev1.Taint {
	clusterTaints := cluster.Spec.Taints
	for _, taint := range taints {
		for index, clusterTaint := range clusterTaints {
			if clusterTaint.Effect == taint.Effect &&
				clusterTaint.Key == taint.Key {
				clusterTaints = append(clusterTaints[:index], clusterTaints[index+1:]...)
				break
			}
		}
	}
	return clusterTaints
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ClusterTaintPolicyController) SetupWithManager(mgr controllerruntime.Manager) error {
	clusterStatusConditionPredicateFn := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			// After controller reboot, it will reconcile all clusters.
			return true
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldCluster := event.ObjectOld.(*clusterv1alpha1.Cluster)
			newCluster := event.ObjectNew.(*clusterv1alpha1.Cluster)
			return !reflect.DeepEqual(oldCluster.Status.Conditions, newCluster.Status.Conditions)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool { return false },
	}

	clusterTaintPolicyPredicateFn := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldPolicy := event.ObjectOld.(*policyv1alpha1.ClusterTaintPolicy)
			newPolicy := event.ObjectNew.(*policyv1alpha1.ClusterTaintPolicy)
			return !reflect.DeepEqual(oldPolicy.Spec, newPolicy.Spec)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool { return false },
	}

	clusterTaintPolicyMapFunc := handler.MapFunc(
		func(ctx context.Context, policyObj client.Object) []reconcile.Request {
			clusterList := &clusterv1alpha1.ClusterList{}
			listOption := &client.ListOptions{UnsafeDisableDeepCopy: ptr.To(true)}
			if err := c.Client.List(ctx, clusterList, listOption); err != nil {
				klog.ErrorS(err, "Failed to list Cluster")
				return nil
			}

			policy := policyObj.(*policyv1alpha1.ClusterTaintPolicy)
			var requests []reconcile.Request
			for _, cluster := range clusterList.Items {
				if policy.Spec.TargetClusters == nil ||
					util.ClusterMatches(&cluster, *policy.Spec.TargetClusters) {
					requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: cluster.Name}})
				}
			}
			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&clusterv1alpha1.Cluster{}, builder.WithPredicates(clusterStatusConditionPredicateFn)).
		Watches(&policyv1alpha1.ClusterTaintPolicy{}, handler.EnqueueRequestsFromMapFunc(clusterTaintPolicyMapFunc),
			builder.WithPredicates(clusterTaintPolicyPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}
