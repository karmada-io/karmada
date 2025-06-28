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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	karmadaclientsetfake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func newCluster(name string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec:   clusterv1alpha1.ClusterSpec{},
		Status: clusterv1alpha1.ClusterStatus{},
	}
}

func withAPIEndPoint(cluster *clusterv1alpha1.Cluster, apiEndPoint string) *clusterv1alpha1.Cluster {
	cluster.Spec.APIEndpoint = apiEndPoint
	return cluster
}

func withSyncMode(cluster *clusterv1alpha1.Cluster, syncMode clusterv1alpha1.ClusterSyncMode) *clusterv1alpha1.Cluster {
	cluster.Spec.SyncMode = syncMode
	return cluster
}

func TestCreateOrUpdateClusterObject(t *testing.T) {
	type args struct {
		controlPlaneClient karmadaclientset.Interface
		clusterObj         *clusterv1alpha1.Cluster
		mutate             func(*clusterv1alpha1.Cluster)
	}
	tests := []struct {
		name    string
		args    args
		want    *clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "cluster exist, and update cluster",
			args: args{
				controlPlaneClient: karmadaclientsetfake.NewSimpleClientset(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443")),
				clusterObj:         newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
			},
			want: withSyncMode(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443"), clusterv1alpha1.Pull),
		},
		{
			name: "cluster exist and equal, not update",
			args: args{
				controlPlaneClient: karmadaclientsetfake.NewSimpleClientset(withSyncMode(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443"), clusterv1alpha1.Pull)),
				clusterObj:         withSyncMode(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443"), clusterv1alpha1.Pull),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
			},
			want: withSyncMode(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443"), clusterv1alpha1.Pull),
		},
		{
			name: "cluster not exist, and create cluster",
			args: args{
				controlPlaneClient: karmadaclientsetfake.NewSimpleClientset(),
				clusterObj:         newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
			},
			want: withSyncMode(newCluster(ClusterMember1), clusterv1alpha1.Pull),
		},
		{
			name: "get cluster error",
			args: args{
				controlPlaneClient: func() karmadaclientset.Interface {
					c := karmadaclientsetfake.NewSimpleClientset()
					c.PrependReactor("get", "*", errorAction)
					return c
				}(),
				clusterObj: newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "create cluster error",
			args: args{
				controlPlaneClient: func() karmadaclientset.Interface {
					c := karmadaclientsetfake.NewSimpleClientset()
					c.PrependReactor("create", "*", errorAction)
					return c
				}(),
				clusterObj: newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
			},
			wantErr: true,
			want:    nil,
		},
		{
			name: "update cluster error",
			args: args{
				controlPlaneClient: func() karmadaclientset.Interface {
					c := karmadaclientsetfake.NewSimpleClientset(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443"))
					c.PrependReactor("update", "*", errorAction)
					return c
				}(),
				clusterObj: newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
			},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateOrUpdateClusterObject(tt.args.controlPlaneClient, tt.args.clusterObj, tt.args.mutate)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateClusterObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateOrUpdateClusterObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterRegisterOption_IsKubeCredentialsEnabled(t *testing.T) {
	type fields struct {
		ReportSecrets []string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "secrets empty",
			fields: fields{
				ReportSecrets: nil,
			},
			want: false,
		},
		{
			name: "secrets are [None]",
			fields: fields{
				ReportSecrets: []string{None},
			},
			want: false,
		},
		{
			name: "secrets are [KubeCredentials]",
			fields: fields{
				ReportSecrets: []string{KubeCredentials},
			},
			want: true,
		},
		{
			name: "secrets are [None,KubeCredentials]",
			fields: fields{
				ReportSecrets: []string{None, KubeCredentials},
			},
			want: true,
		},
		{
			name: "secrets are [None,None]",
			fields: fields{
				ReportSecrets: []string{None, None},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ClusterRegisterOption{
				ReportSecrets: tt.fields.ReportSecrets,
			}
			if got := r.IsKubeCredentialsEnabled(); got != tt.want {
				t.Errorf("IsKubeCredentialsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterRegisterOption_IsKubeImpersonatorEnabled(t *testing.T) {
	type fields struct {
		ReportSecrets []string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "secrets empty",
			fields: fields{
				ReportSecrets: nil,
			},
			want: false,
		},
		{
			name: "secrets are [None]",
			fields: fields{
				ReportSecrets: []string{None},
			},
			want: false,
		},
		{
			name: "secrets are [KubeImpersonator]",
			fields: fields{
				ReportSecrets: []string{KubeImpersonator},
			},
			want: true,
		},
		{
			name: "secrets are [None,KubeImpersonator]",
			fields: fields{
				ReportSecrets: []string{None, KubeImpersonator},
			},
			want: true,
		},
		{
			name: "secrets are [None,None]",
			fields: fields{
				ReportSecrets: []string{None, None},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ClusterRegisterOption{
				ReportSecrets: tt.fields.ReportSecrets,
			}
			if got := r.IsKubeImpersonatorEnabled(); got != tt.want {
				t.Errorf("IsKubeImpersonatorEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterRegisterOption_ValidateCluster(t *testing.T) {
	registeredClusterList := &clusterv1alpha1.ClusterList{
		Items: []clusterv1alpha1.Cluster{
			{
				Spec: clusterv1alpha1.ClusterSpec{
					ID: "1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "member1",
				},
			},
			{
				Spec: clusterv1alpha1.ClusterSpec{
					ID: "2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "member2",
				},
			},
			{
				Spec: clusterv1alpha1.ClusterSpec{
					ID: "3",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "member3",
				},
			},
		},
	}

	testItems := []struct {
		name                    string
		clusterList             *clusterv1alpha1.ClusterList
		opts                    ClusterRegisterOption
		expectedClusterIDUsed   bool
		expectedClusterNameUsed bool
		expectedSameCluster     bool
	}{
		{
			name:        "registering a brand new cluster",
			clusterList: registeredClusterList,
			opts: ClusterRegisterOption{
				ClusterID:   "4",
				ClusterName: "member4",
			},
			expectedClusterIDUsed:   false,
			expectedClusterNameUsed: false,
			expectedSameCluster:     false,
		},
		{
			name:        "clusterName is used",
			clusterList: registeredClusterList,
			opts: ClusterRegisterOption{
				ClusterID:   "4",
				ClusterName: "member2",
			},
			expectedClusterIDUsed:   false,
			expectedClusterNameUsed: true,
			expectedSameCluster:     false,
		},
		{
			name:        "clusterID is used",
			clusterList: registeredClusterList,
			opts: ClusterRegisterOption{
				ClusterID:   "2",
				ClusterName: "member4",
			},
			expectedClusterIDUsed:   true,
			expectedClusterNameUsed: false,
			expectedSameCluster:     false,
		},
		{
			name:        "the same cluster",
			clusterList: registeredClusterList,
			opts: ClusterRegisterOption{
				ClusterID:   "2",
				ClusterName: "member2",
			},
			expectedClusterIDUsed:   true,
			expectedClusterNameUsed: true,
			expectedSameCluster:     true,
		},
	}

	for _, item := range testItems {
		t.Run(item.name, func(t *testing.T) {
			clusterIDUsed, clusterNameUsed, sameCluster := item.opts.validateCluster(item.clusterList)
			if clusterIDUsed != item.expectedClusterIDUsed {
				t.Errorf("clusterNameUsed = %v, want %v", clusterIDUsed, item.expectedClusterIDUsed)
			}
			if clusterNameUsed != item.expectedClusterNameUsed {
				t.Errorf("clusterNameUsed = %v, want %v", clusterNameUsed, item.expectedClusterNameUsed)
			}
			if sameCluster != item.expectedSameCluster {
				t.Errorf("clusterNameUsed = %v, want %v", sameCluster, item.expectedSameCluster)
			}
		})
	}
}

func TestCreateClusterObject(t *testing.T) {
	type args struct {
		controlPlaneClient karmadaclientset.Interface
		clusterObj         *clusterv1alpha1.Cluster
	}
	tests := []struct {
		name    string
		args    args
		want    *clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "cluster not exit, and create it",
			args: args{
				controlPlaneClient: karmadaclientsetfake.NewSimpleClientset(),
				clusterObj:         newCluster("test"),
			},
			want:    newCluster("test"),
			wantErr: false,
		},
		{
			name: "cluster exits, and return error",
			args: args{
				controlPlaneClient: karmadaclientsetfake.NewSimpleClientset(newCluster("test")),
				clusterObj:         newCluster("test"),
			},
			want:    newCluster("test"),
			wantErr: true,
		},
		{
			name: "get cluster error",
			args: args{
				controlPlaneClient: func() karmadaclientset.Interface {
					c := karmadaclientsetfake.NewSimpleClientset()
					c.PrependReactor("get", "*", errorAction)
					return c
				}(),
				clusterObj: newCluster("test"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create cluster error",
			args: args{
				controlPlaneClient: func() karmadaclientset.Interface {
					c := karmadaclientsetfake.NewSimpleClientset()
					c.PrependReactor("create", "*", errorAction)
					return c
				}(),
				clusterObj: newCluster("test"),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateClusterObject(tt.args.controlPlaneClient, tt.args.clusterObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateClusterObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateClusterObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCluster(t *testing.T) {
	type args struct {
		hostClient  client.Client
		clusterName string
	}
	tests := []struct {
		name    string
		args    args
		want    *clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "cluster not found",
			args: args{
				hostClient:  fakeclient.NewClientBuilder().Build(),
				clusterName: "test",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "cluster get success",
			args: args{
				hostClient:  fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("test")).Build(),
				clusterName: "test",
			},
			want:    newCluster("test"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCluster(tt.args.hostClient, tt.args.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCluster() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				// remove fields injected by fake client
				got.TypeMeta = metav1.TypeMeta{}
				got.ResourceVersion = ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCluster() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObtainClusterID(t *testing.T) {
	type args struct {
		clusterKubeClient kubernetes.Interface
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "namespace not exist",
			args: args{
				clusterKubeClient: fake.NewSimpleClientset(),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "namespace exists",
			args: args{
				clusterKubeClient: fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem, UID: "123"}}),
			},
			want:    "123",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ObtainClusterID(tt.args.clusterKubeClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("ObtainClusterID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ObtainClusterID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterRegisterOption_Validate(t *testing.T) {
	var testItems = []struct {
		name            string
		registerOptions ClusterRegisterOption
		isAgent         bool
		wantErr         bool
		assertMsg       string
	}{
		{
			name: "agent restart",
			registerOptions: ClusterRegisterOption{
				ClusterName: "pull",
				ClusterID:   "123",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "pull"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "123"},
					},
				),
			},
			isAgent:   true,
			wantErr:   false,
			assertMsg: "should not return an error when the agent is restarted",
		},
		{
			name: "the pull mode cluster registered again with a different cluster name",
			registerOptions: ClusterRegisterOption{
				ClusterName: "foo",
				ClusterID:   "123",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "pull"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "123"},
					},
				),
			},
			isAgent:   true,
			wantErr:   true,
			assertMsg: "should return an error when the pull mode cluster is registered again with a different cluster name",
		},
		{
			name: "a pull mode cluster registered with an existing cluster name",
			registerOptions: ClusterRegisterOption{
				ClusterName: "pull",
				ClusterID:   "234",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "pull"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "123"},
					},
				),
			},
			isAgent:   true,
			wantErr:   true,
			assertMsg: "should return an error when a new pull mode cluster registered with an existing cluster name",
		},
		{
			name: "a new push mode cluster registered",
			registerOptions: ClusterRegisterOption{
				ClusterName: "push",
				ClusterID:   "123",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "foo"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "234"},
					},
				),
			},
			isAgent:   false,
			wantErr:   false,
			assertMsg: "should not return an error when a new push mode cluster registered",
		},
		{
			name: "a push mode cluster registered with an existing cluster name",
			registerOptions: ClusterRegisterOption{
				ClusterName: "foo",
				ClusterID:   "123",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "foo"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "234"},
					},
				),
			},
			isAgent:   false,
			wantErr:   true,
			assertMsg: "should return an error when a push mode cluster registered  with an existing cluster name",
		},
		{
			name: "the push mode cluster registered again with a different cluster name",
			registerOptions: ClusterRegisterOption{
				ClusterName: "push",
				ClusterID:   "234",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "foo"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "234"},
					},
				),
			},
			isAgent:   false,
			wantErr:   true,
			assertMsg: "should return an error when the push mode cluster registered again with a different cluster name",
		},
		{
			name: "the push mode cluster registered again",
			registerOptions: ClusterRegisterOption{
				ClusterName: "foo",
				ClusterID:   "234",
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "foo"},
						Spec:       clusterv1alpha1.ClusterSpec{ID: "234"},
					},
				),
			},
			isAgent:   false,
			wantErr:   true,
			assertMsg: "should return an error when the push mode cluster registered again",
		},
	}

	for _, tt := range testItems {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.registerOptions.Validate(tt.isAgent)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestClusterRegisterOption_RunRegister(t *testing.T) {
	generateClusterInControllerPlane := func(opts ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
		clusterObj := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: opts.ClusterName}}
		clusterObj.Spec.APIEndpoint = opts.ClusterAPIEndpoint
		clusterObj.Spec.ProxyURL = opts.ProxyServerAddress
		clusterObj.Spec.ID = opts.ClusterID

		if opts.IsKubeCredentialsEnabled() {
			clusterObj.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
				Namespace: opts.Secret.Namespace,
				Name:      opts.Secret.Name,
			}
		}
		if opts.IsKubeImpersonatorEnabled() {
			clusterObj.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
				Namespace: opts.ImpersonatorSecret.Namespace,
				Name:      opts.ImpersonatorSecret.Name,
			}
		}
		return opts.KarmadaClient.ClusterV1alpha1().Clusters().Create(context.TODO(), clusterObj, metav1.CreateOptions{})
	}

	var testItems = []struct {
		name                   string
		registerOptions        ClusterRegisterOption
		wantErr                bool
		clusterExists          bool
		expectedCluster        *clusterv1alpha1.Cluster
		kubeImpersonatorExists bool
		kubeCredentialExists   bool
	}{
		{
			name: "do nothing when dry run",
			registerOptions: ClusterRegisterOption{
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(),
				ClusterName:   "foo",
				ClusterID:     "123",
				DryRun:        true,
			},
			wantErr:                false,
			clusterExists:          false,
			expectedCluster:        nil,
			kubeImpersonatorExists: false,
			kubeCredentialExists:   false,
		},
		{
			name: "report kube credentials",
			registerOptions: ClusterRegisterOption{
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(),
				ClusterKubeClient: fake.NewSimpleClientset(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      names.GenerateServiceAccountName("foo"),
							Namespace: "karmada-cluster",
						},
						Data: map[string][]byte{
							"token": []byte("test token"),
						},
					}),
				ControlPlaneKubeClient: fake.NewSimpleClientset(),
				ClusterName:            "foo",
				ClusterNamespace:       "karmada-cluster",
				ClusterID:              "123",
				ReportSecrets:          []string{"KubeCredentials"},
				ClusterAPIEndpoint:     "https://172.18.0.3:6443",
				ProxyServerAddress:     "/foo",
			},
			wantErr:       false,
			clusterExists: true,
			expectedCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					ID:          "123",
					APIEndpoint: "https://172.18.0.3:6443",
					ProxyURL:    "/foo",
					SecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-cluster",
						Name:      "foo",
					},
					ImpersonatorSecretRef: nil,
				},
			},
		},
		{
			name: "report kube impersonator",
			registerOptions: ClusterRegisterOption{
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(),
				ClusterKubeClient: fake.NewSimpleClientset(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      names.GenerateServiceAccountName("impersonator"),
							Namespace: "karmada-cluster",
						},
						Data: map[string][]byte{
							"token": []byte("test token"),
						},
					}),
				ControlPlaneKubeClient: fake.NewSimpleClientset(),
				ClusterName:            "foo",
				ClusterNamespace:       "karmada-cluster",
				ClusterID:              "123",
				ReportSecrets:          []string{"KubeImpersonator"},
				ClusterAPIEndpoint:     "https://172.18.0.3:6443",
				ProxyServerAddress:     "/foo",
			},
			wantErr:       false,
			clusterExists: true,
			expectedCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					ID:          "123",
					APIEndpoint: "https://172.18.0.3:6443",
					ProxyURL:    "/foo",
					SecretRef:   nil,
					ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-cluster",
						Name:      names.GenerateImpersonationSecretName("foo"),
					},
				},
			},
		},
		{
			name: "report kube impersonator and kube credentials",
			registerOptions: ClusterRegisterOption{
				KarmadaClient: karmadaclientsetfake.NewSimpleClientset(),
				ClusterKubeClient: fake.NewSimpleClientset(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      names.GenerateServiceAccountName("foo"),
							Namespace: "karmada-cluster",
						},
						Data: map[string][]byte{
							"token": []byte("test token"),
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      names.GenerateServiceAccountName("impersonator"),
							Namespace: "karmada-cluster",
						},
						Data: map[string][]byte{
							"token": []byte("test token"),
						},
					},
				),
				ControlPlaneKubeClient: fake.NewSimpleClientset(),
				ClusterName:            "foo",
				ClusterNamespace:       "karmada-cluster",
				ClusterID:              "123",
				ReportSecrets:          []string{"KubeImpersonator"},
				ClusterAPIEndpoint:     "https://172.18.0.3:6443",
				ProxyServerAddress:     "/foo",
			},
			wantErr:       false,
			clusterExists: true,
			expectedCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					ID:          "123",
					APIEndpoint: "https://172.18.0.3:6443",
					ProxyURL:    "/foo",
					SecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-cluster",
						Name:      "foo",
					},
					ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-cluster",
						Name:      names.GenerateImpersonationSecretName("foo"),
					},
				},
			},
		},
		{
			name: "report none",
			registerOptions: ClusterRegisterOption{
				KarmadaClient:          karmadaclientsetfake.NewSimpleClientset(),
				ClusterKubeClient:      fake.NewSimpleClientset(),
				ControlPlaneKubeClient: fake.NewSimpleClientset(),
				ClusterName:            "foo",
				ClusterNamespace:       "karmada-cluster",
				ClusterID:              "123",
				ReportSecrets:          []string{"None"},
				ClusterAPIEndpoint:     "https://172.18.0.3:6443",
				ProxyServerAddress:     "/foo",
			},
			wantErr:       false,
			clusterExists: true,
			expectedCluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: clusterv1alpha1.ClusterSpec{
					ID:                    "123",
					APIEndpoint:           "https://172.18.0.3:6443",
					ProxyURL:              "/foo",
					SecretRef:             nil,
					ImpersonatorSecretRef: nil,
				},
			},
		},
	}

	for _, tt := range testItems {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.registerOptions.RunRegister(generateClusterInControllerPlane)
			assert.Equal(t, tt.wantErr, err != nil)
			get, err := tt.registerOptions.KarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), tt.registerOptions.ClusterName, metav1.GetOptions{})
			if tt.clusterExists {
				assert.Equal(t, nil, err)
				assert.Equal(t, tt.expectedCluster.Name, get.Name)
				assert.Equal(t, tt.expectedCluster.Spec.ID, get.Spec.ID)
				assert.Equal(t, tt.expectedCluster.Spec.APIEndpoint, get.Spec.APIEndpoint)
				assert.Equal(t, tt.expectedCluster.Spec.ProxyURL, get.Spec.ProxyURL)
				if tt.registerOptions.IsKubeCredentialsEnabled() {
					assert.NotNil(t, get.Spec.SecretRef)
					assert.Equal(t, tt.expectedCluster.Spec.SecretRef.Namespace, get.Spec.SecretRef.Namespace)
					assert.Equal(t, tt.expectedCluster.Spec.SecretRef.Name, get.Spec.SecretRef.Name)
				} else {
					assert.Nil(t, get.Spec.SecretRef)
				}
				if tt.registerOptions.IsKubeImpersonatorEnabled() {
					assert.NotNil(t, get.Spec.ImpersonatorSecretRef)
					assert.Equal(t, tt.expectedCluster.Spec.ImpersonatorSecretRef.Namespace, get.Spec.ImpersonatorSecretRef.Namespace)
					assert.Equal(t, tt.expectedCluster.Spec.ImpersonatorSecretRef.Name, get.Spec.ImpersonatorSecretRef.Name)
				} else {
					assert.Nil(t, get.Spec.ImpersonatorSecretRef)
				}
			} else {
				assert.Equal(t, true, apierrors.IsNotFound(err))
			}
		})
	}
}
