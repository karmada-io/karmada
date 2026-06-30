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
	// Return a fake karmada clientset for testing purposes
	// This allows tests to use the TestFactory without needing a real karmada API server
	return versioned.NewForConfig(t.RESTConfig)
}

// FactoryForMemberCluster returns a cmdutil.Factory for the member cluster
func (t *TestFactory) FactoryForMemberCluster(_ string) (cmdutil.Factory, error) {
	// Return a factory that uses the same configuration as the test factory
	// This allows tests to create factories for member clusters using the test configuration
	return cmdutil.NewFactory(t.RESTConfig), nil
}
