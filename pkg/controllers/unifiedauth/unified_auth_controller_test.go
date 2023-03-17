package unifiedauth

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func Test_findRBACSubjectsWithCluster(t *testing.T) {
	clusterRoleWithCluster := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-proxy-clusterrole"},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"*"},
				APIGroups:     []string{"cluster.karmada.io"},
				Resources:     []string{"clusters/proxy"},
				ResourceNames: []string{"member1", "member2"},
			}}}
	clusterRoleBindingWithCluster := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-proxy-clusterrolebinding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     util.ClusterRoleKind,
			Name:     "cluster-proxy-clusterrole",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Namespace: "default", Name: "tom"},
			{Kind: "Group", Name: "system:serviceaccounts"},
			{Kind: "Group", Name: "system:serviceaccounts:default"},
		},
	}
	clusterRoleWithSearch := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "search-proxy-clusterrole"},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"search.karmada.io"},
				Resources: []string{"proxying/proxy"},
			}}}
	clusterRoleBindingWithSearch := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "search-proxy-clusterrolebinding"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     util.ClusterRoleKind,
			Name:     "search-proxy-clusterrole",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Namespace: "default", Name: "zhangsan"},
		},
	}

	type args struct {
		c       client.Client
		cluster string
	}
	tests := []struct {
		name    string
		args    args
		want    []rbacv1.Subject
		wantErr bool
	}{
		{
			name: "find rbac subjects with cluster",
			args: args{
				c: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					clusterRoleWithCluster, clusterRoleWithSearch, clusterRoleBindingWithCluster, clusterRoleBindingWithSearch).Build(),
				cluster: "member1",
			},
			want: []rbacv1.Subject{
				{Kind: "ServiceAccount", Namespace: "default", Name: "tom"},
				{Kind: "Group", Name: "system:serviceaccounts"},
				{Kind: "Group", Name: "system:serviceaccounts:default"},
				{Kind: "ServiceAccount", Namespace: "default", Name: "zhangsan"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findRBACSubjectsWithCluster(tt.args.c, tt.args.cluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("findRBACSubjectsWithCluster() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findRBACSubjectsWithCluster() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_generateRequestsFromClusterRole(t *testing.T) {
	cluster1 := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}}
	cluster2 := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member2"}}
	cluster3 := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member3"}}

	type fields struct {
		Client        client.Client
		EventRecorder record.EventRecorder
	}
	type args struct {
		clusterRole *rbacv1.ClusterRole
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []reconcile.Request
	}{
		{
			name: "specify resource names of cluster.karmada.io with cluster/proxy resource",
			fields: fields{
				Client:        fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			},
			args: args{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-proxy"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:         []string{"*"},
							APIGroups:     []string{"cluster.karmada.io"},
							Resources:     []string{"clusters/proxy"},
							ResourceNames: []string{"member1", "member2"},
						}}}},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "member1"}},
				{NamespacedName: types.NamespacedName{Name: "member2"}},
			},
		},
		{
			name: "specify cluster.karmada.io with cluster/proxy resource",
			fields: fields{
				Client:        fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			},
			args: args{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-proxy"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"cluster.karmada.io"},
							Resources: []string{"clusters/proxy"},
						}}}},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "member1"}},
				{NamespacedName: types.NamespacedName{Name: "member2"}},
				{NamespacedName: types.NamespacedName{Name: "member3"}},
			},
		},
		{
			name: "specify cluster.karmada.io with wildcard resource",
			fields: fields{
				Client:        fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			},
			args: args{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-proxy"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"cluster.karmada.io"},
							Resources: []string{"*"},
						}}}},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "member1"}},
				{NamespacedName: types.NamespacedName{Name: "member2"}},
				{NamespacedName: types.NamespacedName{Name: "member3"}},
			},
		},
		{
			name: "specify search.karmada.io with proxying/proxy resource",
			fields: fields{
				Client:        fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			},
			args: args{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "search-proxy"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"search.karmada.io"},
							Resources: []string{"proxying/proxy"},
						}}}},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "member1"}},
				{NamespacedName: types.NamespacedName{Name: "member2"}},
				{NamespacedName: types.NamespacedName{Name: "member3"}},
			},
		},
		{
			name: "specify search.karmada.io with wildcard resource",
			fields: fields{
				Client:        fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			},
			args: args{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "search-proxy"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"search.karmada.io"},
							Resources: []string{"*"},
						}}}},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "member1"}},
				{NamespacedName: types.NamespacedName{Name: "member2"}},
				{NamespacedName: types.NamespacedName{Name: "member3"}},
			},
		},
		{
			name: "specify wildcard apiGroups with wildcard resource",
			fields: fields{
				Client:        fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			},
			args: args{
				clusterRole: &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"*"},
							Resources: []string{"*"},
						}}}},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "member1"}},
				{NamespacedName: types.NamespacedName{Name: "member2"}},
				{NamespacedName: types.NamespacedName{Name: "member3"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Client:        tt.fields.Client,
				EventRecorder: tt.fields.EventRecorder,
			}
			if got := c.generateRequestsFromClusterRole(tt.args.clusterRole); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateRequestsFromClusterRole() = %v, want %v", got, tt.want)
			}
		})
	}
}
