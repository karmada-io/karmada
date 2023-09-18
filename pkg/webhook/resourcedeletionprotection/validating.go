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

package resourcedeletionprotection

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// ValidatingAdmission validates resource templates to ensure those protected resources are not delectable.
type ValidatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Delete {
		// We only care about the Delete operation.
		return admission.Allowed("")
	}

	// Parse the uncertain type resource object
	obj := &unstructured.Unstructured{}
	if err := v.Decoder.DecodeRaw(req.OldObject, obj); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(2).Infof("Validating ResourceDeletionProtection for resource: Kind:%s Name:%s Namespace:%s", req.Kind.Kind, obj.GetName(), obj.GetNamespace())

	if value, ok := obj.GetLabels()[workv1alpha2.DeletionProtectionLabelKey]; ok {
		// In normal, requests will be processed here.
		// Only v1alpha2.DeletionProtectionAlways value will be denied
		if value == workv1alpha2.DeletionProtectionAlways {
			return admission.Denied(fmt.Sprintf("This resource is protected, please make sure to remove the label: %s", workv1alpha2.DeletionProtectionLabelKey))
		}
	}
	return admission.Allowed("")
}
