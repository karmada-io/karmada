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

package work

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native/prune"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	work := &workv1alpha1.Work{}

	err := a.Decoder.Decode(req, work)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Mutating work(%s) for request: %s", work.Name, req.Operation)

	var manifests []workv1alpha1.Manifest

	for _, manifest := range work.Spec.Workload.Manifests {
		workloadObj := &unstructured.Unstructured{}
		err := json.Unmarshal(manifest.Raw, workloadObj)
		if err != nil {
			klog.Errorf("Failed to unmarshal work(%s) manifest to Unstructured", work.Name)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		err = prune.RemoveIrrelevantField(workloadObj, prune.RemoveJobTTLSeconds)
		if err != nil {
			klog.Errorf("Failed to remove irrelevant field for work(%s): %v", work.Name, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		workloadJSON, err := workloadObj.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal workload of work(%s)", work.Name)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		manifests = append(manifests, workv1alpha1.Manifest{RawExtension: runtime.RawExtension{Raw: workloadJSON}})
	}

	work.Spec.Workload.Manifests = manifests
	marshaledBytes, err := json.Marshal(work)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
