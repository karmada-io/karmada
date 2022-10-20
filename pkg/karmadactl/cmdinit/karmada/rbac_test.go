package karmada

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

var noError = false

func Test_grantProxyPermissionToAdmin(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := grantProxyPermissionToAdmin(client); (err != nil) != noError {
		t.Errorf("grantProxyPermissionToAdmin() error = %v, wantErr %v", err, noError)
	}
}

func Test_grantAccessPermissionToAgent(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := grantAccessPermissionToAgent(client); (err != nil) != noError {
		t.Errorf("grantAccessPermissionToAgent() error = %v, wantErr %v", err, noError)
	}
}
