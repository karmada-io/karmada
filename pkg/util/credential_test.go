package util

import (
	"context"
	"reflect"
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
	"github.com/karmada-io/karmada/pkg/util/names"
)

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
					stopChan := make(chan struct{})
					go func(clusterKubeClient kubernetes.Interface) {
						_ = wait.PollUntil(1*time.Second, func() (done bool, err error) {
							impersonationSA, err := clusterKubeClient.CoreV1().ServiceAccounts("karmada-cluster").Get(context.TODO(), names.GenerateServiceAccountName("impersonator"), metav1.GetOptions{})
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
							_, _ = clusterKubeClient.CoreV1().ServiceAccounts(impersonationSA.Namespace).Update(context.TODO(), impersonationSA, metav1.UpdateOptions{})
							_, err = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Create(context.TODO(), impersonatorSecret, metav1.CreateOptions{})
							if err != nil && apierrors.IsAlreadyExists(err) {
								_, _ = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Update(context.TODO(), impersonatorSecret, metav1.UpdateOptions{})
								t.Logf("secret exists and update it")
							}
							t.Log("create secret successfully")
							return true, nil
						}, stopChan)
					}(clusterKubeClient)
					return func() {
						close(stopChan)
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
					stopChan := make(chan struct{})
					go func(clusterKubeClient kubernetes.Interface) {
						_ = wait.PollUntil(1*time.Second, func() (done bool, err error) {
							impersonationSA, err := clusterKubeClient.CoreV1().ServiceAccounts("karmada-cluster").Get(context.TODO(), names.GenerateServiceAccountName("impersonator"), metav1.GetOptions{})
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
							_, _ = clusterKubeClient.CoreV1().ServiceAccounts(impersonationSA.Namespace).Update(context.TODO(), impersonationSA, metav1.UpdateOptions{})
							_, err = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Create(context.TODO(), impersonatorSecret, metav1.CreateOptions{})
							if err != nil && apierrors.IsAlreadyExists(err) {
								_, _ = clusterKubeClient.CoreV1().Secrets(impersonatorSecret.Namespace).Update(context.TODO(), impersonatorSecret, metav1.UpdateOptions{})
								t.Log("impersonatorSecret exists and update it")
							}
							t.Log("create impersonatorSecret successfully")

							serviceAccountObj, err := clusterKubeClient.CoreV1().ServiceAccounts("karmada-cluster").Get(context.TODO(), names.GenerateServiceAccountName("member1"), metav1.GetOptions{})
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
							_, _ = clusterKubeClient.CoreV1().ServiceAccounts(serviceAccountObj.Namespace).Update(context.TODO(), serviceAccountObj, metav1.UpdateOptions{})
							_, err = clusterKubeClient.CoreV1().Secrets(clusterSecret.Namespace).Create(context.TODO(), clusterSecret, metav1.CreateOptions{})
							if err != nil && apierrors.IsAlreadyExists(err) {
								_, _ = clusterKubeClient.CoreV1().Secrets(clusterSecret.Namespace).Update(context.TODO(), clusterSecret, metav1.UpdateOptions{})
								t.Log("secret exist and update it")
							}
							t.Log("create clusterSecret successfully")
							return true, nil
						}, stopChan)
					}(clusterKubeClient)
					return func() {
						close(stopChan)
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
			tt.args.clusterKubeClient = fake.NewSimpleClientset()
			if tt.args.aop != nil {
				cancel := tt.args.aop(t, tt.args.clusterKubeClient)
				defer cancel()
			}
			got, got1, err := ObtainCredentialsFromMemberCluster(tt.args.clusterKubeClient, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObtainCredentialsFromMemberCluster() error = %v, wantErr %v", err, tt.wantErr)
				return
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
	fakeClient := fake.NewSimpleClientset()
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
