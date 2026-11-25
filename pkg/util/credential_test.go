/*
Copyright 2022 The Karmada Authors.

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

package util

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util/names"
)

//nolint:gocyclo
func TestObtainCredentialsFromMemberCluster(t *testing.T) {
	type args struct {
		clusterKubeClient kubernetes.Interface
		opts              ClusterRegisterOption
		aop               func(t *testing.T, clusterKubeClient kubernetes.Interface) func()
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.Secret
		want1   *corev1.Secret
		wantErr bool
	}{
		{
			name: "disable report secret",
			args: args{
				opts: ClusterRegisterOption{
					ClusterNamespace: "karmada-cluster",
					ClusterName:      "member1",
					ReportSecrets:    []string{KubeImpersonator},
					DryRun:           false,
				},
				aop: func(t *testing.T, clusterKubeClient kubernetes.Interface) func() {
					ctx, cancel := context.WithCancel(context.TODO())
					var wg sync.WaitGroup
					wg.Add(1)
					go func(clusterKubeClient kubernetes.Interface) {
						defer wg.Done()
						_ = wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
							impersonationSA, err := clusterKubeClient.CoreV1().ServiceAccounts("karmada-cluster").Get(ctx, names.GenerateServiceAccountName("impersonator"), metav1.GetOptions{})
							if err != nil {
								t.Logf("get sa failed, err: %v", err)
								return false, nil
							}
							impersonatorSecret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: impersonationSA.Name},
								Type:       corev1.SecretTypeServiceAccountToken,
								Data: map[string][]byte{
									"token": []byte(`impersonatorSecret-token-value`),
								},
							}
							impersonationSA.Secrets = []corev1.ObjectReference{{Kind: "Secret", Namespace: impersonatorSecret.Namespace, Name: impersonatorSecret.Name}}
							_, _ = clusterKubeClient.CoreV1().ServiceAccounts(impersonationSA.Namespace).Update(ctx, impersonationSA, metav1.UpdateOptions{})
							_, err = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Create(ctx, impersonatorSecret, metav1.CreateOptions{})
							if err != nil && apierrors.IsAlreadyExists(err) {
								_, _ = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Update(ctx, impersonatorSecret, metav1.UpdateOptions{})
								t.Logf("secret exists and update it")
							}
							t.Log("create secret successfully")
							return true, nil
						})
					}(clusterKubeClient)
					return func() {
						cancel()
						wg.Wait()
					}
				},
			},
			want:  nil,
			want1: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: names.GenerateServiceAccountName("impersonator")}, Type: corev1.SecretTypeServiceAccountToken, Data: map[string][]byte{"token": []byte(`impersonatorSecret-token-value`)}},
		},
		{
			name: "report secret enabled",
			args: args{
				opts: ClusterRegisterOption{
					ClusterNamespace: "karmada-cluster",
					ClusterName:      "member1",
					ReportSecrets:    []string{KubeImpersonator, KubeCredentials},
					DryRun:           false,
				},
				aop: func(t *testing.T, clusterKubeClient kubernetes.Interface) func() {
					ctx, cancel := context.WithCancel(context.TODO())
					var wg sync.WaitGroup
					wg.Add(1)
					go func(clusterKubeClient kubernetes.Interface) {
						defer wg.Done()
						_ = wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
							impersonationSA, err := clusterKubeClient.CoreV1().ServiceAccounts("karmada-cluster").Get(ctx, names.GenerateServiceAccountName("impersonator"), metav1.GetOptions{})
							if err != nil {
								t.Logf("create impersonationSA failed, err:%v", err)
								return false, nil
							}
							impersonatorSecret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: impersonationSA.Name},
								Type:       corev1.SecretTypeServiceAccountToken,
								Data: map[string][]byte{
									"token": []byte(`impersonatorSecret-token-value`),
								},
							}
							impersonationSA.Secrets = []corev1.ObjectReference{{Kind: "Secret", Namespace: impersonatorSecret.Namespace, Name: impersonatorSecret.Name}}
							_, _ = clusterKubeClient.CoreV1().ServiceAccounts(impersonationSA.Namespace).Update(ctx, impersonationSA, metav1.UpdateOptions{})
							_, err = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Create(ctx, impersonatorSecret, metav1.CreateOptions{})
							if err != nil && apierrors.IsAlreadyExists(err) {
								_, _ = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Update(ctx, impersonatorSecret, metav1.UpdateOptions{})
								t.Log("impersonatorSecret exists and update it")
							}
							t.Log("create impersonatorSecret successfully")

							serviceAccountObj, err := clusterKubeClient.CoreV1().ServiceAccounts("karmada-cluster").Get(ctx, names.GenerateServiceAccountName("member1"), metav1.GetOptions{})
							if err != nil {
								t.Logf("get sa failed, err:%v", err)
								return false, nil
							}
							clusterSecret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: serviceAccountObj.Name},
								Type:       corev1.SecretTypeServiceAccountToken,
								Data: map[string][]byte{
									"token": []byte(`clusterSecret-token-value`),
								},
							}
							serviceAccountObj.Secrets = []corev1.ObjectReference{{Kind: "Secret", Namespace: clusterSecret.Namespace, Name: clusterSecret.Name}}
							_, _ = clusterKubeClient.CoreV1().ServiceAccounts(serviceAccountObj.Namespace).Update(ctx, serviceAccountObj, metav1.UpdateOptions{})
							_, err = clusterKubeClient.CoreV1().Secrets(clusterSecret.Namespace).Create(ctx, clusterSecret, metav1.CreateOptions{})
							if err != nil && apierrors.IsAlreadyExists(err) {
								_, _ = clusterKubeClient.CoreV1().Secrets(clusterSecret.Namespace).Update(ctx, clusterSecret, metav1.UpdateOptions{})
								t.Log("secret exist and update it")
							}
							t.Log("create clusterSecret successfully")
							return true, nil
						})
					}(clusterKubeClient)
					return func() {
						cancel()
						wg.Wait()
					}
				},
			},
			want: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: names.GenerateServiceAccountName("member1")}, Type: corev1.SecretTypeServiceAccountToken, Data: map[string][]byte{
				"token": []byte(`clusterSecret-token-value`),
			}},
			want1: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: names.GenerateServiceAccountName("impersonator")}, Type: corev1.SecretTypeServiceAccountToken, Data: map[string][]byte{
				"token": []byte(`impersonatorSecret-token-value`),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.clusterKubeClient = fake.NewClientset()
			if tt.args.aop != nil {
				cancel := tt.args.aop(t, tt.args.clusterKubeClient)
				defer cancel()
			}
			got, got1, err := ObtainCredentialsFromMemberCluster(tt.args.clusterKubeClient, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObtainCredentialsFromMemberCluster() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				// remove fields injected by fake client
				got.TypeMeta = metav1.TypeMeta{}
				got.ResourceVersion = ""
				got.UID = ""
				got.Generation = 0
				got.ManagedFields = nil
			}
			if got1 != nil {
				// remove fields injected by fake client
				got1.TypeMeta = metav1.TypeMeta{}
				got1.ResourceVersion = ""
				got1.UID = ""
				got1.Generation = 0
				got1.ManagedFields = nil
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObtainCredentialsFromMemberCluster() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ObtainCredentialsFromMemberCluster() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestRegisterClusterInControllerPlane(t *testing.T) {
	fakeClient := fake.NewClientset()
	type args struct {
		opts                             ClusterRegisterOption
		controlPlaneKubeClient           kubernetes.Interface
		generateClusterInControllerPlane generateClusterInControllerPlaneFunc
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "report secret not enabled",
			args: args{
				opts: ClusterRegisterOption{
					ClusterNamespace:   "karmada-cluster",
					ClusterName:        "member1",
					ReportSecrets:      []string{"KubeImpersonator"},
					DryRun:             false,
					ControlPlaneConfig: &rest.Config{},
					ClusterConfig:      &rest.Config{},
					ImpersonatorSecret: corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: names.GenerateImpersonationSecretName("member1")},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("impersonator-secret-token-value")},
					},
				},
				controlPlaneKubeClient: fakeClient,
				generateClusterInControllerPlane: func(opts ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
					clusterObj := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: opts.ClusterName}}
					clusterObj.Spec.SyncMode = clusterv1alpha1.Pull
					clusterObj.Spec.APIEndpoint = opts.ClusterAPIEndpoint
					clusterObj.Spec.ProxyURL = opts.ProxyServerAddress
					clusterObj.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
						Namespace: opts.ImpersonatorSecret.Namespace,
						Name:      opts.ImpersonatorSecret.Name,
					}
					return clusterObj, nil
				},
			},
		},
		{
			name: "enable report secret",
			args: args{
				opts: ClusterRegisterOption{
					ClusterNamespace:   "karmada-cluster",
					ClusterName:        "member1",
					ReportSecrets:      []string{"KubeImpersonator", "KubeCredentials"},
					DryRun:             false,
					ControlPlaneConfig: &rest.Config{},
					ClusterConfig:      &rest.Config{},
					Secret: corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: "member1"},
						Data:       map[string][]byte{"ca.crt": []byte("ca.crt-values"), clusterv1alpha1.SecretTokenKey: []byte(`secret-token-value`)},
					},
					ImpersonatorSecret: corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: names.GenerateImpersonationSecretName("member1")},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte(`impersonator-secret-token-value`)},
					},
				},
				controlPlaneKubeClient: fakeClient,
				generateClusterInControllerPlane: func(opts ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
					clusterObj := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: opts.ClusterName}}
					clusterObj.Spec.SyncMode = clusterv1alpha1.Push
					clusterObj.Spec.APIEndpoint = opts.ClusterAPIEndpoint
					clusterObj.Spec.ProxyURL = opts.ProxyServerAddress
					clusterObj.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
						Namespace: opts.ImpersonatorSecret.Namespace,
						Name:      opts.ImpersonatorSecret.Name,
					}
					clusterObj.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
						Namespace: opts.Secret.Namespace,
						Name:      opts.Secret.Name,
					}
					return clusterObj, nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RegisterClusterInControllerPlane(tt.args.opts, tt.args.controlPlaneKubeClient, tt.args.generateClusterInControllerPlane); (err != nil) != tt.wantErr {
				t.Errorf("RegisterClusterInControllerPlane() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterClusterCredentials(t *testing.T) {
	for _, existingCluster := range []bool{false, true} {
		t.Run(fmt.Sprint("existing cluster=", existingCluster), func(t *testing.T) {
			cluster := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1", UID: "cluster-uid"}}
			clusters := karmadafake.NewClientset()
			old := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "karmada-cluster", Name: "member1"}, Data: map[string][]byte{"token": []byte("old"), "caBundle": []byte("old-ca")}}
			if existingCluster {
				old.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(cluster, clusterResourceKind)}
				if _, err := clusters.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			impersonator := old.DeepCopy()
			impersonator.Name = names.GenerateImpersonationSecretName("member1")
			client := fake.NewClientset(old, impersonator)
			opts := ClusterRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", ReportSecrets: []string{KubeCredentials, KubeImpersonator}, Secret: corev1.Secret{Data: map[string][]byte{"token": []byte("new"), "ca.crt": []byte("new-ca")}}, ImpersonatorSecret: corev1.Secret{Data: map[string][]byte{"token": []byte("new-impersonator")}}}
			err := RegisterClusterInControllerPlane(opts, client, func(got ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
				if string(got.Secret.Data["token"]) != "new" || string(got.Secret.Data["caBundle"]) != "new-ca" || string(got.ImpersonatorSecret.Data["token"]) != "new-impersonator" {
					t.Error("generator received stale credentials")
				}
				if existingCluster {
					return clusters.ClusterV1alpha1().Clusters().Get(context.TODO(), "member1", metav1.GetOptions{})
				}
				return clusters.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
			})
			if err != nil {
				t.Fatal(err)
			}
			for name, token := range map[string]string{"member1": "new", impersonator.Name: "new-impersonator"} {
				got, err := GetSecret(client, "karmada-cluster", name)
				if err != nil {
					t.Fatal(err)
				}
				if string(got.Data["token"]) != token {
					t.Errorf("%s token = %q, want %q", name, got.Data["token"], token)
				}
				if name == "member1" && string(got.Data["caBundle"]) != "new-ca" {
					t.Errorf("CA = %q, want new-ca", got.Data["caBundle"])
				}
				if !reflect.DeepEqual(got.OwnerReferences, []metav1.OwnerReference{*metav1.NewControllerRef(cluster, clusterResourceKind)}) {
					t.Errorf("unexpected owners: %v", got.OwnerReferences)
				}
			}
			if _, err := clusters.ClusterV1alpha1().Clusters().Get(context.TODO(), "member1", metav1.GetOptions{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRegisterClusterCredentialsRollback(t *testing.T) {
	for _, preset := range []string{"", "member1", names.GenerateImpersonationSecretName("member1")} {
		t.Run("preset="+preset, func(t *testing.T) {
			client := fake.NewClientset()
			if preset != "" {
				if _, err := client.CoreV1().Secrets("karmada-cluster").Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: preset}, Data: map[string][]byte{"token": []byte("old")}}, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			failure := errors.New("cluster generation failed")
			err := RegisterClusterInControllerPlane(ClusterRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", ReportSecrets: []string{KubeCredentials, KubeImpersonator}}, client, func(ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) { return nil, failure })
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v, want original error", err)
			}
			for _, name := range []string{"member1", names.GenerateImpersonationSecretName("member1")} {
				_, err := GetSecret(client, "karmada-cluster", name)
				if name == preset {
					if err != nil {
						t.Errorf("preset secret removed: %v", err)
					}
				} else if !apierrors.IsNotFound(err) {
					t.Errorf("new secret %s remains, err=%v", name, err)
				}
			}
		})
	}
}
