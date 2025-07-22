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

package deploymentreplicassyncer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "deployment-replicas-syncer"

	waitDeploymentStatusInterval = 1 * time.Second
)

// DeploymentReplicasSyncer is to sync deployment replicas from status field to spec field.
type DeploymentReplicasSyncer struct {
	Client             client.Client
	RateLimiterOptions ratelimiterflag.Options
}

var predicateFunc = predicate.Funcs{
	CreateFunc: func(event.CreateEvent) bool { return false },
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldObj := e.ObjectOld.(*appsv1.Deployment)
		newObj := e.ObjectNew.(*appsv1.Deployment)

		// if deployment is not labeled `retain-replicas`, means it is not controlled by hpa, ignore the event
		retainReplicasLabel := util.GetLabelValue(newObj.GetLabels(), util.RetainReplicasLabel)
		if retainReplicasLabel != util.RetainReplicasValue {
			return false
		}

		// if old deployment is not labeled `retain-replicas`, but new is labeled, reconcile is needed.
		// in case of deployment status changed before `retain-replicas` labeled.
		oldRetainReplicasLabel := util.GetLabelValue(oldObj.GetLabels(), util.RetainReplicasLabel)
		if oldRetainReplicasLabel != util.RetainReplicasValue {
			return true
		}

		if oldObj.Spec.Replicas == nil || newObj.Spec.Replicas == nil {
			klog.ErrorS(fmt.Errorf("spec.replicas is nil in either old or new deployment object"),
				"Unexpected nil spec.replicas",
				"oldReplicas", oldObj.Spec.Replicas, "newReplicas", newObj.Spec.Replicas,
				"namespace", oldObj.Namespace, "name", oldObj.Name)
			return false
		}

		// if replicas field has no change, either in spec.replicas or in status.replicas
		if *oldObj.Spec.Replicas == *newObj.Spec.Replicas && oldObj.Status.Replicas == newObj.Status.Replicas {
			return false
		}

		return true
	},
	DeleteFunc:  func(event.DeleteEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
}

// SetupWithManager creates a controller and register to controller manager.
func (r *DeploymentReplicasSyncer) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&appsv1.Deployment{}, builder.WithPredicates(predicateFunc)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](r.RateLimiterOptions)}).
		Complete(r)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *DeploymentReplicasSyncer) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling for Deployment", "namespace", req.Namespace, "name", req.Name)

	deployment := &appsv1.Deployment{}
	binding := &workv1alpha2.ResourceBinding{}
	bindingName := names.GenerateBindingName(util.DeploymentKind, req.Name)

	// 1. get latest binding
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: bindingName}, binding); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("no need to update deployment replicas as binding was not found",
				"namespace", req.Namespace, "name", req.Name)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// 2. if it is not divided schedule type, no need to update replicas
	if binding.Spec.Placement.ReplicaSchedulingType() != policyv1alpha1.ReplicaSchedulingTypeDivided {
		return controllerruntime.Result{}, nil
	}

	// 3. get latest deployment
	if err := r.Client.Get(ctx, req.NamespacedName, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("no need to update deployment replicas as deployment was not found",
				"namespace", req.Namespace, "name", req.Name)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// 4. if replicas in spec already the same as in status, no need to update replicas
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == deployment.Status.Replicas {
		klog.InfoS("replicas in spec field are already equal to those in status field",
			"specReplicas", *deployment.Spec.Replicas, "statusReplicas", deployment.Status.Replicas,
			"namespace", deployment.Namespace, "name", deployment.Name)
		return controllerruntime.Result{}, nil
	}

	// 5. judge the deployment modification in spec has taken effect and its status has been collected, otherwise retry.
	if !isDeploymentStatusCollected(deployment, binding) {
		return controllerruntime.Result{RequeueAfter: waitDeploymentStatusInterval}, nil
	}

	// 6. update spec replicas with status replicas
	oldReplicas := *deployment.Spec.Replicas
	deployment.Spec.Replicas = &deployment.Status.Replicas
	if err := r.Client.Update(ctx, deployment); err != nil {
		klog.ErrorS(err, "failed to update deployment replicas", "namespace", deployment.Namespace, "name", deployment.Name)
		return controllerruntime.Result{}, err
	}

	klog.InfoS("successfully update deployment replicas",
		"namespace", deployment.Namespace, "name", deployment.Name,
		"oldReplicas", oldReplicas, "newReplicas", deployment.Status.Replicas)

	return controllerruntime.Result{}, nil
}

