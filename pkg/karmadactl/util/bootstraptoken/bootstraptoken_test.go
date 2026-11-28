/*
Copyright 2026 The Karmada Authors.

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

package bootstraptoken

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
)

func TestGenerateRandomBootstrapTokenTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     *metav1.Duration
		wantErr bool
	}{
		{
			name:    "negative TTL is rejected",
			ttl:     &metav1.Duration{Duration: -time.Hour},
			wantErr: true,
		},
		{
			name: "zero TTL never expires and is accepted",
			ttl:  &metav1.Duration{Duration: 0},
		},
		{
			name: "positive TTL is accepted",
			ttl:  &metav1.Duration{Duration: 24 * time.Hour},
		},
		{
			name: "unset TTL is accepted",
			ttl:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateRandomBootstrapToken(tt.ttl, "test", nil, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for TTL %v, got token %v", tt.ttl, token)
				}
				if token != nil {
					t.Fatalf("expected no token on error, got %v", token)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token == nil {
				t.Fatal("expected a token, got nil")
			}
		})
	}
}

func TestEncodeTokenSecretDataTTL(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	token := &BootstrapToken{
		Token: &Token{ID: "abcdef", Secret: "0123456789abcdef"},
		TTL:   &metav1.Duration{Duration: 0},
	}

	data := encodeTokenSecretData(token, now)
	if _, ok := data[bootstrapapi.BootstrapTokenExpirationKey]; ok {
		t.Fatal("a zero TTL must not set an expiration")
	}

	token.TTL = &metav1.Duration{Duration: 2 * time.Hour}
	data = encodeTokenSecretData(token, now)
	expiration, ok := data[bootstrapapi.BootstrapTokenExpirationKey]
	if !ok {
		t.Fatal("a positive TTL must set an expiration")
	}
	if want := now.Add(2 * time.Hour).Format(time.RFC3339); string(expiration) != want {
		t.Fatalf("expected expiration %q, got %q", want, string(expiration))
	}
}
