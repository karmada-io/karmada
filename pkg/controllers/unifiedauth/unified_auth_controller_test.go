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

package unifiedauth

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
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
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
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
			got, err := findRBACSubjectsWithCluster(context.Background(), tt.args.c, tt.args.cluster)
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

	type args struct {
		clusterRole *rbacv1.ClusterRole
	}
	tests := []struct {
		name string
		args args
		want []reconcile.Request
	}{
		{
			name: "specify resource names of cluster.karmada.io with cluster/proxy resource",
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
				Client:        fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster1, cluster2, cluster3).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			}
			if got := c.generateRequestsFromClusterRole(context.Background(), tt.args.clusterRole); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateRequestsFromClusterRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_buildImpersonationClusterRole(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *clusterv1alpha1.Cluster
		rules         []rbacv1.PolicyRule
		expectedWorks int
	}{
		{
			name: "successful creation",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
			},
			rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"impersonate"},
					APIGroups: []string{""},
					Resources: []string{"users", "groups", "serviceaccounts"},
				},
			},
			expectedWorks: 1,
		},
		{
			name: "cluster with no sync mode",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "no-sync-cluster"},
			},
			rules:         []rbacv1.PolicyRule{},
			expectedWorks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupTestScheme()

			c := &Controller{
				Client:        fake.NewClientBuilder().WithScheme(s).WithObjects(tt.cluster).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			}

			err := c.buildImpersonationClusterRole(context.Background(), tt.cluster, tt.rules)
			assert.NoError(t, err)

			var createdWorks workv1alpha1.WorkList
			err = c.Client.List(context.Background(), &createdWorks, &client.ListOptions{
				Namespace: generateExecutionSpaceName(tt.cluster.Name),
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedWorks, len(createdWorks.Items))
		})
	}
}

func TestController_buildImpersonationClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *clusterv1alpha1.Cluster
		expectedWorks int
		expectPanic   bool
	}{
		{
			name: "successful creation",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Push,
					ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-system",
						Name:      "test-secret",
					},
				},
			},
			expectedWorks: 1,
			expectPanic:   false,
		},
		{
			name: "cluster with no impersonator secret",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "no-secret-cluster"},
				Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
			},
			expectedWorks: 0,
			expectPanic:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupTestScheme()

			c := &Controller{
				Client:        fake.NewClientBuilder().WithScheme(s).WithObjects(tt.cluster).Build(),
				EventRecorder: record.NewFakeRecorder(1024),
			}

			if tt.expectPanic {
				assert.Panics(t, func() {
					_ = c.buildImpersonationClusterRoleBinding(context.Background(), tt.cluster)
				}, "Expected buildImpersonationClusterRoleBinding to panic, but it didn't")
			} else {
				err := c.buildImpersonationClusterRoleBinding(context.Background(), tt.cluster)
				assert.NoError(t, err)

				var createdWorks workv1alpha1.WorkList
				err = c.Client.List(context.Background(), &createdWorks, &client.ListOptions{
					Namespace: generateExecutionSpaceName(tt.cluster.Name),
				})
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedWorks, len(createdWorks.Items))
			}
		})
	}
}

func TestController_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *clusterv1alpha1.Cluster
		expectedResult reconcile.Result
		expectedErrMsg string
		expectedEvents []string
	}{
		{
			name: "successful reconciliation",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Push,
					ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-system",
						Name:      "test-secret",
					},
				},
			},
			expectedResult: reconcile.Result{},
			expectedEvents: []string{"Normal SyncImpersonationConfigSucceed Sync impersonation config succeed."},
		},
		{
			name: "cluster not found",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "non-existent-cluster"},
			},
			expectedResult: reconcile.Result{},
		},
		{
			name: "cluster being deleted",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deleting-cluster",
					DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
					Finalizers:        []string{"test-finalizer"},
				},
			},
			expectedResult: reconcile.Result{},
		},
		{
			name: "cluster without impersonator secret",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "no-secret-cluster"},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Push,
				},
			},
			expectedResult: reconcile.Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupTestScheme()

			fakeRecorder := record.NewFakeRecorder(10)
			c := &Controller{
				Client:        fake.NewClientBuilder().WithScheme(s).WithObjects(tt.cluster).Build(),
				EventRecorder: fakeRecorder,
			}

			result, err := c.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Name: tt.cluster.Name},
			})

			assert.Equal(t, tt.expectedResult, result)

			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}

			close(fakeRecorder.Events)
			var actualEvents []string
			for event := range fakeRecorder.Events {
				actualEvents = append(actualEvents, event)
			}
			assert.Equal(t, tt.expectedEvents, actualEvents)
		})
	}
}

