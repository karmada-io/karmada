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
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "namespace-sync-controller"
)

// Controller is to sync Work.
type Controller struct {
	client.Client                // used to operate Work resources.
	EventRecorder                record.EventRecorder
	SkippedPropagatingNamespaces []*regexp.Regexp
	OverrideManager              overridemanager.OverrideManager
	RateLimiterOptions           ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Namespaces sync controller reconciling namespace", "namespace", req.NamespacedName.String())
	if !c.namespaceShouldBeSynced(req.Name) {
		return controllerruntime.Result{}, nil
	}

	namespace := &corev1.Namespace{}
	if err := c.Client.Get(ctx, req.NamespacedName, namespace); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	if !namespace.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	skipAutoPropagation := util.GetLabelValue(namespace.Labels, policyv1alpha1.NamespaceSkipAutoPropagationLabel)
	if strings.ToLower(skipAutoPropagation) == "true" {
		klog.InfoS("Skip auto propagation namespace", "namespace", namespace.Name)
		return controllerruntime.Result{}, nil
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		klog.ErrorS(err, "Failed to list clusters")
		return controllerruntime.Result{}, err
	}

	err := c.buildWorks(ctx, namespace, clusterList.Items)
	if err != nil {
		klog.ErrorS(err, "Failed to build work for namespace", "namespace", namespace.GetName())
		return controllerruntime.Result{}, err
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

func (c *Controller) buildWorks(ctx context.Context, namespace *corev1.Namespace, clusters []clusterv1alpha1.Cluster) error {
	namespaceObj, err := helper.ToUnstructured(namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to transform namespace", "namespace", namespace.GetName())
		return err
	}

	errChan := make(chan error, len(clusters))
	for i := range clusters {
		go func(cluster *clusterv1alpha1.Cluster, ch chan<- error) {
			clonedNamespaced := namespaceObj.DeepCopy()

			// namespace only care about ClusterOverridePolicy
			cops, _, err := c.OverrideManager.ApplyOverridePolicies(clonedNamespaced, cluster.Name)
			if err != nil {
				klog.ErrorS(err, "Failed to apply overrides for namespace", "namespace", clonedNamespaced.GetName(), "cluster", cluster.GetName())
				ch <- fmt.Errorf("sync namespace(%s) to cluster(%s) failed due to: %v", clonedNamespaced.GetName(), cluster.GetName(), err)
				return
			}

			annotations, err := binding.RecordAppliedOverrides(cops, nil, nil)
			if err != nil {
				klog.ErrorS(err, "Failed to record appliedOverrides")
				ch <- fmt.Errorf("sync namespace(%s) to cluster(%s) failed due to: %v", clonedNamespaced.GetName(), cluster.GetName(), err)
				return
			}

			workName := names.GenerateWorkName(namespaceObj.GetKind(), namespaceObj.GetName(), namespaceObj.GetNamespace())
			objectMeta := metav1.ObjectMeta{
				Name:       workName,
				Namespace:  names.GenerateExecutionSpaceName(cluster.Name),
				Finalizers: []string{util.ExecutionControllerFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(namespace, namespace.GroupVersionKind()),
				},
				Annotations: annotations,
			}

			if err = ctrlutil.CreateOrUpdateWork(ctx, c.Client, objectMeta, clonedNamespaced); err != nil {
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
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			var requests []reconcile.Request
			namespaceList := &corev1.NamespaceList{}
			if err := c.Client.List(ctx, namespaceList); err != nil {
				klog.ErrorS(err, "Failed to list namespace")
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
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(event.UpdateEvent) bool {
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
		func(_ context.Context, obj client.Object) []reconcile.Request {
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
					klog.ErrorS(err, "Failed to list namespace")
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
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(event.UpdateEvent) bool {
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
		Named(ControllerName).
		For(&corev1.Namespace{}).
		Watches(&clusterv1alpha1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterNamespaceFn),
			clusterPredicate).
		Watches(&policyv1alpha1.ClusterOverridePolicy{},
			handler.EnqueueRequestsFromMapFunc(clusterOverridePolicyNamespaceFn),
			clusterOverridePolicyPredicate).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).
		Complete(c)
}
