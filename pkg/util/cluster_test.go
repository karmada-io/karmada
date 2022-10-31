package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	karmadaclientsetfake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util/gclient"
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

func withID(cluster *clusterv1alpha1.Cluster, id string) *clusterv1alpha1.Cluster {
	cluster.Spec.ID = id
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

func TestIsClusterIDUnique(t *testing.T) {
	tests := []struct {
		name           string
		existedCluster []runtime.Object
		id             string
		want           bool
		clustername    string
	}{
		{
			name: "no cluster", id: "1", want: true,
			existedCluster: []runtime.Object{},
		},
		{
			name: "existed id", id: "1", want: false, clustername: "cluster-1",
			existedCluster: []runtime.Object{withID(newCluster("cluster-1"), "1")},
		},
		{
			name: "unique id", id: "2", want: true,
			existedCluster: []runtime.Object{withID(newCluster("cluster-1"), "1")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := karmadaclientsetfake.NewSimpleClientset(tc.existedCluster...)

			ok, name, err := IsClusterIdentifyUnique(fakeClient, tc.id)
			if err != nil {
				t.Fatal(err)
			}

			if ok != tc.want {
				t.Errorf("expected value: %v, but got: %v", tc.want, ok)
			}

			if !ok && name != tc.clustername {
				t.Errorf("expected clustername: %v, but got: %v", tc.clustername, name)
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
