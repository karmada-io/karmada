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
	"encoding/json"
	"net/http"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	schedulingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/scheduling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func makeRequest(t *testing.T, operation admissionv1.Operation, tq *schedulingv1alpha1.TenantQueue) admission.Request {
	t.Helper()
	raw, err := json.Marshal(tq)
	if err != nil {
		t.Fatalf("failed to marshal TenantQueue: %v", err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: operation,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

func newTenantQueue(namespace, name string, strategy schedulingv1alpha1.QueueingStrategy) *schedulingv1alpha1.TenantQueue {
	return &schedulingv1alpha1.TenantQueue{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schedulingv1alpha1.SchemeGroupVersion.String(),
			Kind:       schedulingv1alpha1.ResourceKindTenantQueue,
		},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       schedulingv1alpha1.TenantQueueSpec{QueueingStrategy: strategy},
	}
}

func TestValidatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name        string
		operation   admissionv1.Operation
		tenantQueue *schedulingv1alpha1.TenantQueue
		wantAllowed bool
	}{
		{
			name:        "singleton name is allowed on create",
			operation:   admissionv1.Create,
			tenantQueue: newTenantQueue("ns1", schedulingv1alpha1.TenantQueueSingletonName, schedulingv1alpha1.BestEffortFIFO),
			wantAllowed: true,
		},
		{
			name:        "singleton name is allowed on update",
			operation:   admissionv1.Update,
			tenantQueue: newTenantQueue("ns1", schedulingv1alpha1.TenantQueueSingletonName, schedulingv1alpha1.StrictFIFO),
			wantAllowed: true,
		},
		{
			name:        "any other name is denied",
			operation:   admissionv1.Create,
			tenantQueue: newTenantQueue("ns1", "my-queue", schedulingv1alpha1.BestEffortFIFO),
			wantAllowed: false,
		},
		{
			name:        "name that only differs in case is denied",
			operation:   admissionv1.Create,
			tenantQueue: newTenantQueue("ns1", "Queue", schedulingv1alpha1.BestEffortFIFO),
			wantAllowed: false,
		},
	}

	v := &ValidatingAdmission{Decoder: admission.NewDecoder(gclient.NewSchema())}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.Handle(context.Background(), makeRequest(t, tt.operation, tt.tenantQueue))
			if got.Allowed != tt.wantAllowed {
				t.Errorf("Handle() allowed = %v, want %v (message: %q)", got.Allowed, tt.wantAllowed, got.Result.Message)
			}
		})
	}
}

func TestValidatingAdmission_HandleUndecodableObject(t *testing.T) {
	v := &ValidatingAdmission{Decoder: admission.NewDecoder(gclient.NewSchema())}
	got := v.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte("not json")},
		},
	})
	if got.Allowed {
		t.Error("Handle() allowed an undecodable object")
	}
	if got.Result.Code != http.StatusBadRequest {
		t.Errorf("Handle() code = %d, want %d", got.Result.Code, http.StatusBadRequest)
	}
}
