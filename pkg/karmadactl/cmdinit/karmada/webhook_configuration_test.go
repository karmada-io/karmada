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
