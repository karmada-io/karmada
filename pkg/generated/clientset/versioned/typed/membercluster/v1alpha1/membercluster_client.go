// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/generated/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type MemberclusterV1alpha1Interface interface {
	RESTClient() rest.Interface
	MemberClustersGetter
}

// MemberclusterV1alpha1Client is used to interact with features provided by the membercluster.karmada.io group.
type MemberclusterV1alpha1Client struct {
	restClient rest.Interface
}

func (c *MemberclusterV1alpha1Client) MemberClusters() MemberClusterInterface {
	return newMemberClusters(c)
}

// NewForConfig creates a new MemberclusterV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*MemberclusterV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &MemberclusterV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new MemberclusterV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *MemberclusterV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new MemberclusterV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *MemberclusterV1alpha1Client {
	return &MemberclusterV1alpha1Client{c}
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
func (c *MemberclusterV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
