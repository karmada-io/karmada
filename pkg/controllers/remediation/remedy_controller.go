/*
Copyright 2024 The Karmada Authors.

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

package remediation

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// ControllerName is the controller name that will be used when reporting events and metrics.
const ControllerName = "remedy-controller"

// RemedyController is to sync Cluster resource, according to the cluster status
// condition, condition matching is performed through remedy, and then the actions
// required to be performed by the cluster are calculated.
type RemedyController struct {
	client.Client
	RateLimitOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RemedyController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Start to reconcile cluster", "cluster", req.NamespacedName.String())
	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	clusterRelatedRemedies, err := getClusterRelatedRemedies(ctx, c.Client, cluster)
	if err != nil {
		klog.ErrorS(err, "Failed to get cluster related remedies", "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}

	actions := calculateActions(clusterRelatedRemedies, cluster)
	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = helper.UpdateStatus(ctx, c.Client, cluster, func() error {
			cluster.Status.RemedyActions = actions
			return nil
		})
		return err
	}); err != nil {
		klog.ErrorS(err, "Failed to sync cluster remedy actions", "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}
	klog.V(4).InfoS("Success to sync cluster remedy actions", "cluster", cluster.Name, "actions", actions)
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *RemedyController) SetupWithManager(mgr controllerruntime.Manager) error {
	remedyController, err := controller.New(ControllerName, mgr, controller.Options{
		Reconciler:  c,
		RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimitOptions),
	})
	if err != nil {
		return err
	}

	err = c.setupWatches(remedyController, mgr)
	if err != nil {
		return err
	}
	return nil
}

func (c *RemedyController) setupWatches(remedyController controller.Controller, mgr controllerruntime.Manager) error {
	clusterChan := make(chan event.TypedGenericEvent[*clusterv1alpha1.Cluster])
	clusterHandler := newClusterEventHandler()
	remedyHandler := newRemedyEventHandler(clusterChan, c.Client)

	if err := remedyController.Watch(source.Kind[*clusterv1alpha1.Cluster](mgr.GetCache(), &clusterv1alpha1.Cluster{}, clusterHandler)); err != nil {
		return err
	}
	if err := remedyController.Watch(source.Channel(clusterChan, clusterHandler)); err != nil {
		return err
	}
	if err := remedyController.Watch(source.Kind(mgr.GetCache(), &remedyv1alpha1.Remedy{}, remedyHandler)); err != nil {
		return err
	}
	return nil
}
