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

package hpascaletargetmarker

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "hpa-scale-target-marker"
	// scaleTargetWorkerNum is the async Worker number
	scaleTargetWorkerNum = 1
)

// HpaScaleTargetMarker is to automatically add `retain-replicas` label to resource template managed by HPA.
type HpaScaleTargetMarker struct {
	DynamicClient dynamic.Interface
	RESTMapper    meta.RESTMapper

	scaleTargetWorker  util.AsyncWorker
	RateLimiterOptions ratelimiterflag.Options
}

// SetupWithManager creates a controller and register to controller manager.
func (r *HpaScaleTargetMarker) SetupWithManager(mgr controllerruntime.Manager) error {
	scaleTargetWorkerOptions := util.Options{
		Name:          "scale target worker",
		ReconcileFunc: r.reconcileScaleRef,
	}
	r.scaleTargetWorker = util.NewAsyncWorker(scaleTargetWorkerOptions)
	r.scaleTargetWorker.Run(context.Background(), scaleTargetWorkerNum)

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&autoscalingv2.HorizontalPodAutoscaler{}, builder.WithPredicates(r)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](r.RateLimiterOptions),
		}).
		Complete(r)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *HpaScaleTargetMarker) Reconcile(_ context.Context, _ controllerruntime.Request) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
