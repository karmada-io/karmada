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

package unifiedauth

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "unified-auth-controller"

	karmadaImpersonatorName = "karmada-impersonator"

	clusterAPIGroup       = "cluster.karmada.io"
	clusterProxyResource  = "clusters/proxy"
	searchAPIGroup        = "search.karmada.io"
	proxyingProxyResource = "proxying/proxy"
)

// Controller is to sync impersonation config to member clusters for unified authentication.
type Controller struct {
	client.Client      // used to operate Cluster resources.
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling cluster %s", req.NamespacedName.String())

	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	if cluster.Spec.ImpersonatorSecretRef == nil {
		klog.Infof("Unified auth is disabled on cluster %s as it does not have an impersonator secret", cluster.Name)
		return controllerruntime.Result{}, nil
	}

	err := c.syncImpersonationConfig(ctx, cluster)
	if err != nil {
		klog.Errorf("Failed to sync impersonation config for cluster %s. Error: %v.", cluster.Name, err)
		c.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, events.EventReasonSyncImpersonationConfigFailed, err.Error())
		return controllerruntime.Result{}, err
	}
	c.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, events.EventReasonSyncImpersonationConfigSucceed, "Sync impersonation config succeed.")

	return controllerruntime.Result{}, nil
}

func (c *Controller) syncImpersonationConfig(ctx context.Context, cluster *clusterv1alpha1.Cluster) error {
	// step1: find out target RBAC subjects.
	targetSubjects, err := findRBACSubjectsWithCluster(ctx, c.Client, cluster.Name)
	if err != nil {
		return err
	}

	// step2: generate rules for impersonation
	rules := util.GenerateImpersonationRules(targetSubjects)

	// step3: sync ClusterRole to cluster for impersonation
	if err := c.buildImpersonationClusterRole(ctx, cluster, rules); err != nil {
		klog.Errorf("Failed to sync impersonate ClusterRole to cluster(%s): %v", cluster.Name, err)
		return err
	}

	// step4: sync ClusterRoleBinding to cluster for impersonation
	if err := c.buildImpersonationClusterRoleBinding(ctx, cluster); err != nil {
		klog.Errorf("Failed to sync impersonate ClusterRoleBinding to cluster(%s): %v", cluster.Name, err)
		return err
	}

	return nil
}

func findRBACSubjectsWithCluster(ctx context.Context, c client.Client, cluster string) ([]rbacv1.Subject, error) {
	clusterRoleList := &rbacv1.ClusterRoleList{}
	if err := c.List(ctx, clusterRoleList); err != nil {
		klog.Errorf("Failed to list ClusterRoles, error: %v", err)
		return nil, err
	}

	matchedClusterRoles := sets.NewString()
	for _, clusterRole := range clusterRoleList.Items {
		for index := range clusterRole.Rules {
			if util.PolicyRuleAPIGroupMatches(&clusterRole.Rules[index], clusterAPIGroup) &&
				util.PolicyRuleResourceMatches(&clusterRole.Rules[index], clusterProxyResource) &&
				util.PolicyRuleResourceNameMatches(&clusterRole.Rules[index], cluster) {
				matchedClusterRoles.Insert(clusterRole.Name)
				break
			}

			if util.PolicyRuleAPIGroupMatches(&clusterRole.Rules[index], searchAPIGroup) &&
				util.PolicyRuleResourceMatches(&clusterRole.Rules[index], proxyingProxyResource) {
				matchedClusterRoles.Insert(clusterRole.Name)
				break
			}
		}
	}

	// short path: no matched ClusterRoles
	if matchedClusterRoles.Len() == 0 {
		return nil, nil
	}

	clusterRoleBindings := &rbacv1.ClusterRoleBindingList{}
	if err := c.List(ctx, clusterRoleBindings); err != nil {
		klog.Errorf("Failed to list ClusterRoleBindings, error: %v", err)
		return nil, err
	}

	var targetSubjects []rbacv1.Subject
	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		if clusterRoleBinding.RoleRef.Kind == util.ClusterRoleKind &&
			matchedClusterRoles.Has(clusterRoleBinding.RoleRef.Name) {
			targetSubjects = append(targetSubjects, clusterRoleBinding.Subjects...)
		}
	}
	return targetSubjects, nil
}

func (c *Controller) buildImpersonationClusterRole(ctx context.Context, cluster *clusterv1alpha1.Cluster, rules []rbacv1.PolicyRule) error {
	impersonationClusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       util.ClusterRoleKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: karmadaImpersonatorName,
			Labels: map[string]string{
				util.KarmadaSystemLabel: util.KarmadaSystemLabelValue,
			},
		},
		Rules: rules,
	}

	clusterRoleObj, err := helper.ToUnstructured(impersonationClusterRole)
	if err != nil {
		klog.Errorf("Failed to transform ClusterRole %s. Error: %v", impersonationClusterRole.GetName(), err)
		return err
	}

	return c.buildWorks(ctx, cluster, clusterRoleObj)
}

