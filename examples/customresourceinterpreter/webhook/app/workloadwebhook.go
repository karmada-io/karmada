package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/webhook/interpreter"
)

// Check if our workloadInterpreter implements necessary interface
var _ interpreter.Handler = &workloadInterpreter{}
var _ interpreter.DecoderInjector = &workloadInterpreter{}

// workloadInterpreter explore resource with request operation.
type workloadInterpreter struct {
	decoder *interpreter.Decoder
}

// Handle implements interpreter.Handler interface.
// It yields a response to an ExploreRequest.
func (e *workloadInterpreter) Handle(ctx context.Context, req interpreter.Request) interpreter.Response {
	workload := &workloadv1alpha1.Workload{}
	err := e.decoder.Decode(req, workload)
	if err != nil {
		return interpreter.Errored(http.StatusBadRequest, err)
	}
	klog.Infof("Explore workload(%s/%s) for request: %s", workload.GetNamespace(), workload.GetName(), req.Operation)

	switch req.Operation {
	case configv1alpha1.InterpreterOperationInterpretReplica:
		return e.responseWithExploreReplica(workload)
	case configv1alpha1.InterpreterOperationReviseReplica:
		return e.responseWithExploreReviseReplica(workload, req)
	case configv1alpha1.InterpreterOperationRetain:
		return e.responseWithExploreRetaining(workload, req)
	case configv1alpha1.InterpreterOperationAggregateStatus:
		return e.responseWithExploreAggregateStatus(workload, req)
	case configv1alpha1.InterpreterOperationInterpretHealth:
		return e.responseWithExploreInterpretHealth(workload)
	case configv1alpha1.InterpreterOperationInterpretStatus:
		return e.responseWithExploreInterpretStatus(workload)
	default:
		return interpreter.Errored(http.StatusBadRequest, fmt.Errorf("wrong request operation type: %s", req.Operation))
	}
}

// InjectDecoder implements interpreter.DecoderInjector interface.
func (e *workloadInterpreter) InjectDecoder(d *interpreter.Decoder) {
	e.decoder = d
}

func (e *workloadInterpreter) responseWithExploreReplica(workload *workloadv1alpha1.Workload) interpreter.Response {
	res := interpreter.Succeeded("")
	res.Replicas = workload.Spec.Replicas
	return res
}

func (e *workloadInterpreter) responseWithExploreReviseReplica(workload *workloadv1alpha1.Workload, req interpreter.Request) interpreter.Response {
	wantedWorkload := workload.DeepCopy()
	wantedWorkload.Spec.Replicas = req.DesiredReplicas
	marshaledBytes, err := json.Marshal(wantedWorkload)
	if err != nil {
		return interpreter.Errored(http.StatusInternalServerError, err)
	}
	return interpreter.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (e *workloadInterpreter) responseWithExploreRetaining(desiredWorkload *workloadv1alpha1.Workload, req interpreter.Request) interpreter.Response {
	if req.ObservedObject == nil {
		err := fmt.Errorf("nil observedObject in exploreReview with operation type: %s", req.Operation)
		return interpreter.Errored(http.StatusBadRequest, err)
	}
	observerWorkload := &workloadv1alpha1.Workload{}
	err := e.decoder.DecodeRaw(*req.ObservedObject, observerWorkload)
	if err != nil {
		return interpreter.Errored(http.StatusBadRequest, err)
	}

	// Suppose we want to retain the `.spec.paused` field of the actual observed workload object in member cluster,
	// and prevent from being overwritten by karmada controller-plane.
	wantedWorkload := desiredWorkload.DeepCopy()
	wantedWorkload.Spec.Paused = observerWorkload.Spec.Paused
	marshaledBytes, err := json.Marshal(wantedWorkload)
	if err != nil {
		return interpreter.Errored(http.StatusInternalServerError, err)
	}
	return interpreter.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (e *workloadInterpreter) responseWithExploreAggregateStatus(workload *workloadv1alpha1.Workload, req interpreter.Request) interpreter.Response {
	wantedWorkload := workload.DeepCopy()
	var readyReplicas int32
	for _, item := range req.AggregatedStatus {
		if item.Status == nil {
			continue
		}
		status := &workloadv1alpha1.WorkloadStatus{}
		if err := json.Unmarshal(item.Status.Raw, status); err != nil {
			return interpreter.Errored(http.StatusInternalServerError, err)
		}
		readyReplicas += status.ReadyReplicas
	}
	wantedWorkload.Status.ReadyReplicas = readyReplicas
	marshaledBytes, err := json.Marshal(wantedWorkload)
	if err != nil {
		return interpreter.Errored(http.StatusInternalServerError, err)
	}
	return interpreter.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

func (e *workloadInterpreter) responseWithExploreInterpretHealth(workload *workloadv1alpha1.Workload) interpreter.Response {
	healthy := pointer.Bool(false)
	if workload.Status.ReadyReplicas == *workload.Spec.Replicas {
		healthy = pointer.Bool(true)
	}

	res := interpreter.Succeeded("")
	res.Healthy = healthy
	return res
}

func (e *workloadInterpreter) responseWithExploreInterpretStatus(workload *workloadv1alpha1.Workload) interpreter.Response {
	status := workloadv1alpha1.WorkloadStatus{
		ReadyReplicas: workload.Status.ReadyReplicas,
	}

	marshaledBytes, err := json.Marshal(status)
	if err != nil {
		return interpreter.Errored(http.StatusInternalServerError, err)
	}

	res := interpreter.Succeeded("")
	res.RawStatus = &runtime.RawExtension{
		Raw:    marshaledBytes,
	}

	return res
}
