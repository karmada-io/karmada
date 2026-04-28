/*
Copyright 2019 The Kubernetes Authors.

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

package config

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HPAControllerConfiguration contains elements describing HPAController.
type HPAControllerConfiguration struct {
	// horizontalPodAutoscalerSyncPeriod is the period for syncing the number of
	// pods in horizontal pod autoscaler.
	HorizontalPodAutoscalerSyncPeriod metav1.Duration
	// horizontalPodAutoscalerUpscaleForbiddenWindow is a period after which next upscale allowed.
	HorizontalPodAutoscalerUpscaleForbiddenWindow metav1.Duration
	// horizontalPodAutoscalerDownscaleForbiddenWindow is a period after which next downscale allowed.
	HorizontalPodAutoscalerDownscaleForbiddenWindow metav1.Duration
	// HorizontalPodAutoscalerDowncaleStabilizationWindow is a period for which autoscaler will look
	// backwards and not scale down below any recommendation it made during that period.
	HorizontalPodAutoscalerDownscaleStabilizationWindow metav1.Duration
	// horizontalPodAutoscalerTolerance is the tolerance for when
	// resource usage suggests upscaling/downscaling
	HorizontalPodAutoscalerTolerance float64
	// HorizontalPodAutoscalerCPUInitializationPeriod is the period after pod start when CPU samples
	// might be skipped.
	HorizontalPodAutoscalerCPUInitializationPeriod metav1.Duration
	// HorizontalPodAutoscalerInitialReadinessDelay is period after pod start during which readiness
	// changes are treated as readiness being set for the first time. The only effect of this is that
	// HPA will disregard CPU samples from unready pods that had last readiness change during that
	// period.
	HorizontalPodAutoscalerInitialReadinessDelay metav1.Duration
}

// AddFlags adds flags related to HPAController for controller manager to the specified FlagSet.
func (o *HPAControllerConfiguration) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.DurationVar(&o.HorizontalPodAutoscalerSyncPeriod.Duration, "horizontal-pod-autoscaler-sync-period", 15*time.Second, "The period for syncing the number of pods in horizontal pod autoscaler.")
	fs.DurationVar(&o.HorizontalPodAutoscalerUpscaleForbiddenWindow.Duration, "horizontal-pod-autoscaler-upscale-delay", 3*time.Minute, "The period since last upscale, before another upscale can be performed in horizontal pod autoscaler.")
	fs.DurationVar(&o.HorizontalPodAutoscalerDownscaleStabilizationWindow.Duration, "horizontal-pod-autoscaler-downscale-stabilization", 5*time.Minute, "The period for which autoscaler will look backwards and not scale down below any recommendation it made during that period.")
	fs.DurationVar(&o.HorizontalPodAutoscalerDownscaleForbiddenWindow.Duration, "horizontal-pod-autoscaler-downscale-delay", 5*time.Minute, "The period since last downscale, before another downscale can be performed in horizontal pod autoscaler.")
	fs.Float64Var(&o.HorizontalPodAutoscalerTolerance, "horizontal-pod-autoscaler-tolerance", 0.1, "The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.")
	fs.DurationVar(&o.HorizontalPodAutoscalerCPUInitializationPeriod.Duration, "horizontal-pod-autoscaler-cpu-initialization-period", 5*time.Minute, "The period after pod start when CPU samples might be skipped.")
	fs.DurationVar(&o.HorizontalPodAutoscalerInitialReadinessDelay.Duration, "horizontal-pod-autoscaler-initial-readiness-delay", 30*time.Second, "The period after pod start during which readiness changes will be treated as initial readiness.")
}
