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

package webhook

import (
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Test_resolveCluster(t *testing.T) {
	type args struct {
		svc  *corev1.Service
		port int32
	}
	tests := []struct {
		name    string
		args    args
		want    *url.URL
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "ClusterIP service without expect port, can not be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeClusterIP,
						ClusterIP: "10.10.10.10",
						Ports: []corev1.ServicePort{
							{Port: 1234, TargetPort: intstr.FromInt32(1234)},
						}}},
				port: 443,
			},
			wantErr: assert.Error,
		},
		{
			name: "ClusterIP service, can be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeClusterIP,
						ClusterIP: "10.10.10.10",
						Ports: []corev1.ServicePort{
							{Name: "https", Port: 443, TargetPort: intstr.FromInt32(1443)},
							{Port: 1234, TargetPort: intstr.FromInt32(1234)},
						}}},
				port: 443,
			},
			want: &url.URL{
				Scheme: "https",
				Host:   net.JoinHostPort("10.10.10.10", "443"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "headless service, can not be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeClusterIP,
						ClusterIP: corev1.ClusterIPNone,
					}},
				port: 443,
			},
			wantErr: assert.Error,
		},
		{
			name: "LoadBalancer service, can be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeLoadBalancer,
						ClusterIP: "10.10.10.10",
						Ports: []corev1.ServicePort{
							{Name: "https", Port: 443, TargetPort: intstr.FromInt32(1443)},
							{Port: 1234, TargetPort: intstr.FromInt32(1234)},
						}}},
				port: 443,
			},
			want: &url.URL{
				Scheme: "https",
				Host:   net.JoinHostPort("10.10.10.10", "443"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "NodePort service, can be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeLoadBalancer,
						ClusterIP: "10.10.10.10",
						Ports: []corev1.ServicePort{
							{Name: "https", Port: 443, TargetPort: intstr.FromInt32(1443)},
							{Port: 1234, TargetPort: intstr.FromInt32(1234)},
						}}},
				port: 443,
			},
			want: &url.URL{
				Scheme: "https",
				Host:   net.JoinHostPort("10.10.10.10", "443"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "ExternalName service, can be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type:         corev1.ServiceTypeExternalName,
						ExternalName: "foo.bar.com",
					}},
				port: 443,
			},
			want: &url.URL{
				Scheme: "https",
				Host:   net.JoinHostPort("foo.bar.com", "443"),
			},
			wantErr: assert.NoError,
		},
		{
			name: "unsupported type service, can not be resolved",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Namespace: "one", Name: "alfa"},
					Spec: corev1.ServiceSpec{
						Type: "unsupported service",
					}},
				port: 443,
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCluster(tt.args.svc, tt.args.port)
			if !tt.wantErr(t, err, fmt.Sprintf("resolveCluster(%v, %v)", tt.args.svc, tt.args.port)) {
				return
			}
			assert.Equalf(t, tt.want, got, "resolveCluster(%v, %v)", tt.args.svc, tt.args.port)
		})
	}
}
