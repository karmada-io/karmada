/*
Copyright 2023 The Karmada Authors.

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

package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_retainK8sWorkloadReplicas(t *testing.T) {
	desiredNum, observedNum := int32(2), int32(4)
	observed, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &observedNum,
		},
	})
	desired, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				util.RetainReplicasLabel: util.RetainReplicasValue,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredNum,
		},
	})
	want, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				util.RetainReplicasLabel: util.RetainReplicasValue,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &observedNum,
		},
	})
	desired2, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredNum,
		},
	})
	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "deployment is control by hpa",
			args: args{
				desired:  desired,
				observed: observed,
			},
			want:    want,
			wantErr: false,
		},
		{
			name: "deployment is not control by hpa",
			args: args{
				desired:  desired2,
				observed: observed,
			},
			want:    desired2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainWorkloadReplicas(tt.args.desired, tt.args.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("retainWorkloadReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "retainDeploymentFields(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}

func Test_retainSecretServiceAccountToken(t *testing.T) {
	createSecret := func(secretType corev1.SecretType, uuid, key, value string) *unstructured.Unstructured {
		ret, _ := helper.ToUnstructured(&corev1.Secret{
			Type: secretType,
		})
		return ret
	}

	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want *unstructured.Unstructured
	}{
		{
			name: "secret data and uid are retained for type service-account-token",
			args: args{
				desired:  createSecret(corev1.SecretTypeServiceAccountToken, "111", corev1.ServiceAccountTokenKey, "desired-token"),
				observed: createSecret(corev1.SecretTypeServiceAccountToken, "999", corev1.ServiceAccountTokenKey, "observed-token"),
			},
			want: createSecret(corev1.SecretTypeServiceAccountToken, "999", corev1.ServiceAccountTokenKey, "observed-token"),
		},
		{
			name: "ignores missing uid and data for type service-account-token",
			args: args{
				desired:  &unstructured.Unstructured{Object: map[string]interface{}{"type": string(corev1.SecretTypeServiceAccountToken)}},
				observed: &unstructured.Unstructured{Object: map[string]interface{}{"type": string(corev1.SecretTypeServiceAccountToken)}},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{"type": string(corev1.SecretTypeServiceAccountToken)}},
		},
		{
			name: "does not retain for type tls",
			args: args{
				desired:  createSecret(corev1.SecretTypeTLS, "111", corev1.TLSCertKey, "desired-cert"),
				observed: createSecret(corev1.SecretTypeTLS, "999", corev1.TLSCertKey, "observed-cert"),
			},
			want: createSecret(corev1.SecretTypeTLS, "111", corev1.TLSCertKey, "desired-cert"),
		},
		{
			name: "does not retain for type basic-auth",
			args: args{
				desired:  createSecret(corev1.SecretTypeBasicAuth, "111", corev1.BasicAuthUsernameKey, "desired-user"),
				observed: createSecret(corev1.SecretTypeBasicAuth, "999", corev1.BasicAuthUsernameKey, "observed-user"),
			},
			want: createSecret(corev1.SecretTypeBasicAuth, "111", corev1.BasicAuthUsernameKey, "desired-user"),
		},
		{
			name: "does not retain for type dockercfg",
			args: args{
				desired:  createSecret(corev1.SecretTypeDockercfg, "111", corev1.DockerConfigKey, "desired-docker-cfg"),
				observed: createSecret(corev1.SecretTypeDockercfg, "999", corev1.DockerConfigKey, "observed-docker-cfg"),
			},
			want: createSecret(corev1.SecretTypeDockercfg, "111", corev1.DockerConfigKey, "desired-docker-cfg"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainSecretServiceAccountToken(tt.args.desired, tt.args.observed)
			assert.Nil(t, err, "retainSecretServiceAccountToken() error = %v", err)
			assert.Equalf(t, tt.want, got, "retainSecretServiceAccountToken(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}
