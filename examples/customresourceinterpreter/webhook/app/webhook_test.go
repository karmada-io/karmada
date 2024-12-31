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

package app

import (
	"context"
	"testing"
)

func TestNewWebhookCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewWebhookCommand(ctx)

	if cmd == nil {
		t.Fatal("NewWebhookCommand returned nil")
	}

	if cmd.Use != "karmada-interpreter-webhook-example" {
		t.Errorf("Expected command use to be 'karmada-interpreter-webhook-example', got %s", cmd.Use)
	}

	if cmd.Run == nil {
		t.Error("Expected Run function to be set")
	}

	flags := cmd.Flags()
	expectedFlags := []string{"bind-address", "cert-dir", "secure-port"}
	for _, flag := range expectedFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("Expected flag %s to be set", flag)
		}
	}
}
