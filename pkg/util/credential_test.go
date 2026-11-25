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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

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
			assertRegisteredCredentials(t, client, cluster, impersonator.Name)

			if _, err := clusters.ClusterV1alpha1().Clusters().Get(context.TODO(), "member1", metav1.GetOptions{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClaimRegistrationSecretPreservesMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		data map[string][]byte
	}{
		{name: "rotated", data: map[string][]byte{"token": []byte("new"), "caBundle": []byte("new-ca")}},
		{name: "unchanged", data: map[string][]byte{"token": []byte("old"), "obsolete": []byte("remove-me")}},
		{name: "clear data", data: map[string][]byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-cluster", Name: "member1",
					Labels:      map[string]string{"preserve": "label"},
					Annotations: map[string]string{"preserve": "annotation", "karmada.io/registration-attempt": "old-attempt"},
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1", UID: "cluster-uid"}}, clusterResourceKind)},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"token": []byte("old"), "obsolete": []byte("remove-me")},
			}
			client := fake.NewClientset(existing)
			desired := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: existing.Namespace, Name: existing.Name}, Data: tc.data}
			got, created, err := claimRegistrationSecret(client, desired, "member1", "new-attempt")
			if err != nil || created {
				t.Fatalf("claim: created=%v, error=%v", created, err)
			}
			assertClaimedSecret(t, got, existing, tc.data)
			stored, err := GetSecret(client, existing.Namespace, existing.Name)
			if err != nil {
				t.Fatal(err)
			}
			assertClaimedSecret(t, stored, existing, tc.data)
		})
	}
}

func assertClaimedSecret(t *testing.T, got, existing *corev1.Secret, data map[string][]byte) {
	t.Helper()
	if !reflect.DeepEqual(got.Data, data) {
		t.Errorf("data = %v, want %v", got.Data, data)
	}
	if got.Type != existing.Type || !reflect.DeepEqual(got.Labels, existing.Labels) || !reflect.DeepEqual(got.OwnerReferences, existing.OwnerReferences) {
		t.Errorf("claim changed type, labels or owners: %#v", got)
	}
	wantAnnotations := map[string]string{"preserve": "annotation", "karmada.io/registration-attempt": "new-attempt"}
	if !reflect.DeepEqual(got.Annotations, wantAnnotations) {
		t.Errorf("annotations = %v, want %v", got.Annotations, wantAnnotations)
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

// The fake tracker does not enforce resourceVersion preconditions like an API server.
func enforceCredentialVersions(t *testing.T, client *fake.Clientset) {
	t.Helper()
	resource := corev1.SchemeGroupVersion.WithResource("secrets")
	client.PrependReactor("create", "secrets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		secret := action.(clienttesting.CreateAction).GetObject().(*corev1.Secret)
		secret.UID = "credential-uid"
		secret.ResourceVersion = "1"
		return false, nil, nil
	})
	client.PrependReactor("patch", "secrets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		patch := action.(clienttesting.PatchAction)
		var body corev1.Secret
		if err := json.Unmarshal(patch.GetPatch(), &body); err != nil {
			t.Fatal(err)
		}
		if body.ResourceVersion == "" {
			return false, nil, nil
		}
		obj, err := client.Tracker().Get(resource, action.GetNamespace(), patch.GetName())
		if err != nil {
			return true, nil, err
		}
		secret := obj.(*corev1.Secret).DeepCopy()
		if secret.ResourceVersion != body.ResourceVersion {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, secret.Name, errors.New("stale claim"))
		}
		secret.ResourceVersion += "1"
		secret.Annotations = body.Annotations
		if body.Data != nil {
			secret.Data = body.Data
		}
		return true, secret, client.Tracker().Update(resource, secret, secret.Namespace)
	})
	client.PrependReactor("delete", "secrets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		deletion := action.(clienttesting.DeleteAction)
		obj, err := client.Tracker().Get(resource, action.GetNamespace(), deletion.GetName())
		if err != nil {
			return true, nil, err
		}
		secret := obj.(*corev1.Secret)
		pre := deletion.GetDeleteOptions().Preconditions
		if pre == nil || pre.UID == nil || pre.ResourceVersion == nil {
			t.Fatal("rollback must constrain UID and resourceVersion")
		}
		if *pre.UID != secret.UID || *pre.ResourceVersion != secret.ResourceVersion {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, secret.Name, errors.New("stale rollback"))
		}
		return false, nil, nil
	})
}

func TestRegisterClusterCredentialsConcurrentReuse(t *testing.T) {
	client := fake.NewClientset()
	enforceCredentialVersions(t, client)
	opts := ClusterRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", ReportSecrets: []string{KubeCredentials, KubeImpersonator}}
	reused := make(chan struct{})
	rolledBack := make(chan struct{})
	completed := make(chan error, 1)
	failure := errors.New("original registration failed")
	err := RegisterClusterInControllerPlane(opts, client, func(ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
		go func() {
			completed <- RegisterClusterInControllerPlane(opts, client, func(ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
				close(reused)
				<-rolledBack
				return &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1", UID: "cluster-uid"}}, nil
			})
		}()
		<-reused
		return nil, failure
	})
	close(rolledBack)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want original failure", err)
	}
	if err := <-completed; err != nil {
		t.Fatalf("successful registration lost reused credentials: %v", err)
	}
	for _, name := range []string{"member1", names.GenerateImpersonationSecretName("member1")} {
		if _, err := GetSecret(client, opts.ClusterNamespace, name); err != nil {
			t.Fatal(err)
		}
	}
}

