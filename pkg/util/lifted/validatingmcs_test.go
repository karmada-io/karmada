/*
Copyright 2024 The Kubernetes Authors.

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

package lifted

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateLoadBalancerStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     *corev1.LoadBalancerStatus
		wantErrors int
	}{
		{
			name: "valid IP",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "192.168.1.1"},
				},
			},
			wantErrors: 0,
		},
		{
			name: "valid hostname",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "example.com"},
				},
			},
			wantErrors: 0,
		},
		{
			name: "invalid IP",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "300.300.300.300"},
				},
			},
			wantErrors: 1,
		},
		{
			name: "invalid hostname",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "invalid_hostname"},
				},
			},
			wantErrors: 1,
		},
		{
			name: "IP in hostname field",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "192.168.1.1"},
				},
			},
			wantErrors: 1,
		},
		{
			name: "multiple valid entries",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "192.168.1.1"},
					{Hostname: "example.com"},
				},
			},
			wantErrors: 0,
		},
		{
			name: "multiple entries with errors",
			status: &corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "300.300.300.300"},
					{Hostname: "invalid_hostname"},
					{Hostname: "192.168.1.1"},
				},
			},
			wantErrors: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateLoadBalancerStatus(tt.status, field.NewPath("status"))
			assert.Len(t, errors, tt.wantErrors)

			if tt.wantErrors == 0 {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
			}
		})
	}
}
