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
	errA := new(string)
	*errA = "error-a"
	errB := new(string)
	*errB = "error-b"

	tests := []struct {
		name  string
		ports []corev1.PortStatus
		want  []corev1.PortStatus
	}{
		{
			name: "sort by port number ascending",
			ports: []corev1.PortStatus{
				{Port: 443, Protocol: corev1.ProtocolTCP},
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 443, Protocol: corev1.ProtocolTCP},
			},
		},
		{
			name: "same port, sort by protocol",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolUDP},
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 80, Protocol: corev1.ProtocolUDP},
			},
		},
		{
			name: "same port and protocol, nil error sorts before non-nil",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP, Error: errA},
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP},
				{Port: 80, Protocol: corev1.ProtocolTCP, Error: errA},
			},
		},
		{
			name: "same port and protocol, both non-nil error sorts by error string",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP, Error: errB},
				{Port: 80, Protocol: corev1.ProtocolTCP, Error: errA},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP, Error: errA},
				{Port: 80, Protocol: corev1.ProtocolTCP, Error: errB},
			},
		},
		{
			name:  "empty slice",
			ports: []corev1.PortStatus{},
			want:  []corev1.PortStatus{},
		},
		{
			name: "single element unchanged",
			ports: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
			want: []corev1.PortStatus{
				{Port: 80, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortServicePortStatus(tt.ports)
			assert.Equal(t, tt.want, tt.ports)
		})
	}
}

func TestSortServiceLoadBalancerIngress(t *testing.T) {
	proxyMode := corev1.LoadBalancerIPModeProxy
	vipMode := corev1.LoadBalancerIPModeVIP

	tests := []struct {
		name      string
		ingresses []corev1.LoadBalancerIngress
		want      []corev1.LoadBalancerIngress
	}{
		{
			name: "sort by hostname ascending",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "beta.example.com"},
				{Hostname: "alpha.example.com"},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "alpha.example.com"},
				{Hostname: "beta.example.com"},
			},
		},
		{
			name: "hostname sorts before ip",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1"},
				{Hostname: "alpha.example.com"},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "alpha.example.com"},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "sort by ip ascending when no hostname",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "2.2.2.2"},
				{IP: "1.1.1.1"},
			},
			want: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1"},
				{IP: "2.2.2.2"},
			},
		},
		{
			name: "same hostname and ip, non-nil ipmode sorts before nil",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1"},
				{IP: "1.1.1.1", IPMode: &vipMode},
			},
			want: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1", IPMode: &vipMode},
				{IP: "1.1.1.1"},
			},
		},
		{
			name: "same hostname and ip, sort by ipmode string",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1", IPMode: &vipMode},
				{IP: "1.1.1.1", IPMode: &proxyMode},
			},
			want: []corev1.LoadBalancerIngress{
				{IP: "1.1.1.1", IPMode: &proxyMode},
				{IP: "1.1.1.1", IPMode: &vipMode},
			},
		},
		{
			name:      "empty slice",
			ingresses: []corev1.LoadBalancerIngress{},
			want:      []corev1.LoadBalancerIngress{},
		},
		{
			name: "single element unchanged",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "alpha.example.com"},
			},
			want: []corev1.LoadBalancerIngress{
				{Hostname: "alpha.example.com"},
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
