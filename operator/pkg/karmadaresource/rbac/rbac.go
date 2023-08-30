package rbac

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
)

// EnsureKarmadaRBAC create karmada resource view and edit clusterrole
func EnsureKarmadaRBAC(client clientset.Interface) error {
	if err := grantKarmadaResourceViewClusterrole(client); err != nil {
		return err
	}
	return grantKarmadaResourceEditClusterrole(client)
}

func grantKarmadaResourceViewClusterrole(client clientset.Interface) error {
	viewClusterrole := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(KarmadaResourceViewClusterRole), viewClusterrole); err != nil {
		return fmt.Errorf("err when decoding Karmada view Clusterrole: %w", err)
	}
	return apiclient.CreateOrUpdateClusterRole(client, viewClusterrole)
}

func grantKarmadaResourceEditClusterrole(client clientset.Interface) error {
	editClusterrole := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(KarmadaResourceEditClusterRole), editClusterrole); err != nil {
		return fmt.Errorf("err when decoding Karmada edit Clusterrole: %w", err)
	}
	return apiclient.CreateOrUpdateClusterRole(client, editClusterrole)
}