func TestController_syncImpersonationConfig(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *clusterv1alpha1.Cluster
		existingRBAC   []client.Object
		expectedErrMsg string
		expectedWorks  int
	}{
		{
			name: "successful sync",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Push,
					ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-system",
						Name:      "test-secret",
					},
				},
			},
			existingRBAC: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
					Rules: []rbacv1.PolicyRule{
						{
							Verbs:         []string{"*"},
							APIGroups:     []string{"cluster.karmada.io"},
							Resources:     []string{"clusters/proxy"},
							ResourceNames: []string{"test-cluster"},
						},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "test-binding"},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "test-role",
					},
					Subjects: []rbacv1.Subject{
						{Kind: "User", Name: "test-user"},
					},
				},
			},
			expectedWorks: 2, // One for ClusterRole, one for ClusterRoleBinding
		},
		{
			name: "no matching RBAC",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1alpha1.ClusterSpec{
					SyncMode: clusterv1alpha1.Push,
					ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
						Namespace: "karmada-system",
						Name:      "test-secret",
					},
				},
			},
			expectedWorks: 2, // One for ClusterRole, one for ClusterRoleBinding
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupTestScheme()

			builder := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.cluster)
			for _, obj := range tt.existingRBAC {
				builder = builder.WithObjects(obj)
			}

			c := &Controller{
				Client:        builder.Build(),
				EventRecorder: record.NewFakeRecorder(10),
			}

			err := c.syncImpersonationConfig(context.Background(), tt.cluster)

			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)

				var createdWorks workv1alpha1.WorkList
				err = c.Client.List(context.Background(), &createdWorks, &client.ListOptions{
					Namespace: generateExecutionSpaceName(tt.cluster.Name),
				})
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedWorks, len(createdWorks.Items))
			}
		})
	}
}

func TestController_newClusterRoleMapFunc(t *testing.T) {
	tests := []struct {
		name           string
		clusterRole    *rbacv1.ClusterRole
		expectedLength int
	}{
		{
			name: "ClusterRole with matching cluster.karmada.io rule",
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"cluster.karmada.io"},
						Resources:     []string{"clusters/proxy"},
						ResourceNames: []string{"cluster1", "cluster2"},
					},
				},
			},
			expectedLength: 2,
		},
		{
			name: "ClusterRole with matching search.karmada.io rule",
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "search-role"},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"search.karmada.io"},
						Resources: []string{"proxying/proxy"},
					},
				},
			},
			expectedLength: 1, // 1 because we're not actually listing clusters in this test
		},
		{
			name: "ClusterRole without matching rules",
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "non-matching-role"},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}}).
				Build()

			c := &Controller{
				Client:        fakeClient,
				EventRecorder: record.NewFakeRecorder(1024),
			}

			mapFunc := c.newClusterRoleMapFunc()
			result := mapFunc(context.Background(), tt.clusterRole)
			assert.Len(t, result, tt.expectedLength)
		})
	}
}

func TestController_newClusterRoleBindingMapFunc(t *testing.T) {
	tests := []struct {
		name               string
		clusterRoleBinding *rbacv1.ClusterRoleBinding
		clusterRole        *rbacv1.ClusterRole
		expectedLength     int
	}{
		{
			name: "ClusterRoleBinding with matching ClusterRole",
			clusterRoleBinding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "test-binding"},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "test-role",
				},
			},
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "test-role"},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"cluster.karmada.io"},
						Resources:     []string{"clusters/proxy"},
						ResourceNames: []string{"cluster1", "cluster2"},
					},
				},
			},
			expectedLength: 2,
		},
		{
			name: "ClusterRoleBinding with non-matching ClusterRole",
			clusterRoleBinding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "non-matching-binding"},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: "non-matching-role",
				},
			},
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "non-matching-role"},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
					},
				},
			},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.clusterRole, &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}}).
				Build()

			c := &Controller{
				Client:        fakeClient,
				EventRecorder: record.NewFakeRecorder(1024),
			}

			mapFunc := c.newClusterRoleBindingMapFunc()
			result := mapFunc(context.Background(), tt.clusterRoleBinding)
			assert.Len(t, result, tt.expectedLength)
		})
	}
}

// Helper Functions

// generateExecutionSpaceName is a helper function to generate the execution space name
func generateExecutionSpaceName(clusterName string) string {
	return fmt.Sprintf("karmada-es-%s", clusterName)
}

// Helper function to setup scheme
func setupTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clusterv1alpha1.Install(s)
	_ = workv1alpha1.Install(s)
	_ = rbacv1.AddToScheme(s)
	return s
}
