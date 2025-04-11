/*
Copyright 2025 The Karmada Authors.

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

package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/utils/ptr"
)

func TestDedupeAndSortIngressLoadBalancerIngress(t *testing.T) {
	type args struct {
		ingresses []networkingv1.IngressLoadBalancerIngress
	}
	tests := []struct {
		name string
		args args
		want []networkingv1.IngressLoadBalancerIngress
	}{
		{
			name: "sort hostname",
			args: args{
				ingresses: []networkingv1.IngressLoadBalancerIngress{
					{Hostname: "hostname-2"},
					{Hostname: "hostname-1"},
				},
			},
			want: []networkingv1.IngressLoadBalancerIngress{
				{Hostname: "hostname-1"},
				{Hostname: "hostname-2"},
			},
		},
		{
			name: "sort ip",
			args: args{
				ingresses: []networkingv1.IngressLoadBalancerIngress{
					{IP: "2.2.2.2"},
					{IP: "1.1.1.1"},
				},
			},
			want: []networkingv1.IngressLoadBalancerIngress{
				{IP: "1.1.1.1"},
				{IP: "2.2.2.2"},
			},
		},
		{
			name: "hostname should in front of ip",
			args: args{
				ingresses: []networkingv1.IngressLoadBalancerIngress{
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
				},
			},
			want: []networkingv1.IngressLoadBalancerIngress{
				{Hostname: "hostname-1"},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "merge hostname and ip",
			args: args{
				ingresses: []networkingv1.IngressLoadBalancerIngress{
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
				},
			},
			want: []networkingv1.IngressLoadBalancerIngress{
				{Hostname: "hostname-1"},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "merge and sort ports",
			args: args{
				ingresses: []networkingv1.IngressLoadBalancerIngress{
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 80, Protocol: "TCP"},
					}},
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 81, Protocol: "TCP"},
					}},
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 80, Protocol: "TCP"},
					}},
				},
			},
			want: []networkingv1.IngressLoadBalancerIngress{
				{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
					{Port: 80, Protocol: "TCP"},
					{Port: 81, Protocol: "TCP"},
				}},
			},
		},
		{
			name: "merge and sort errors",
			args: args{
				ingresses: []networkingv1.IngressLoadBalancerIngress{
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 80, Protocol: "TCP", Error: ptr.To("error-1")},
					}},
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 80, Protocol: "TCP", Error: ptr.To("error-1")},
					}},
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 80, Protocol: "TCP", Error: ptr.To("error-2")},
					}},
					{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
						{Port: 80, Protocol: "TCP"},
					}},
				},
			},
			want: []networkingv1.IngressLoadBalancerIngress{
				{Hostname: "hostname-1", Ports: []networkingv1.IngressPortStatus{
					{Port: 80, Protocol: "TCP"},
					{Port: 80, Protocol: "TCP", Error: ptr.To("error-1")},
					{Port: 80, Protocol: "TCP", Error: ptr.To("error-2")},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 1000; i++ { // eliminate randomness in sorting
				pass := assert.Equalf(t, tt.want, DedupeAndSortIngressLoadBalancerIngress(tt.args.ingresses), "DedupeAndSortIngressLoadBalancerIngress(%v)", tt.args.ingresses)
				if !pass {
					break
				}
			}
		})
	}
}
