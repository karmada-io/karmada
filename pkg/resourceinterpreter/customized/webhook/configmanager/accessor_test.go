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

package configmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/utils/ptr"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestResourceExploringAccessor_Getters(t *testing.T) {
	timeoutSeconds := int32(10)
	testCases := []struct {
		name              string
		uid               string
		configurationName string
		webhook           *configv1alpha1.ResourceInterpreterWebhook
	}{
		{
			name:              "basic webhook configuration",
			uid:               "test-uid-1",
			configurationName: "test-config-1",
			webhook: &configv1alpha1.ResourceInterpreterWebhook{
				Name: "test-webhook",
			},
		},
		{
			name:              "empty webhook configuration",
			uid:               "",
			configurationName: "",
			webhook:           &configv1alpha1.ResourceInterpreterWebhook{},
		},
		{
			name:              "complete webhook configuration",
			uid:               "test-uid-2",
			configurationName: "test-config-2",
			webhook: &configv1alpha1.ResourceInterpreterWebhook{
				Name: "test-webhook",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("https://test-webhook.example.com"),
					Service: &admissionregistrationv1.ServiceReference{
						Name:      "test-service",
						Namespace: "test-namespace",
						Port:      ptr.To(int32(443)),
						Path:      ptr.To("/validate"),
					},
					CABundle: []byte("test-ca-bundle"),
				},
				Rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.InterpreterOperation{"interpret"},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Kinds:       []string{"Pod"},
						},
					},
				},
				TimeoutSeconds:             &timeoutSeconds,
				InterpreterContextVersions: []string{"v1", "v2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accessor := NewResourceExploringAccessor(tc.uid, tc.configurationName, tc.webhook)

			// Test basic fields
			assert.Equal(t, tc.uid, accessor.GetUID(), "uid should match")
			assert.Equal(t, tc.configurationName, accessor.GetConfigurationName(), "configuration name should match")
			assert.Equal(t, tc.webhook.Name, accessor.GetName(), "webhook name should match")

			// Test additional fields
			assert.Equal(t, tc.webhook.ClientConfig, accessor.GetClientConfig(), "client config should match")
			assert.Equal(t, tc.webhook.Rules, accessor.GetRules(), "rules should match")
			assert.Equal(t, tc.webhook.TimeoutSeconds, accessor.GetTimeoutSeconds(), "timeout seconds should match")
			assert.Equal(t, tc.webhook.InterpreterContextVersions, accessor.GetInterpreterContextVersions(), "interpreter context versions should match")
		})
	}
}

func TestHookClientConfigForWebhook(t *testing.T) {
	testCases := []struct {
		name         string
		hookName     string
		clientConfig admissionregistrationv1.WebhookClientConfig
		expected     webhookutil.ClientConfig
	}{
		{
			name:     "URL configuration",
			hookName: "test-webhook",
			clientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      ptr.To("https://test-webhook.example.com"),
				CABundle: []byte("test-ca-bundle"),
			},
			expected: webhookutil.ClientConfig{
				Name:     "test-webhook",
				URL:      "https://test-webhook.example.com",
				CABundle: []byte("test-ca-bundle"),
			},
		},
		{
			name:     "Service configuration with defaults",
			hookName: "test-webhook",
			clientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
				CABundle: []byte("test-ca-bundle"),
			},
			expected: webhookutil.ClientConfig{
				Name:     "test-webhook",
				CABundle: []byte("test-ca-bundle"),
				Service: &webhookutil.ClientConfigService{
					Name:      "test-service",
					Namespace: "test-namespace",
					Port:      443,
				},
			},
		},
		{
			name:     "Service configuration with custom values",
			hookName: "test-webhook",
			clientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "test-service",
					Namespace: "test-namespace",
					Port:      ptr.To(int32(8443)),
					Path:      ptr.To("/validate"),
				},
				CABundle: []byte("test-ca-bundle"),
			},
			expected: webhookutil.ClientConfig{
				Name:     "test-webhook",
				CABundle: []byte("test-ca-bundle"),
				Service: &webhookutil.ClientConfigService{
					Name:      "test-service",
					Namespace: "test-namespace",
					Port:      8443,
					Path:      "/validate",
				},
			},
		},
		{
			name:     "Empty service configuration",
			hookName: "test-webhook",
			clientConfig: admissionregistrationv1.WebhookClientConfig{
				Service:  &admissionregistrationv1.ServiceReference{},
				CABundle: []byte("test-ca-bundle"),
			},
			expected: webhookutil.ClientConfig{
				Name:     "test-webhook",
				CABundle: []byte("test-ca-bundle"),
				Service: &webhookutil.ClientConfigService{
					Port: 443,
				},
			},
		},
		{
			name:     "Nil service and URL configuration",
			hookName: "test-webhook",
			clientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: []byte("test-ca-bundle"),
			},
			expected: webhookutil.ClientConfig{
				Name:     "test-webhook",
				CABundle: []byte("test-ca-bundle"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hookClientConfigForWebhook(tc.hookName, tc.clientConfig)
			assert.Equal(t, tc.expected, result, "webhook client config should match expected configuration")
		})
	}
}
