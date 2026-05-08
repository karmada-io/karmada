/*
Copyright The Karmada Authors.

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

package tenantqueue

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	schedulingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/scheduling/v1alpha1"
)

// ValidatingAdmission validates TenantQueue object when creating/updating.
type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	tq := &schedulingv1alpha1.TenantQueue{}

	err := v.Decoder.Decode(req, tq)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating TenantQueue(%s) for request: %s", klog.KObj(tq).String(), req.Operation)

	if tq.Name != schedulingv1alpha1.TenantQueueSingletonName {
		return admission.Denied(fmt.Sprintf("TenantQueue must be named %q", schedulingv1alpha1.TenantQueueSingletonName))
	}

	return admission.Allowed("")
}
