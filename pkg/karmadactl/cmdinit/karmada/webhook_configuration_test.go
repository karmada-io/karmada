package karmada

import (
	"encoding/base64"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func Test_createValidatingWebhookConfiguration(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := validatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	if cfg == "" {
		t.Errorf("validatingConfig() return = %v, want yaml config", cfg)
	}
	if err := createValidatingWebhookConfiguration(client, cfg); err != nil {
		t.Errorf("createValidatingWebhookConfiguration() return = %v, want no error", err)
	}
}

func Test_createMutatingWebhookConfiguration(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := mutatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	if cfg == "" {
		t.Errorf("mutatingConfig() return = %v, want yaml config", cfg)
	}
	if err := createMutatingWebhookConfiguration(client, cfg); err != nil {
		t.Errorf("createMutatingWebhookConfiguration() return = %v, want no error", err)
	}
}
