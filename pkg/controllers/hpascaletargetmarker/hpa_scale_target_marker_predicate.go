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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

var _ predicate.Predicate = &HpaScaleTargetMarker{}

// Create implements CreateEvent filter
func (r *HpaScaleTargetMarker) Create(e event.CreateEvent) bool {
	hpa, ok := e.Object.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		klog.Errorf("create predicates in hpa controller called, but obj is not hpa type")
		return false
	}

	// if hpa exist and has been propagated, add label to its scale ref resource
	if hasBeenPropagated(hpa) {
		r.scaleTargetWorker.Add(labelEvent{addLabelEvent, hpa})
	}

	return false
}

// Update implements UpdateEvent filter
func (r *HpaScaleTargetMarker) Update(e event.UpdateEvent) bool {
	oldHPA, ok := e.ObjectOld.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		klog.Errorf("update predicates in hpa controller called, but old obj is not hpa type")
		return false
	}

	newHPA, ok := e.ObjectNew.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		klog.Errorf("update predicates in hpa controller called, but new obj is not hpa type")
		return false
	}

	// hpa scale ref changed, remove old hpa label and add to new hpa
	if oldHPA.Spec.ScaleTargetRef.String() != newHPA.Spec.ScaleTargetRef.String() {
		// if scale ref has label, remove label, otherwise skip
		r.scaleTargetWorker.Add(labelEvent{deleteLabelEvent, oldHPA})
	}

	// if new hpa exist and has been propagated, add label to its scale ref resource
	if hasBeenPropagated(newHPA) {
		r.scaleTargetWorker.Add(labelEvent{addLabelEvent, newHPA})
	}

	return oldHPA.Status.DesiredReplicas != newHPA.Status.DesiredReplicas
}

// Delete implements DeleteEvent filter
func (r *HpaScaleTargetMarker) Delete(e event.DeleteEvent) bool {
	hpa, ok := e.Object.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		klog.Errorf("delete predicates in hpa controller called, but obj is not hpa type")
		return false
	}

	// if scale ref has label, remove label, otherwise skip
	r.scaleTargetWorker.Add(labelEvent{deleteLabelEvent, hpa})

	return false
}

// Generic implements default GenericEvent filter
func (r *HpaScaleTargetMarker) Generic(_ event.GenericEvent) bool {
	return false
}

func hasBeenPropagated(hpa *autoscalingv2.HorizontalPodAutoscaler) bool {
	_, ppExist := hpa.GetLabels()[policyv1alpha1.PropagationPolicyPermanentIDLabel]
	_, cppExist := hpa.GetLabels()[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]
	return ppExist || cppExist
}
