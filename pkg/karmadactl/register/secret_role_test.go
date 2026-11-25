/*
Copyright 2026 The Karmada Authors.

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

package register

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestEnsureAgentSecretRoleMigration(t *testing.T) {
	old := GenerateSecretAccessRole("member1", "karmada-cluster")
	old.ResourceVersion = "7"
	old.Labels = map[string]string{"custom": "preserved"}
	old.Rules[0].Verbs = []string{"get", "patch"}
	old.Rules = append(old.Rules, rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}})
	work := GenerateWorkAccessRole("member1")
	work.Labels = map[string]string{"work": "preserved"}
	client := fake.NewSimpleClientset(old.DeepCopy(), work.DeepCopy())
	opts := &CommandRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", rbacResources: GenerateRBACResources("member1", "karmada-cluster")}
	if err := opts.ensureAgentRBACResourcesExistInControlPlane(client); err != nil {
		t.Fatal(err)
	}
	got, err := client.RbacV1().Roles(old.Namespace).Get(context.Background(), old.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := old.DeepCopy()
	want.Rules = append(want.Rules, rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"delete"}, ResourceNames: []string{"member1", "member1-impersonator"}})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated Role = %#v, want %#v", got, want)
	}
	gotWork, err := client.RbacV1().Roles(work.Namespace).Get(context.Background(), work.Name, metav1.GetOptions{})
	if err != nil || !reflect.DeepEqual(gotWork, work) {
		t.Fatalf("work Role changed: %#v, err: %v", gotWork, err)
	}
	client.ClearActions()
	if err := opts.ensureAgentRBACResourcesExistInControlPlane(client); err != nil {
		t.Fatal(err)
	}
	for _, action := range client.Actions() {
		if action.GetResource().Resource == "roles" && (action.GetVerb() == "patch" || action.GetVerb() == "update") {
			t.Fatalf("already migrated Role written again: %v", action)
		}
	}
}

func TestEnsureAgentSecretRoleAlreadyCovered(t *testing.T) {
	for _, rule := range []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"delete"}, ResourceNames: []string{"member1", "member1-impersonator"}},
		{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
	} {
		old := GenerateSecretAccessRole("member1", "karmada-cluster")
		old.Rules = []rbacv1.PolicyRule{rule}
		client := fake.NewSimpleClientset(old)
		opts := &CommandRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", rbacResources: GenerateRBACResources("member1", "karmada-cluster")}
		if err := opts.ensureAgentRBACResourcesExistInControlPlane(client); err != nil {
			t.Fatal(err)
		}
		for _, action := range client.Actions() {
			if action.GetResource().Resource == "roles" && (action.GetVerb() == "patch" || action.GetVerb() == "update") {
				t.Fatalf("Role with sufficient permissions written: %v", action)
			}
		}
	}
}

func TestEnsureAgentSecretRoleErrors(t *testing.T) {
	for _, verb := range []string{"get", "patch"} {
		t.Run(verb, func(t *testing.T) {
			old := GenerateSecretAccessRole("member1", "karmada-cluster")
			old.Rules[0].Verbs = []string{"get", "patch"}
			client := fake.NewSimpleClientset(old)
			want := errors.New("injected " + verb + " failure")
			client.PrependReactor(verb, "roles", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, want
			})
			opts := &CommandRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", rbacResources: GenerateRBACResources("member1", "karmada-cluster")}
			if err := opts.ensureAgentRBACResourcesExistInControlPlane(client); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestEnsureAgentSecretRoleConflict(t *testing.T) {
	old := GenerateSecretAccessRole("member1", "karmada-cluster")
	old.Rules[0].Verbs = []string{"get", "patch"}
	old.ResourceVersion = "7"
	client := fake.NewSimpleClientset(old.DeepCopy())
	patches := 0
	client.PrependReactor("patch", "roles", func(action clienttesting.Action) (bool, runtime.Object, error) {
		patches++
		var payload struct {
			Metadata metav1.ObjectMeta `json:"metadata"`
		}
		if err := json.Unmarshal(action.(clienttesting.PatchAction).GetPatch(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Metadata.ResourceVersion != old.ResourceVersion {
			t.Fatalf("patch resourceVersion = %q, want %q", payload.Metadata.ResourceVersion, old.ResourceVersion)
		}
		if patches == 1 {
			old.ResourceVersion = "8"
			old.Labels = map[string]string{"concurrent": "preserved"}
			old.Rules = append(old.Rules, rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}})
			if err := client.Tracker().Update(rbacv1.SchemeGroupVersion.WithResource("roles"), old.DeepCopy(), old.Namespace); err != nil {
				t.Fatal(err)
			}
			return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "roles"}, old.Name, errors.New("concurrent edit"))
		}
		return false, nil, nil
	})
	opts := &CommandRegisterOption{ClusterName: "member1", ClusterNamespace: "karmada-cluster", rbacResources: GenerateRBACResources("member1", "karmada-cluster")}
	if err := opts.ensureAgentRBACResourcesExistInControlPlane(client); err != nil {
		t.Fatal(err)
	}
	if patches != 2 {
		t.Fatalf("patch attempts = %d, want 2", patches)
	}
	got, err := client.RbacV1().Roles(old.Namespace).Get(context.Background(), old.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Labels, old.Labels) || !reflect.DeepEqual(got.Rules[:len(old.Rules)], old.Rules) {
		t.Fatalf("concurrent changes lost: %#v", got)
	}
}

func TestGenerateSecretAccessRole(t *testing.T) {
	role := GenerateSecretAccessRole("member1", "karmada-cluster")
	if role.Name != "system:karmada:member1:agent-secret" || role.Namespace != "karmada-cluster" {
		t.Fatalf("unexpected Role identity: %s/%s", role.Namespace, role.Name)
	}
	want := []rbacv1.PolicyRule{
		{
			Verbs:         []string{"get", "patch", "delete"},
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: []string{"member1", "member1-impersonator"},
		},
		{
			Verbs:     []string{"create"},
			APIGroups: []string{""},
			Resources: []string{"secrets"},
		},
	}
	if !reflect.DeepEqual(role.Rules, want) {
		t.Errorf("Secret permissions = %#v, want scoped credential cleanup and creation: %#v", role.Rules, want)
	}
}
