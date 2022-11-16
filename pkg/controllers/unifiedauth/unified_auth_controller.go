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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "unified-auth-controller"

	rbacAPIVersion          = "rbac.authorization.k8s.io/v1"
	clusterProxyResource    = "clusters/proxy"
	clusterProxyAPIGroup    = "cluster.karmada.io"
	karmadaImpersonatorName = "karmada-impersonator"
)

// Controller is to sync impersonation config to member clusters for unified authentication.
type Controller struct {
	client.Client // used to operate Cluster resources.
	EventRecorder record.EventRecorder
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

		return controllerruntime.Result{Requeue: true}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	if cluster.Spec.ImpersonatorSecretRef == nil {
		klog.Infof("Aggregated API feature is disabled on cluster %s as it does not have an impersonator secret", cluster.Name)
		return controllerruntime.Result{}, nil
	}

	err := c.syncImpersonationConfig(cluster)
	if err != nil {
		klog.Errorf("Failed to sync impersonation config for cluster %s. Error: %v.", cluster.Name, err)
		c.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, events.EventReasonSyncImpersonationConfigFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}
	c.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, events.EventReasonSyncImpersonationConfigSucceed, "Sync impersonation config succeed.")

	return controllerruntime.Result{}, nil
}

func (c *Controller) syncImpersonationConfig(cluster *clusterv1alpha1.Cluster) error {
	// step1: list all clusterroles
	clusterRoleList := &rbacv1.ClusterRoleList{}
	if err := c.Client.List(context.TODO(), clusterRoleList); err != nil {
		klog.Errorf("Failed to list clusterroles, error: %v", err)
		return err
	}

	// step2: found out clusterroles that matches current cluster
	allMatchedClusterRoles := sets.NewString()
	for _, clusterRole := range clusterRoleList.Items {
		for i := range clusterRole.Rules {
			if util.PolicyRuleAPIGroupMatches(&clusterRole.Rules[i], clusterProxyAPIGroup) &&
				util.PolicyRuleResourceMatches(&clusterRole.Rules[i], clusterProxyResource) &&
				util.PolicyRuleResourceNameMatches(&clusterRole.Rules[i], cluster.Name) {
				allMatchedClusterRoles.Insert(clusterRole.Name)
				break
			}
		}
	}

	// step3: found out reference clusterRolebindings and collecting subjects.
	clusterRoleBindings := &rbacv1.ClusterRoleBindingList{}
	var allSubjects []rbacv1.Subject
	if len(allMatchedClusterRoles) != 0 {
		if err := c.Client.List(context.TODO(), clusterRoleBindings); err != nil {
			klog.Errorf("Failed to list clusterrolebindings, error: %v", err)
			return err
		}

		for _, clusterRoleBinding := range clusterRoleBindings.Items {
			if clusterRoleBinding.RoleRef.Kind == util.ClusterRoleKind && allMatchedClusterRoles.Has(clusterRoleBinding.RoleRef.Name) {
				allSubjects = append(allSubjects, clusterRoleBinding.Subjects...)
			}
		}
	}

	// step4:  generate rules for impersonation
	rules := util.GenerateImpersonationRules(allSubjects)

	// step5: sync clusterrole to cluster for impersonation
	if err := c.buildImpersonationClusterRole(cluster, rules); err != nil {
		klog.Errorf("failed to sync impersonate clusterrole to cluster(%s): %v", cluster.Name, err)
		return err
	}

	// step6: sync clusterrolebinding to cluster for impersonation
	if err := c.buildImpersonationClusterRoleBinding(cluster); err != nil {
		klog.Errorf("failed to sync impersonate clusterrolebinding to cluster(%s): %v", cluster.Name, err)
		return err
	}

	return nil
}

func (c *Controller) buildImpersonationClusterRole(cluster *clusterv1alpha1.Cluster, rules []rbacv1.PolicyRule) error {
	impersonationClusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacAPIVersion,
			Kind:       util.ClusterRoleKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: karmadaImpersonatorName,
		},
		Rules: rules,
	}

	clusterRoleObj, err := helper.ToUnstructured(impersonationClusterRole)
	if err != nil {
		klog.Errorf("Failed to transform clusterrole %s. Error: %v", impersonationClusterRole.GetName(), err)
		return err
	}

	return c.buildWorks(cluster, clusterRoleObj)
}

