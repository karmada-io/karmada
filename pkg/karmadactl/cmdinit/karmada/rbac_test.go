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
	if err := grantAccessPermissionToAgent(client); err != nil {
		t.Errorf("grantAccessPermissionToAgent() expected no error, but got err: %v", err)
	}
}