func (c *Controller) buildImpersonationClusterRoleBinding(ctx context.Context, cluster *clusterv1alpha1.Cluster) error {
	impersonatorClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       util.ClusterRoleBindingKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: karmadaImpersonatorName,
			Labels: map[string]string{
				util.KarmadaSystemLabel: util.KarmadaSystemLabelValue,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: cluster.Spec.ImpersonatorSecretRef.Namespace,
				Name:      names.GenerateServiceAccountName("impersonator"),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     util.ClusterRoleKind,
			Name:     karmadaImpersonatorName,
		},
	}

	clusterRoleBindingObj, err := helper.ToUnstructured(impersonatorClusterRoleBinding)
	if err != nil {
		klog.Errorf("Failed to transform ClusterRoleBinding %s. Error: %v", impersonatorClusterRoleBinding.GetName(), err)
		return err
	}

	return c.buildWorks(ctx, cluster, clusterRoleBindingObj)
}

func (c *Controller) buildWorks(ctx context.Context, cluster *clusterv1alpha1.Cluster, obj *unstructured.Unstructured) error {
	objectMeta := metav1.ObjectMeta{
		Name:       names.GenerateWorkName(obj.GetKind(), obj.GetName(), obj.GetNamespace()),
		Namespace:  names.GenerateExecutionSpaceName(cluster.Name),
		Finalizers: []string{util.ExecutionControllerFinalizer},
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(cluster, cluster.GroupVersionKind()),
		},
	}

	if err := ctrlutil.CreateOrUpdateWork(ctx, c.Client, objectMeta, obj); err != nil {
		return err
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	// clusterPredicateFunc only cares about create events
	clusterPredicateFunc := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&clusterv1alpha1.Cluster{}, builder.WithPredicates(clusterPredicateFunc)).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(c.newClusterRoleMapFunc())).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(c.newClusterRoleBindingMapFunc())).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *Controller) newClusterRoleMapFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		clusterRole := a.(*rbacv1.ClusterRole)
		return c.generateRequestsFromClusterRole(ctx, clusterRole)
	}
}

func (c *Controller) newClusterRoleBindingMapFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		clusterRoleBinding := a.(*rbacv1.ClusterRoleBinding)
		if clusterRoleBinding.RoleRef.Kind != util.ClusterRoleKind {
			return nil
		}

		clusterRole := &rbacv1.ClusterRole{}
		if err := c.Client.Get(ctx, types.NamespacedName{Name: clusterRoleBinding.RoleRef.Name}, clusterRole); err != nil {
			klog.Errorf("Failed to get reference ClusterRole, error: %v", err)
			return nil
		}
		return c.generateRequestsFromClusterRole(ctx, clusterRole)
	}
}

// generateRequestsFromClusterRole generates the requests for which clusters need be synced with impersonation config.
func (c *Controller) generateRequestsFromClusterRole(ctx context.Context, clusterRole *rbacv1.ClusterRole) []reconcile.Request {
	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		klog.Errorf("Failed to list existing clusters, error: %v", err)
		return nil
	}

	syncClusterNames := sets.NewString()
	for index, rule := range clusterRole.Rules {
		// for rule like:
		//   apiGroup: "cluster.karmada.io"
		//   resources: ["cluster/proxy"]
		//   resourceNames: ["cluster1", "cluster2"]
		if util.PolicyRuleAPIGroupMatches(&clusterRole.Rules[index], clusterAPIGroup) &&
			util.PolicyRuleResourceMatches(&clusterRole.Rules[index], clusterProxyResource) {
			if len(rule.ResourceNames) == 0 {
				// if the length of rule.ResourceNames is 0, means to match all clusters
				for _, cluster := range clusterList.Items {
					syncClusterNames.Insert(cluster.Name)
				}
				break
			}

			for _, resourceName := range rule.ResourceNames {
				syncClusterNames.Insert(resourceName)
			}
		}

		// for rule like:
		//   apiGroup: "search.karmada.io"
		//   resources: ["proxying/proxy"]
		if util.PolicyRuleAPIGroupMatches(&clusterRole.Rules[index], searchAPIGroup) &&
			util.PolicyRuleResourceMatches(&clusterRole.Rules[index], proxyingProxyResource) {
			// push all existing clusters into requests
			for _, cluster := range clusterList.Items {
				syncClusterNames.Insert(cluster.Name)
			}
			break
		}
	}

	var requests []reconcile.Request
	for _, clusterName := range syncClusterNames.List() {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName}})
	}
	return requests
}