func assertRegisteredCredentials(t *testing.T, client kubernetes.Interface, cluster *clusterv1alpha1.Cluster, impersonatorName string) {
	t.Helper()
	for name, token := range map[string]string{"member1": "new", impersonatorName: "new-impersonator"} {
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
}

func TestRegisterClusterCredentialsClaimConflict(t *testing.T) {
	existing := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "member1", Namespace: "karmada-cluster", ResourceVersion: "1"}}
	client := fake.NewClientset(existing)
	enforceCredentialVersions(t, client)
	claims := 0
	client.PrependReactor("patch", "secrets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		claims++
		patch := action.(clienttesting.PatchAction)
		var body corev1.Secret
		if err := json.Unmarshal(patch.GetPatch(), &body); err != nil {
			t.Fatal(err)
		}
		if body.ResourceVersion != "1" {
			t.Fatalf("claim must use observed resourceVersion, got %q", body.ResourceVersion)
		}
		// B claims the version A observed before A's patch reaches the API server.
		newer := existing.DeepCopy()
		newer.ResourceVersion = "2"
		newer.Annotations = map[string]string{"karmada.io/registration-attempt": "B"}
		if err := client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), newer, newer.Namespace); err != nil {
			t.Fatal(err)
		}
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, patch.GetName(), errors.New("B claimed first"))
	})
	opts := ClusterRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", ReportSecrets: []string{KubeImpersonator, KubeCredentials}}
	err := RegisterClusterInControllerPlane(opts, client, func(ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
		t.Fatal("generator must not run after claim conflict")
		return nil, nil
	})
	if !apierrors.IsConflict(err) {
		t.Fatalf("error = %v, want recognizable claim conflict", err)
	}
	if claims != 1 {
		t.Fatalf("claim retried %d times", claims)
	}
	if _, err := GetSecret(client, opts.ClusterNamespace, names.GenerateImpersonationSecretName(opts.ClusterName)); !apierrors.IsNotFound(err) {
		t.Fatalf("new secret not rolled back: %v", err)
	}
	got, err := GetSecret(client, opts.ClusterNamespace, opts.ClusterName)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResourceVersion != "2" {
		t.Fatalf("concurrent claim changed: %v", got)
	}
}

func TestRegisterClusterCredentialsOwnerConflict(t *testing.T) {
	for _, name := range []string{"member1", names.GenerateImpersonationSecretName("member1")} {
		t.Run(name, func(t *testing.T) {
			existing := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: "karmada-cluster",
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "other-cluster", UID: "other-uid"}}, clusterResourceKind)},
			}}
			client := fake.NewClientset(existing)
			before, err := GetSecret(client, existing.Namespace, existing.Name)
			if err != nil {
				t.Fatal(err)
			}
			opts := ClusterRegisterOption{ClusterName: "member1", ClusterNamespace: existing.Namespace, ReportSecrets: []string{KubeImpersonator, KubeCredentials}}
			err = RegisterClusterInControllerPlane(opts, client, func(ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
				t.Fatal("generator must not run after owner conflict")
				return nil, nil
			})
			if !apierrors.IsConflict(err) {
				t.Fatalf("error = %v, want recognizable owner conflict", err)
			}
			after, err := GetSecret(client, existing.Namespace, existing.Name)
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatalf("conflicting secret changed: %v", err)
			}
		})
	}
}

func TestRegisterClusterCredentialsRollbackDeleteFailure(t *testing.T) {
	client := fake.NewClientset()
	enforceCredentialVersions(t, client)
	deleted := []string{}
	client.PrependReactor("delete", "secrets", func(action clienttesting.Action) (bool, runtime.Object, error) {
		name := action.(clienttesting.DeleteAction).GetName()
		deleted = append(deleted, name)
		if len(deleted) == 1 {
			return true, nil, errors.New("delete forbidden")
		}
		return false, nil, nil
	})
	failure := errors.New("original generator failure")
	opts := ClusterRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", ReportSecrets: []string{KubeImpersonator, KubeCredentials}}
	err := RegisterClusterInControllerPlane(opts, client, func(ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) { return nil, failure })
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want original failure", err)
	}
	if !reflect.DeepEqual(deleted, []string{names.GenerateImpersonationSecretName("member1"), "member1"}) {
		t.Fatalf("cleanup stopped after delete error: %v", deleted)
	}
	if _, err := GetSecret(client, opts.ClusterNamespace, "member1"); !apierrors.IsNotFound(err) {
		t.Fatalf("second secret not deleted: %v", err)
	}
}
