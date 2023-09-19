package hpareplicassyncer

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

var _ predicate.Predicate = &HPAReplicasSyncer{}

func (r *HPAReplicasSyncer) Create(e event.CreateEvent) bool {
	hpa, ok := e.Object.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		klog.Errorf("create predicates in hpa controller called, but obj is not hpa type")
		return false
	}

	// if hpa exist and has been propagated, add label to its scale ref resource
	if hasBeenPropagated(hpa) {
		r.scaleRefWorker.Add(labelEvent{addLabelEvent, hpa})
	}

	return false
}

func (r *HPAReplicasSyncer) Update(e event.UpdateEvent) bool {
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
		r.scaleRefWorker.Add(labelEvent{deleteLabelEvent, oldHPA})
	}

	// if new hpa exist and has been propagated, add label to its scale ref resource
	if hasBeenPropagated(newHPA) {
		r.scaleRefWorker.Add(labelEvent{addLabelEvent, newHPA})
	}

	return oldHPA.Status.CurrentReplicas != newHPA.Status.CurrentReplicas
}

func (r *HPAReplicasSyncer) Delete(e event.DeleteEvent) bool {
	hpa, ok := e.Object.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		klog.Errorf("delete predicates in hpa controller called, but obj is not hpa type")
		return false
	}

	// if scale ref has label, remove label, otherwise skip
	r.scaleRefWorker.Add(labelEvent{deleteLabelEvent, hpa})

	return false
}

func (r *HPAReplicasSyncer) Generic(e event.GenericEvent) bool {
	return false
}

func hasBeenPropagated(hpa *autoscalingv2.HorizontalPodAutoscaler) bool {
	_, ppExist := hpa.GetLabels()[policyv1alpha1.PropagationPolicyUIDLabel]
	_, cppExist := hpa.GetLabels()[policyv1alpha1.ClusterPropagationPolicyUIDLabel]
	return ppExist || cppExist
}
