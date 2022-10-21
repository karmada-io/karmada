package util

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateClusterRole(t *testing.T) {
	type args struct {
		client         kubernetes.Interface
		clusterRoleObj *rbacv1.ClusterRole
	}
	tests := []struct {
		name    string
		args    args
		want    *rbacv1.ClusterRole
		wantErr bool
	}{
		{
			name: "already exist",
			args: args{
				client:         fake.NewSimpleClientset(makeClusterRole("test")),
				clusterRoleObj: makeClusterRole("test"),
			},
			want:    makeClusterRole("test"),
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client:         alwaysErrorKubeClient,
				clusterRoleObj: makeClusterRole("test"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create success",
			args: args{
				client:         fake.NewSimpleClientset(),
				clusterRoleObj: makeClusterRole("test"),
			},
			want:    makeClusterRole("test"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateClusterRole(tt.args.client, tt.args.clusterRoleObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateClusterRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateClusterRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateClusterRoleBinding(t *testing.T) {
	type args struct {
		client                kubernetes.Interface
		clusterRoleBindingObj *rbacv1.ClusterRoleBinding
	}
	tests := []struct {
		name    string
		args    args
		want    *rbacv1.ClusterRoleBinding
		wantErr bool
	}{
		{
			name: "already exist",
			args: args{
				client:                fake.NewSimpleClientset(makeClusterRoleBinding("test")),
				clusterRoleBindingObj: makeClusterRoleBinding("test"),
			},
			want:    makeClusterRoleBinding("test"),
			wantErr: false,
		},
		{
			name: "create success",
			args: args{
				client:                fake.NewSimpleClientset(),
				clusterRoleBindingObj: makeClusterRoleBinding("test"),
			},
			want:    makeClusterRoleBinding("test"),
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client:                alwaysErrorKubeClient,
				clusterRoleBindingObj: makeClusterRoleBinding("test"),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateClusterRoleBinding(tt.args.client, tt.args.clusterRoleBindingObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateClusterRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateClusterRoleBinding() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateOrUpdateClusterRole(t *testing.T) {
	type args struct {
		client      kubernetes.Interface
		clusterRole *rbacv1.ClusterRole
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not exist and create",
			args: args{
				client:      fake.NewSimpleClientset(),
				clusterRole: makeClusterRole("test"),
			},
			wantErr: false,
		},
		{
			name: "already exist and update",
			args: args{
				client:      fake.NewSimpleClientset(makeClusterRole("test")),
				clusterRole: makeClusterRole("test"),
			},
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset()
					c.PrependReactor("create", "*", errorAction)
					return c
				}(),
				clusterRole: makeClusterRole("test"),
			},
			wantErr: true,
		},
		{
			name: "update error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset(makeClusterRole("test"))
					c.PrependReactor("update", "*", errorAction)
					return c
				}(),
				clusterRole: makeClusterRole("test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateOrUpdateClusterRole(tt.args.client, tt.args.clusterRole); (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateClusterRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateOrUpdateClusterRoleBinding(t *testing.T) {
	type args struct {
		client             kubernetes.Interface
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not exist and create",
			args: args{
				client:             fake.NewSimpleClientset(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: false,
		},
		{
			name: "already exist and update",
			args: args{
				client:             fake.NewSimpleClientset(makeClusterRole("test")),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset()
					c.PrependReactor("create", "*", errorAction)
					return c
				}(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: true,
		},
		{
			name: "update error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset(makeClusterRoleBinding("test"))
					c.PrependReactor("update", "*", errorAction)
					return c
				}(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateOrUpdateClusterRoleBinding(tt.args.client, tt.args.clusterRoleBinding); (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateClusterRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteClusterRole(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		name   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not found",
			args: args{
				client: fake.NewSimpleClientset(),
				name:   "test",
			},
			wantErr: false,
		},
		{
			name: "existed and deleted",
			args: args{
				client: fake.NewSimpleClientset(makeClusterRole("test")),
				name:   "test",
			},
			wantErr: false,
		},
		{
			name: "delete error",
			args: args{
				client: alwaysErrorKubeClient,
				name:   "test",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteClusterRole(tt.args.client, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteClusterRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteClusterRoleBinding(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		name   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not found",
			args: args{
				client: fake.NewSimpleClientset(),
				name:   "test",
			},
			wantErr: false,
		},
		{
			name: "existed and deleted",
			args: args{
				client: fake.NewSimpleClientset(makeClusterRoleBinding("test")),
				name:   "test",
			},
			wantErr: false,
		},
		{
			name: "delete error",
			args: args{
				client: alwaysErrorKubeClient,
				name:   "test",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteClusterRoleBinding(tt.args.client, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteClusterRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateImpersonationRules(t *testing.T) {
	type args struct {
		allSubjects []rbacv1.Subject
	}
	tests := []struct {
		name string
		args args
		want []rbacv1.PolicyRule
	}{
		{
			name: "empty",
			args: args{
				allSubjects: nil,
			},
			want: nil,
		},
		{
			name: "generate success",
			args: args{
				allSubjects: []rbacv1.Subject{
					{Kind: rbacv1.UserKind, Name: "user1"},
					{Kind: rbacv1.UserKind, Name: "user2"},
					{Kind: rbacv1.ServiceAccountKind, Name: "sa1"},
					{Kind: rbacv1.ServiceAccountKind, Name: "sa2"},
					{Kind: rbacv1.GroupKind, Name: "group1"},
					{Kind: rbacv1.GroupKind, Name: "group2"},
				},
			},
			want: []rbacv1.PolicyRule{
				{Verbs: []string{"impersonate"}, Resources: []string{"users"}, APIGroups: []string{""}, ResourceNames: []string{"user1", "user2"}},
				{Verbs: []string{"impersonate"}, Resources: []string{"serviceaccounts"}, APIGroups: []string{""}, ResourceNames: []string{"sa1", "sa2"}},
				{Verbs: []string{"impersonate"}, Resources: []string{"groups"}, APIGroups: []string{""}, ResourceNames: []string{"group1", "group2"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateImpersonationRules(tt.args.allSubjects); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateImpersonationRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsClusterRoleBindingExist(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		name   string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "not found",
			args: args{
				client: fake.NewSimpleClientset(),
				name:   "test",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "exist",
			args: args{
				client: fake.NewSimpleClientset(makeClusterRoleBinding("test")),
				name:   "test",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "get error",
			args: args{
				client: alwaysErrorKubeClient,
				name:   "test",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsClusterRoleBindingExist(tt.args.client, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsClusterRoleBindingExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsClusterRoleBindingExist() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsClusterRoleExist(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		name   string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "not found",
			args: args{
				client: fake.NewSimpleClientset(),
				name:   "test",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "exist",
			args: args{
				client: fake.NewSimpleClientset(makeClusterRole("test")),
				name:   "test",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "get error",
			args: args{
				client: alwaysErrorKubeClient,
				name:   "test",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsClusterRoleExist(tt.args.client, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsClusterRoleExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsClusterRoleExist() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyRuleAPIGroupMatches(t *testing.T) {
	type args struct {
		rule           *rbacv1.PolicyRule
		requestedGroup string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty group",
			args: args{
				rule:           &rbacv1.PolicyRule{},
				requestedGroup: "test",
			},
			want: false,
		},
		{
			name: "group all",
			args: args{
				rule:           &rbacv1.PolicyRule{APIGroups: []string{rbacv1.APIGroupAll}},
				requestedGroup: "test",
			},
			want: true,
		},
		{
			name: "not matched",
			args: args{
				rule:           &rbacv1.PolicyRule{APIGroups: []string{"foo", "bar"}},
				requestedGroup: "test",
			},
			want: false,
		},
		{
			name: "matched",
			args: args{
				rule:           &rbacv1.PolicyRule{APIGroups: []string{"foo", "test"}},
				requestedGroup: "test",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PolicyRuleAPIGroupMatches(tt.args.rule, tt.args.requestedGroup); got != tt.want {
				t.Errorf("PolicyRuleAPIGroupMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyRuleResourceMatches(t *testing.T) {
	type args struct {
		rule              *rbacv1.PolicyRule
		requestedResource string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty resources",
			args: args{
				rule:              &rbacv1.PolicyRule{},
				requestedResource: "test",
			},
			want: false,
		},
		{
			name: "group all",
			args: args{
				rule:              &rbacv1.PolicyRule{Resources: []string{rbacv1.ResourceAll}},
				requestedResource: "test",
			},
			want: true,
		},
		{
			name: "not matched",
			args: args{
				rule:              &rbacv1.PolicyRule{Resources: []string{"foo", "bar"}},
				requestedResource: "test",
			},
			want: false,
		},
		{
			name: "matched",
			args: args{
				rule:              &rbacv1.PolicyRule{Resources: []string{"foo", "test"}},
				requestedResource: "test",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PolicyRuleResourceMatches(tt.args.rule, tt.args.requestedResource); got != tt.want {
				t.Errorf("PolicyRuleResourceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyRuleResourceNameMatches(t *testing.T) {
	type args struct {
		rule          *rbacv1.PolicyRule
		requestedName string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty",
			args: args{
				rule:          &rbacv1.PolicyRule{},
				requestedName: "test",
			},
			want: true,
		},
		{
			name: "not matched",
			args: args{
				rule:          &rbacv1.PolicyRule{ResourceNames: []string{"foo", "bar"}},
				requestedName: "test",
			},
			want: false,
		},
		{
			name: "matched",
			args: args{
				rule:          &rbacv1.PolicyRule{ResourceNames: []string{"foo", "test"}},
				requestedName: "test",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PolicyRuleResourceNameMatches(tt.args.rule, tt.args.requestedName); got != tt.want {
				t.Errorf("PolicyRuleResourceNameMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
