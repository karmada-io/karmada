/*
Copyright 2024 The Karmada Authors.

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

package webhookconfiguration

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"

	"github.com/karmada-io/karmada/operator/pkg/util"
)

func TestEnsureWebhookConfiguration(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeClient := fakeclientset.NewSimpleClientset()
	err := EnsureWebhookConfiguration(fakeClient, namespace, name, caBundle)
	if err != nil {
		t.Fatalf("failed to create karmada mutating webhook configuration and validation webhook configuration: %v", err)
	}

	// Ensure the expected action (mutating webhook configuration and validating webhook configuration creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, but got %d", len(actions))
	}
}

func TestMutatingWebhookConfiguration(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeClient := fakeclientset.NewSimpleClientset()
	err := mutatingWebhookConfiguration(fakeClient, namespace, name, caBundle)
	if err != nil {
		t.Fatalf("error creating the mutating webhook configuration: %v", err)
	}

	// Ensure the expected action (mutating webhook configuration creation) occurred on the fake clientset.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (MutatingWebhookConfiguration).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	// Validate the created MutatingWebhookConfiguration object.
	mutatingWebhookConfig := createAction.GetObject().(*admissionregistrationv1.MutatingWebhookConfiguration)
	serviceName := util.KarmadaWebhookName(name)
	for _, webhook := range mutatingWebhookConfig.Webhooks {
		clientConfigRootURL := fmt.Sprintf("https://%s.%s.svc:443", serviceName, namespace)
		if !strings.HasPrefix(*webhook.ClientConfig.URL, clientConfigRootURL) {
			t.Errorf("expected webhook client config url '%s' to start with '%s'", *webhook.ClientConfig.URL, clientConfigRootURL)
		}

		if string(webhook.ClientConfig.CABundle) != caTestData {
			t.Fatalf("expected webhook client config caBundle %s, but got %s", caTestData, string(webhook.ClientConfig.CABundle))
		}
	}
}

func TestValidatingWebhookConfiguration(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeClient := fakeclientset.NewSimpleClientset()
	err := validatingWebhookConfiguration(fakeClient, namespace, name, caBundle)
	if err != nil {
		t.Fatalf("error creating the mutating webhook configuration: %v", err)
	}

	// Ensure the expected action (validating webhook configuration creation) occurred on the fake clientset.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (ValidatingWebhookConfiguration).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	// Validate the created ValidatingWebhookConfiguration object.
	mutatingWebhookConfig := createAction.GetObject().(*admissionregistrationv1.ValidatingWebhookConfiguration)
	serviceName := util.KarmadaWebhookName(name)
	for _, webhook := range mutatingWebhookConfig.Webhooks {
		clientConfigRootURL := fmt.Sprintf("https://%s.%s.svc:443", serviceName, namespace)
		if !strings.HasPrefix(*webhook.ClientConfig.URL, clientConfigRootURL) {
			t.Errorf("expected webhook client config url '%s' to start with '%s'", *webhook.ClientConfig.URL, clientConfigRootURL)
		}

		if string(webhook.ClientConfig.CABundle) != caTestData {
			t.Fatalf("expected webhook client config caBundle %s, but got %s", caTestData, string(webhook.ClientConfig.CABundle))
		}
	}
}
