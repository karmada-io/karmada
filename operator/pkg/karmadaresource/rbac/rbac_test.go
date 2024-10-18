/*
Copyright 2024 The Karmada Authors.

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

package rbac

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"

	"github.com/karmada-io/karmada/pkg/util"
)

func TestEnsureKarmadaRBAC(t *testing.T) {
	fakeClient := fakeclientset.NewSimpleClientset()
	err := EnsureKarmadaRBAC(fakeClient)
	if err != nil {
		t.Fatalf("failed to ensure karmada rbac: %v", err)
	}

	actions := fakeClient.Actions()
	if len(actions) != 4 {
		t.Fatalf("expected 4 actions, but got %d", len(actions))
	}
}

func TestGrantKarmadaResourceEditClusterrole(t *testing.T) {
	fakeClient := fakeclientset.NewSimpleClientset()
	err := grantKarmadaResourceEditClusterRole(fakeClient)
	if err != nil {
		t.Fatalf("failed to grant karmada resource edit clusterrole: %v", err)
	}

	// Ensure the expected action (clusterrole creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (clusterrole).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	resourceExpected := "clusterroles"
	if createAction.GetResource().Resource != resourceExpected {
		t.Fatalf("expected action on %s, but got %s", resourceExpected, createAction.GetResource().Resource)
	}

	clusterRole := createAction.GetObject().(*rbacv1.ClusterRole)
	if _, exists := clusterRole.Labels[util.KarmadaSystemLabel]; !exists {
		t.Errorf("expected label %s to exist on the clusterrole, but it does not", util.KarmadaSystemLabel)
	}

	editClusterRoleNameExpected := "karmada-edit"
	if clusterRole.Name != editClusterRoleNameExpected {
		t.Errorf("expected edit cluster role name to be %s, but found %s", editClusterRoleNameExpected, clusterRole.Name)
	}
}

func TestGrantKarmadaResourceViewClusterrole(t *testing.T) {
	fakeClient := fakeclientset.NewSimpleClientset()
	err := grantKarmadaResourceViewClusterRole(fakeClient)
	if err != nil {
		t.Fatalf("failed to grant karmada resource view clusterrole: %v", err)
	}

	// Ensure the expected action (clusterrole creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (clusterrole).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	resourceExpected := "clusterroles"
	if createAction.GetResource().Resource != resourceExpected {
		t.Fatalf("expected action on %s, but got %s", resourceExpected, createAction.GetResource().Resource)
	}

	clusterRole := createAction.GetObject().(*rbacv1.ClusterRole)
	if _, exists := clusterRole.Labels[util.KarmadaSystemLabel]; !exists {
		t.Fatalf("expected label %s to exist on the clusterrole, but it does not", util.KarmadaSystemLabel)
	}

	viewClusterRoleNameExpected := "karmada-view"
	if clusterRole.Name != viewClusterRoleNameExpected {
		t.Fatalf("expected view cluster role name to be %s, but found %s", viewClusterRoleNameExpected, clusterRole.Name)
	}
}
