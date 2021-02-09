// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/typed/policy/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakePolicyV1alpha1 struct {
	*testing.Fake
}

func (c *FakePolicyV1alpha1) OverridePolicies(namespace string) v1alpha1.OverridePolicyInterface {
	return &FakeOverridePolicies{c, namespace}
}

func (c *FakePolicyV1alpha1) PropagationBindings(namespace string) v1alpha1.PropagationBindingInterface {
	return &FakePropagationBindings{c, namespace}
}

func (c *FakePolicyV1alpha1) PropagationPolicies(namespace string) v1alpha1.PropagationPolicyInterface {
	return &FakePropagationPolicies{c, namespace}
}

func (c *FakePolicyV1alpha1) Works(namespace string) v1alpha1.WorkInterface {
	return &FakeWorks{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakePolicyV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
