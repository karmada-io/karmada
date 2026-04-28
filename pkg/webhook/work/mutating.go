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

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native/prune"
	"github.com/karmada-io/karmada/pkg/util"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
// This function could affect the informer cache of controller-manager.
// In controller-manager, redundant updates to Work are avoided as much as possible, and changes to informer cache will affect this behavior.
// Therefore, if someone modify this function, please ensure that the relevant logic in controller-manager also be updated.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	work := &workv1alpha1.Work{}

	err := a.Decoder.Decode(req, work)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Mutating the work(%s/%s) for request: %s", work.Namespace, work.Name, req.Operation)

	if util.GetLabelValue(work.Labels, workv1alpha2.WorkPermanentIDLabel) == "" {
		util.MergeLabel(work, workv1alpha2.WorkPermanentIDLabel, uuid.New().String())
	}

	var manifests []workv1alpha1.Manifest

	for _, manifest := range work.Spec.Workload.Manifests {
		workloadObj := &unstructured.Unstructured{}
		err := json.Unmarshal(manifest.Raw, workloadObj)
		if err != nil {
			klog.Errorf("Failed to unmarshal the work(%s/%s) manifest to Unstructured, err: %v", work.Namespace, work.Name, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		err = prune.RemoveIrrelevantFields(workloadObj, prune.RemoveJobTTLSeconds)
		if err != nil {
			klog.Errorf("Failed to remove irrelevant fields for the work(%s/%s), err: %v", work.Namespace, work.Name, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		util.SetLabelsAndAnnotationsForWorkload(workloadObj, work)

		workloadJSON, err := workloadObj.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal workload of the work(%s/%s), err: %s", work.Namespace, work.Name, err)
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
