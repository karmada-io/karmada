// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type PropagationstrategyV1alpha1Interface interface {
	RESTClient() rest.Interface
	PropagationBindingsGetter
	PropagationWorksGetter
}

// PropagationstrategyV1alpha1Client is used to interact with features provided by the propagationstrategy.karmada.io group.
type PropagationstrategyV1alpha1Client struct {
	restClient rest.Interface
}

func (c *PropagationstrategyV1alpha1Client) PropagationBindings(namespace string) PropagationBindingInterface {
	return newPropagationBindings(c, namespace)
}

func (c *PropagationstrategyV1alpha1Client) PropagationWorks(namespace string) PropagationWorkInterface {
	return newPropagationWorks(c, namespace)
}

// NewForConfig creates a new PropagationstrategyV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*PropagationstrategyV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &PropagationstrategyV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new PropagationstrategyV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *PropagationstrategyV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new PropagationstrategyV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *PropagationstrategyV1alpha1Client {
	return &PropagationstrategyV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *PropagationstrategyV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
