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

package karmada

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func Test_grantProxyPermissionToAdmin(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := grantProxyPermissionToAdmin(client); err != nil {
		t.Errorf("grantProxyPermissionToAdmin() expected no error, but got err: %v", err)
	}
}

func Test_grantAccessPermissionToAgent(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := grantAccessPermissionToAgentRBACGenerator(client); err != nil {
		t.Errorf("grantAccessPermissionToAgentRBACGenerator() expected no error, but got err: %v", err)
	}
}

func Test_grantKarmadaPermissionToViewClusterRole(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := grantKarmadaPermissionToViewClusterRole(client); err != nil {
		t.Errorf("grantKarmadaPermissionToViewClusterRole() expected no error, but got err: %v", err)
	}
}

func Test_grantKarmadaPermissionToEditClusterRole(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := grantKarmadaPermissionToEditClusterRole(client); err != nil {
		t.Errorf("grantKarmadaPermissionToEditClusterRole() expected no error, but got err: %v", err)
	}
}
