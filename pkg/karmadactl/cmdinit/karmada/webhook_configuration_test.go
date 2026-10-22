/*
Copyright 2022 The Karmada Authors.

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

package karmada

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

func Test_createOrUpdateValidatingWebhookConfiguration(t *testing.T) {
	client := fake.NewClientset()
	cfg := validatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	if cfg == "" {
		t.Errorf("validatingConfig() return = %v, want yaml config", cfg)
	}
	if err := createOrUpdateValidatingWebhookConfiguration(client, cfg); err != nil {
		t.Errorf("createOrUpdateValidatingWebhookConfiguration() return = %v, want no error", err)
	}
}

func Test_createOrUpdateMutatingWebhookConfiguration(t *testing.T) {
	client := fake.NewClientset()
	cfg := mutatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	if cfg == "" {
		t.Errorf("mutatingConfig() return = %v, want yaml config", cfg)
	}
	if err := createOrUpdateMutatingWebhookConfiguration(client, cfg); err != nil {
		t.Errorf("createOrUpdateMutatingWebhookConfiguration() return = %v, want no error", err)
	}
}

func TestValidatingConfigClusterTaintPolicyWebhook(t *testing.T) {
	cfg := validatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	obj := admissionregistrationv1.ValidatingWebhookConfiguration{}
	if err := json.Unmarshal(utils.StaticYamlToJSONByte(cfg), &obj); err != nil {
		t.Fatalf("failed to decode validatingConfig() output: %v", err)
	}

	var got *admissionregistrationv1.ValidatingWebhook
	for i := range obj.Webhooks {
		if obj.Webhooks[i].Name == "clustertaintpolicy.karmada.io" {
			got = &obj.Webhooks[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("validatingConfig() is missing the clustertaintpolicy.karmada.io webhook")
	}

	want := &admissionregistrationv1.ValidatingWebhook{
		Name:         "clustertaintpolicy.karmada.io",
		ClientConfig: admissionregistrationv1.WebhookClientConfig{URL: new("https://karmada-webhook.bar.svc:443/validate-clustertaintpolicy"), CABundle: []byte("foo")},
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"policy.karmada.io"},
				APIVersions: []string{"*"},
				Resources:   []string{"clustertaintpolicies"},
				Scope:       new(admissionregistrationv1.ClusterScope),
			},
		}},
		FailurePolicy:           new(admissionregistrationv1.Fail),
		SideEffects:             new(admissionregistrationv1.SideEffectClassNone),
		TimeoutSeconds:          new(int32(3)),
		AdmissionReviewVersions: []string{"v1"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("clustertaintpolicy.karmada.io webhook mismatch (-want +got):\n%s", diff)
	}
}
