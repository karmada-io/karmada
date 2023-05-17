package namespace

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "namespace-sync-controller"
)

// Controller is to sync Work.
type Controller struct {
	client.Client                // used to operate Work resources.
	EventRecorder                record.EventRecorder
	SkippedPropagatingNamespaces []*regexp.Regexp
	OverrideManager              overridemanager.OverrideManager
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Namespaces sync controller reconciling %s", req.NamespacedName.String())
	if !c.namespaceShouldBeSynced(req.Name) {
		return controllerruntime.Result{}, nil
	}

	namespace := &corev1.Namespace{}
	if err := c.Client.Get(ctx, req.NamespacedName, namespace); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !namespace.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	skipAutoPropagation := util.GetLabelValue(namespace.Labels, policyv1alpha1.NamespaceSkipAutoPropagationLabel)
	if strings.ToLower(skipAutoPropagation) == "true" {
		klog.Infof("Skip auto propagation namespace:%s", namespace.Name)
		return controllerruntime.Result{}, nil
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		klog.Errorf("Failed to list clusters, error: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err := c.buildWorks(namespace, clusterList.Items)
	if err != nil {
		klog.Errorf("Failed to build work for namespace %s. Error: %v.", namespace.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *Controller) namespaceShouldBeSynced(namespace string) bool {
	if names.IsReservedNamespace(namespace) || namespace == names.NamespaceDefault {
		return false
	}

	for _, nsRegexp := range c.SkippedPropagatingNamespaces {
		if match := nsRegexp.MatchString(namespace); match {
			return false
		}
	}
	return true
}

func (c *Controller) buildWorks(namespace *corev1.Namespace, clusters []clusterv1alpha1.Cluster) error {
	namespaceObj, err := helper.ToUnstructured(namespace)
	if err != nil {
		klog.Errorf("Failed to transform namespace %s. Error: %v", namespace.GetName(), err)
		return err
	}

	errChan := make(chan error, len(clusters))
	for i := range clusters {
		go func(cluster *clusterv1alpha1.Cluster, ch chan<- error) {
			clonedNamespaced := namespaceObj.DeepCopy()

			// namespace only care about ClusterOverridePolicy
			cops, _, err := c.OverrideManager.ApplyOverridePolicies(clonedNamespaced, cluster.Name)
			if err != nil {
				klog.Errorf("Failed to apply overrides for %s/%s/%s, err is: %v", clonedNamespaced.GetKind(), clonedNamespaced.GetNamespace(), clonedNamespaced.GetName(), err)
				ch <- fmt.Errorf("sync namespace(%s) to cluster(%s) failed due to: %v", clonedNamespaced.GetName(), cluster.GetName(), err)
				return
			}

			annotations, err := binding.RecordAppliedOverrides(cops, nil, nil)
			if err != nil {
				klog.Errorf("Failed to record appliedOverrides, Error: %v", err)
				ch <- fmt.Errorf("sync namespace(%s) to cluster(%s) failed due to: %v", clonedNamespaced.GetName(), cluster.GetName(), err)
				return
			}

			workNamespace := names.GenerateExecutionSpaceName(cluster.Name)

			workName := names.GenerateWorkName(namespaceObj.GetKind(), namespaceObj.GetName(), namespaceObj.GetNamespace())
			objectMeta := metav1.ObjectMeta{
				Name:       workName,
				Namespace:  workNamespace,
				Finalizers: []string{util.ExecutionControllerFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(namespace, namespace.GroupVersionKind()),
				},
				Annotations: annotations,
			}

			util.MergeLabel(clonedNamespaced, workv1alpha1.WorkNamespaceLabel, workNamespace)
			util.MergeLabel(clonedNamespaced, workv1alpha1.WorkNameLabel, workName)
			util.MergeLabel(clonedNamespaced, util.ManagedByKarmadaLabel, util.ManagedByKarmadaLabelValue)

			if err = helper.CreateOrUpdateWork(c.Client, objectMeta, clonedNamespaced); err != nil {
				ch <- fmt.Errorf("sync namespace(%s) to cluster(%s) failed due to: %v", clonedNamespaced.GetName(), cluster.GetName(), err)
				return
			}
			ch <- nil
		}(&clusters[i], errChan)
	}

	var errs []error
	for range clusters {
		if err := <-errChan; err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	clusterNamespaceFn := handler.MapFunc(
		func(a client.Object) []reconcile.Request {
			var requests []reconcile.Request
			namespaceList := &corev1.NamespaceList{}
			if err := c.Client.List(context.TODO(), namespaceList); err != nil {
				klog.Errorf("Failed to list namespace, error: %v", err)
				return nil
			}

			for _, namespace := range namespaceList.Items {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name: namespace.Name,
				}})
			}
			return requests
		})

	clusterPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})

	clusterOverridePolicyNamespaceFn := handler.MapFunc(
		func(obj client.Object) []reconcile.Request {
			var requests []reconcile.Request
			cop, ok := obj.(*policyv1alpha1.ClusterOverridePolicy)
			if !ok {
				return requests
			}

			selectedNamespaces := sets.NewString()
			containsAllNamespace := false
			for _, rs := range cop.Spec.ResourceSelectors {
				if rs.APIVersion != "v1" || rs.Kind != "Namespace" {
					continue
				}

				if rs.Name == "" {
					containsAllNamespace = true
					break
				}

				selectedNamespaces.Insert(rs.Name)
			}

			if containsAllNamespace {
				namespaceList := &corev1.NamespaceList{}
				if err := c.Client.List(context.TODO(), namespaceList); err != nil {
					klog.Errorf("Failed to list namespace, error: %v", err)
					return nil
				}

				for _, namespace := range namespaceList.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
						Name: namespace.Name,
					}})
				}

				return requests
			}

			for _, ns := range selectedNamespaces.UnsortedList() {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name: ns,
				}})
			}

			return requests
		})

	clusterOverridePolicyPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&source.Kind{Type: &clusterv1alpha1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(clusterNamespaceFn),
			clusterPredicate).
		Watches(&source.Kind{Type: &policyv1alpha1.ClusterOverridePolicy{}},
			handler.EnqueueRequestsFromMapFunc(clusterOverridePolicyNamespaceFn),
			clusterOverridePolicyPredicate).
		Complete(c)
}
