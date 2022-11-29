package testing

import (
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

// TestFactory extends cmdutil.Factory
type TestFactory struct {
	*cmdtesting.TestFactory
}

var _ util.Factory = (*TestFactory)(nil)

// NewTestFactory returns an initialized TestFactory instance
func NewTestFactory() *TestFactory {
	return &TestFactory{
		TestFactory: cmdtesting.NewTestFactory(),
	}
}

// KarmadaClientSet returns a karmada clientset
func (t *TestFactory) KarmadaClientSet() (versioned.Interface, error) {
	// TODO implement me
	panic("implement me")
}

// FactoryForMemberCluster returns a cmdutil.Factory for the member cluster
func (t *TestFactory) FactoryForMemberCluster(clusterName string) (cmdutil.Factory, error) {
	// TODO implement me
	panic("implement me")
}
