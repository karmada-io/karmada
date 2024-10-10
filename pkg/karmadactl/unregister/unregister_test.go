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

package unregister

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/karmadactl/register"
)

const agentNamespace = "karmada-system"

func TestCommandUnregisterOption_getSpecifiedKarmadaContext(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   string
	}{
		{
			name: "context specified",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: register.KarmadaAgentName, Namespace: agentNamespace},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Command: []string{"/bin/karmada-agent", "--karmada-context=karmada-apiserver"}}},
						},
					},
				},
			},
			want: "karmada-apiserver",
		},
		{
			name: "context not specified",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: register.KarmadaAgentName, Namespace: agentNamespace},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{Command: []string{"/bin/karmada-agent"}}},
						},
					},
				},
			},
			want: "",
		},
		{
			name:   "deploy not found",
			deploy: &appsv1.Deployment{},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &CommandUnregisterOption{
				Namespace: agentNamespace,
			}
			client := fake.NewSimpleClientset(tt.deploy)
			if got := j.getSpecifiedKarmadaContext(client); got != tt.want {
				t.Errorf("getSpecifiedKarmadaContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