// isDeploymentStatusCollected judge whether deployment modification in spec has taken effect and its status has been collected.
func isDeploymentStatusCollected(deployment *appsv1.Deployment, binding *workv1alpha2.ResourceBinding) bool {
	// make sure the replicas change in deployment.spec can sync to binding.spec, otherwise retry
	if deployment.Spec.Replicas == nil {
		// should never happen, as karmada-apiserver defaults `deployment.spec.Replicas` to 1 if it is not set.
		klog.ErrorS(nil, "deployment replicas is nil", "namespace", deployment.Namespace, "name", deployment.Name)
		return false
	}
	if *deployment.Spec.Replicas != binding.Spec.Replicas {
		klog.V(4).InfoS("wait until binding replicas are equal to deployment replicas",
			"bindingReplicas", binding.Spec.Replicas, "deploymentReplicas", *deployment.Spec.Replicas,
			"namespace", deployment.Namespace, "deploymentName", deployment.Name, "bindingName", binding.Name)
		return false
	}

	// make sure the scheduler observed generation equal to generation in binding, otherwise retry
	if binding.Generation != binding.Status.SchedulerObservedGeneration {
		klog.V(4).InfoS("wait until scheduler observed generation is equal to generation in binding",
			"schedulerObservedGeneration", binding.Status.SchedulerObservedGeneration, "generation", binding.Generation,
			"namespace", binding.Namespace, "name", binding.Name)
		return false
	}

	// make sure the number of aggregated status in binding is as expected.
	if len(binding.Status.AggregatedStatus) != len(binding.Spec.Clusters) {
		klog.V(4).InfoS("wait until all clusters' statuses are collected",
			"got", len(binding.Status.AggregatedStatus), "expected", len(binding.Spec.Clusters),
			"namespace", binding.Namespace, "name", binding.Name)
		return false
	}

	// make sure the status.replicas in binding has been collected from work to binding.
	bindingStatusSumReplicas := int32(0)
	for _, status := range binding.Status.AggregatedStatus {
		if status.Status == nil {
			klog.V(4).InfoS("wait until aggregated status of cluster is collected", "cluster", status.ClusterName)
			return false
		}
		itemStatus := &appsv1.DeploymentStatus{}
		if err := json.Unmarshal(status.Status.Raw, itemStatus); err != nil {
			klog.ErrorS(err, "Failed to unmarshal aggregated status", "cluster", status.ClusterName)
			return false
		}
		// if member cluster deployment is controlled by hpa, its status.replicas must > 0 (since hpa.minReplicas > 0),
		// so, if its status replicas is 0, means the status haven't been collected from member cluster, and needs waiting.
		if itemStatus.Replicas <= 0 {
			klog.V(4).InfoS("wait until aggregated status replicas of cluster are collected", "cluster", status.ClusterName)
			return false
		}
		bindingStatusSumReplicas += itemStatus.Replicas
	}

	// make sure the latest collected status.replicas is synced from binding to template.
	// e.g: user set deployment.spec.replicas = 2, then sum replicas in binding.status becomes 2, however, if now
	// deployment.status.replicas is still 1, we shouldn't update spec.replicas to 1 until status.replicas becomes 2.
	if deployment.Status.Replicas != bindingStatusSumReplicas {
		klog.V(4).InfoS("wait until deployment status replicas are equal to binding status replicas",
			"deploymentStatusReplicas", deployment.Status.Replicas, "bindingStatusReplicas", bindingStatusSumReplicas,
			"namespace", binding.Namespace, "bindingName", binding.Name, "deploymentName", deployment.Name)
		return false
	}

	return true
}
