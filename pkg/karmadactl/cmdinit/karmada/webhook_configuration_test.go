package karmada

import (
	"encoding/base64"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func Test_createOrUpdateValidatingWebhookConfiguration(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := validatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	if cfg == "" {
		t.Errorf("validatingConfig() return = %v, want yaml config", cfg)
	}
	if err := createOrUpdateValidatingWebhookConfiguration(client, cfg); err != nil {
		t.Errorf("createOrUpdateValidatingWebhookConfiguration() return = %v, want no error", err)
	}
}

func Test_createOrUpdateMutatingWebhookConfiguration(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := mutatingConfig(base64.StdEncoding.EncodeToString([]byte("foo")), "bar")
	if cfg == "" {
		t.Errorf("mutatingConfig() return = %v, want yaml config", cfg)
	}
	if err := createOrUpdateMutatingWebhookConfiguration(client, cfg); err != nil {
		t.Errorf("createOrUpdateMutatingWebhookConfiguration() return = %v, want no error", err)
	}
}
