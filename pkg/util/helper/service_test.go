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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestDedupeAndSortServiceLoadBalancerIngress(t *testing.T) {
	type args struct {
		ingresses []corev1.LoadBalancerIngress
	}
	tests := []struct {
		name string
		args args
		want []corev1.LoadBalancerIngress
	}{
		{
			name: "sort hostname",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{Hostname: "hostname-2"},
					{Hostname: "hostname-1"},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1"},
				{Hostname: "hostname-2"},
			},
		},
		{
			name: "sort ip",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{IP: "2.2.2.2"},
					{IP: "1.1.1.1"},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1"},
				{IP: "2.2.2.2"},
			},
		},
		{
			name: "hostname should in front of ip",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1"},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "sort ipMode",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeProxy)},
					{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeProxy)},
				{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
			},
		},
		{
			name: "merge ipMode",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
					{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
			},
		},
		{
			name: "merge hostname and ip",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
					{IP: "1.1.1.1"},
					{Hostname: "hostname-1"},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1"},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "merge and sort ports",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 80, Protocol: "TCP"},
					}},
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 81, Protocol: "TCP"},
					}},
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 80, Protocol: "TCP"},
					}},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", Ports: []corev1.PortStatus{
					{Port: 80, Protocol: "TCP"},
					{Port: 81, Protocol: "TCP"},
				}},
			},
		},
		{
			name: "merge and sort errors",
			args: args{
				ingresses: []corev1.LoadBalancerIngress{
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 80, Protocol: "TCP", Error: new("error-1")},
					}},
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 80, Protocol: "TCP", Error: new("error-1")},
					}},
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 80, Protocol: "TCP", Error: new("error-2")},
					}},
					{Hostname: "hostname-1", Ports: []corev1.PortStatus{
						{Port: 80, Protocol: "TCP"},
					}},
				},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", Ports: []corev1.PortStatus{
					{Port: 80, Protocol: "TCP"},
					{Port: 80, Protocol: "TCP", Error: new("error-1")},
					{Port: 80, Protocol: "TCP", Error: new("error-2")},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 1000 { // eliminate randomness in sorting
				pass := assert.Equalf(t, tt.want, DedupeAndSortServiceLoadBalancerIngress(tt.args.ingresses), "DedupeAndSortServiceLoadBalancerIngress(%v)", tt.args.ingresses)
				if !pass {
					break
				}
			}
		})
	}
}

func TestSortServicePortStatus(t *testing.T) {
	tests := []struct {
		name  string
		ports []corev1.PortStatus
		want  []corev1.PortStatus
	}{
		{
			name: "sort by port number",
			ports: []corev1.PortStatus{
				{Port: 82, Protocol: "TCP"},
				{Port: 80, Protocol: "TCP"},
				{Port: 81, Protocol: "TCP"},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: "TCP"},
				{Port: 81, Protocol: "TCP"},
				{Port: 82, Protocol: "TCP"},
			},
		},
		{
			name: "same port, sort by protocol",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: "UDP"},
				{Port: 80, Protocol: "SCTP"},
				{Port: 80, Protocol: "TCP"},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: "SCTP"},
				{Port: 80, Protocol: "TCP"},
				{Port: 80, Protocol: "UDP"},
			},
		},
		{
			name: "same port and protocol, nil error sorts first",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: "TCP", Error: new("some-error")},
				{Port: 80, Protocol: "TCP"},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: "TCP"},
				{Port: 80, Protocol: "TCP", Error: new("some-error")},
			},
		},
		{
			name: "same port and protocol, sort by error string",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: "TCP", Error: new("error-2")},
				{Port: 80, Protocol: "TCP", Error: new("error-1")},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: "TCP", Error: new("error-1")},
				{Port: 80, Protocol: "TCP", Error: new("error-2")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 1000 { // eliminate randomness in sorting
				sortServicePortStatus(tt.ports)
				pass := assert.Equalf(t, tt.want, tt.ports, "sortServicePortStatus(%v)", tt.ports)
				if !pass {
					break
				}
			}
		})
	}
}

func TestSortServiceLoadBalancerIngress(t *testing.T) {
	tests := []struct {
		name      string
		ingresses []corev1.LoadBalancerIngress
		want      []corev1.LoadBalancerIngress
	}{
		{
			name: "non-empty hostname sorts before empty hostname",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1"},
				{Hostname: "hostname-1"},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1"},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "sort hostnames alphabetically",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-c"},
				{Hostname: "hostname-a"},
				{Hostname: "hostname-b"},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-a"},
				{Hostname: "hostname-b"},
				{Hostname: "hostname-c"},
			},
		},
		{
			name: "same hostname, sort by IP",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", IP: "3.3.3.3"},
				{Hostname: "hostname-1", IP: "1.1.1.1"},
				{Hostname: "hostname-1", IP: "2.2.2.2"},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", IP: "1.1.1.1"},
				{Hostname: "hostname-1", IP: "2.2.2.2"},
				{Hostname: "hostname-1", IP: "3.3.3.3"},
			},
		},
		{
			name: "same hostname and IP, non-nil IPMode sorts before nil",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", IP: "1.1.1.1"},
				{Hostname: "hostname-1", IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
				{Hostname: "hostname-1", IP: "1.1.1.1"},
			},
		},
		{
			name: "same hostname and IP, sort by IPMode value",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
				{Hostname: "hostname-1", IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeProxy)},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "hostname-1", IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeProxy)},
				{Hostname: "hostname-1", IP: "1.1.1.1", IPMode: ptr.To(corev1.LoadBalancerIPModeVIP)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 1000 { // eliminate randomness in sorting
				sortServiceLoadBalancerIngress(tt.ingresses)
				pass := assert.Equalf(t, tt.want, tt.ingresses, "sortServiceLoadBalancerIngress(%v)", tt.ingresses)
				if !pass {
					break
				}
			}
		})
	}
}
