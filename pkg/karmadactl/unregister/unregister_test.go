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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/karmadactl/register"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	clusterNamespace      = "default"
	namespace             = "default"
	clusterName           = "member_test"
	agentConfigSecretName = "karmada-agent-config" //nolint:gosec
	agentConfigKeyName    = "karmada.config"
	agentSecretVolumeName = "karmada-config" //nolint:gosec
)

func TestCommandUnregisterOption_Complete_Validate(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		clusterKubeConfig string
		wait              time.Duration
		wantCompleteErr   bool
		wantValidateErr   bool
	}{
		{
			name:            "args more than one",
			args:            []string{"member1", "member2"},
			wantCompleteErr: false,
			wantValidateErr: true,
		},
		{
			name:            "invalid cluster name",
			args:            []string{"member.1"},
			wantCompleteErr: false,
			wantValidateErr: true,
		},
		{
			name:              "empty clusterKubeConfig",
			args:              []string{"member1"},
			clusterKubeConfig: "",
			wantCompleteErr:   false,
			wantValidateErr:   true,
		},
		{
			name:              "negative wait time",
			args:              []string{"member1"},
			clusterKubeConfig: "./kube/config",
			wait:              -1,
			wantCompleteErr:   false,
			wantValidateErr:   true,
		},
		{
			name:              "normal case",
			args:              []string{"member1"},
			clusterKubeConfig: "./kube/config",
			wait:              60 * time.Second,
			wantCompleteErr:   false,
			wantValidateErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &CommandUnregisterOption{
				ClusterKubeConfig: tt.clusterKubeConfig,
				Wait:              tt.wait,
			}

			err := j.Complete(tt.args)
			if (err == nil && tt.wantCompleteErr) || (err != nil && !tt.wantCompleteErr) {
				t.Errorf("Complete() error = %v, wantCompleteErr %v", err, tt.wantCompleteErr)
			}

			err = j.Validate(tt.args)
			if (err == nil && tt.wantValidateErr) || (err != nil && !tt.wantValidateErr) {
				t.Errorf("Validate() error = %v, wantValidateErr %v", err, tt.wantValidateErr)
			}
		})
	}
}

func TestCommandUnregisterOption_getKarmadaAgentConfig(t *testing.T) {
	agentConfig := register.CreateBasic("http://127.0.0.1:5443", clusterName, "test", nil)
	agentConfigBytes, _ := clientcmd.Write(*agentConfig)
	agentConfigSecret := createSecret(agentConfigSecretName, namespace, agentConfigKeyName, agentConfigBytes)

	tests := []struct {
		name              string
		mountPath         string
		karmadaConfigPath string
		clusterResources  []runtime.Object
		wantErr           bool
	}{
		{
			name:              "common case",
			mountPath:         "/etc/karmada/config",
			karmadaConfigPath: "/etc/karmada/config/karmada.config",
			clusterResources:  []runtime.Object{agentConfigSecret},
			wantErr:           false,
		},
		{
			name:              "mount path end up with a extra / symbol",
			mountPath:         "/etc/karmada/config/",
			karmadaConfigPath: "/etc/karmada/config/karmada.config",
			clusterResources:  []runtime.Object{agentConfigSecret},
			wantErr:           false,
		},
		{
			name:              "agent config secret not found",
			mountPath:         "/etc/karmada/config",
			karmadaConfigPath: "/etc/karmada/config/karmada.config",
			wantErr:           true,
		},
		{
			name:              "agent config secret exist but has a invalid key name",
			mountPath:         "/etc/karmada/config",
			karmadaConfigPath: "/etc/karmada/config/karmada-config",
			clusterResources:  []runtime.Object{agentConfigSecret},
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &CommandUnregisterOption{
				Namespace:           namespace,
				MemberClusterClient: fake.NewSimpleClientset(tt.clusterResources...),
			}
			agent := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: names.KarmadaAgentComponentName, Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Command: []string{
									"/bin/karmada-agent",
									"--karmada-kubeconfig=" + tt.karmadaConfigPath,
								},
								VolumeMounts: []corev1.VolumeMount{{
									Name:      agentSecretVolumeName,
									MountPath: tt.mountPath,
								}},
							}},
							Volumes: []corev1.Volume{{
								Name: agentSecretVolumeName,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{SecretName: agentConfigSecretName},
								},
							}},
						},
					},
				},
			}
			_, err := j.getKarmadaAgentConfig(agent)
			if (err == nil && tt.wantErr) || (err != nil && !tt.wantErr) {
				t.Errorf("getSpecifiedKarmadaContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandUnregisterOption_RunUnregisterCluster(t *testing.T) {
	tests := []struct {
		name             string
		clusterObject    []runtime.Object
		clusterResources []runtime.Object
		wantErr          bool
	}{
		{
			name:             "cluster object not exist",
			clusterObject:    []runtime.Object{},
			clusterResources: []runtime.Object{},
			wantErr:          true,
		},
		{
			name: "cluster exist, but cluster resources not found",
			clusterObject: []runtime.Object{&clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName},
				Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Pull}},
			},
			clusterResources: []runtime.Object{},
			wantErr:          false,
		},
		{
			name: "push mode member cluster",
			clusterObject: []runtime.Object{&clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName},
				Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push}},
			},
			clusterResources: []runtime.Object{},
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &CommandUnregisterOption{
				ClusterName:      clusterName,
				Namespace:        namespace,
				ClusterNamespace: clusterNamespace,
				Wait:             60 * time.Second,
			}
			j.ControlPlaneClient = fakekarmadaclient.NewSimpleClientset(tt.clusterObject...)
			j.MemberClusterClient = fake.NewSimpleClientset(tt.clusterResources...)
			j.rbacResources = register.GenerateRBACResources(j.ClusterName, j.ClusterNamespace)
			err := j.RunUnregisterCluster()
			if (err == nil && tt.wantErr) || (err != nil && !tt.wantErr) {
				t.Errorf("RunUnregisterCluster() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func createSecret(secretName, secretNamespace, keyName string, value []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: secretNamespace},
		Data: map[string][]byte{
			keyName: value,
		},
	}
}