func (c *Controller) buildImpersonationClusterRoleBinding(cluster *clusterv1alpha1.Cluster) error {
	impersonatorClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacAPIVersion,
			Kind:       util.ClusterRoleBindingKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: karmadaImpersonatorName,
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
		klog.Errorf("Failed to transform clusterrolebinding %s. Error: %v", impersonatorClusterRoleBinding.GetName(), err)
		return err
	}

	return c.buildWorks(cluster, clusterRoleBindingObj)
}

func (c *Controller) buildWorks(cluster *clusterv1alpha1.Cluster, obj *unstructured.Unstructured) error {
	workNamespace, err := names.GenerateExecutionSpaceName(cluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", cluster.Name, err)
		return err
	}

	clusterRoleBindingWorkName := names.GenerateWorkName(obj.GetKind(), obj.GetName(), obj.GetNamespace())
	objectMeta := metav1.ObjectMeta{
		Name:       clusterRoleBindingWorkName,
		Namespace:  workNamespace,
		Finalizers: []string{util.ExecutionControllerFinalizer},
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(cluster, cluster.GroupVersionKind()),
		},
	}

	util.MergeLabel(obj, workv1alpha1.WorkNamespaceLabel, workNamespace)
	util.MergeLabel(obj, workv1alpha1.WorkNameLabel, clusterRoleBindingWorkName)

	if err = helper.CreateOrUpdateWork(c.Client, objectMeta, obj); err != nil {
		return err
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	// clusterPredicateFunc only cares about create events
	clusterPredicateFunc := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.Cluster{}, builder.WithPredicates(clusterPredicateFunc)).
		Watches(&source.Kind{Type: &rbacv1.ClusterRole{}}, handler.EnqueueRequestsFromMapFunc(c.newClusterRoleMapFunc())).
		Watches(&source.Kind{Type: &rbacv1.ClusterRoleBinding{}}, handler.EnqueueRequestsFromMapFunc(c.newClusterRoleBindingMapFunc())).
		Complete(c)
}

func (c *Controller) newClusterRoleMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		clusterRole := a.(*rbacv1.ClusterRole)
		return c.generateRequestsFromClusterRole(clusterRole)
	}
}

func (c *Controller) newClusterRoleBindingMapFunc() handler.MapFunc {
	return func(a client.Object) []reconcile.Request {
		clusterRoleBinding := a.(*rbacv1.ClusterRoleBinding)
		if clusterRoleBinding.RoleRef.Kind != util.ClusterRoleKind {
			return nil
		}

		clusterRole := &rbacv1.ClusterRole{}
		if err := c.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleBinding.RoleRef.Name}, clusterRole); err != nil {
			klog.Errorf("Failed to get reference clusterrole, error: %v", err)
			return nil
		}
		return c.generateRequestsFromClusterRole(clusterRole)
	}
}

// found out which clusters need to sync impersonation config from rules like:
//   resources: ["cluster/proxy"]
//   resourceNames: ["cluster1", "cluster2"]
func (c *Controller) generateRequestsFromClusterRole(clusterRole *rbacv1.ClusterRole) []reconcile.Request {
	var requests []reconcile.Request
	for i := range clusterRole.Rules {
		if util.PolicyRuleAPIGroupMatches(&clusterRole.Rules[i], clusterProxyAPIGroup) &&
			util.PolicyRuleResourceMatches(&clusterRole.Rules[i], clusterProxyResource) {
			if len(clusterRole.Rules[i].ResourceNames) == 0 {
				// if the length of rule[i].ResourceNames is 0, means to match all clusters
				clusterList := &clusterv1alpha1.ClusterList{}
				if err := c.Client.List(context.TODO(), clusterList); err != nil {
					klog.Errorf("Failed to list clusters, error: %v", err)
					return nil
				}
				for _, cluster := range clusterList.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
						Name: cluster.Name,
					}})
				}
			} else {
				for _, resourceName := range clusterRole.Rules[i].ResourceNames {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
						Name: resourceName,
					}})
				}
			}
		}
	}
	return requests
}
