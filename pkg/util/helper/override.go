package helper

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// IsOverridePolicyExist checks if specific OverridePolicy exist.
func IsOverridePolicyExist(c client.Client, ns string, name string) (bool, error) {
	obj := &policyv1alpha1.OverridePolicy{}
	if err := c.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// IsClusterOverridePolicyExist checks if specific ClusterOverridePolicy exist.
func IsClusterOverridePolicyExist(c client.Client, name string) (bool, error) {
	obj := &policyv1alpha1.ClusterOverridePolicy{}
	if err := c.Get(context.TODO(), client.ObjectKey{Name: name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
