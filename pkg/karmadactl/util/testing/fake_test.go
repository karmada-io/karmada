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
	"testing"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

func TestTestFactory_KarmadaClientSet(t *testing.T) {
	tf := NewTestFactory()
	defer tf.Cleanup()

	client, err := tf.KarmadaClientSet()
	if err != nil {
		t.Fatalf("KarmadaClientSet() returned an error: %v", err)
	}

	if client == nil {
		t.Fatal("KarmadaClientSet() returned nil client")
	}

	// Verify that the client implements the expected interface
	if _, ok := client.(versioned.Interface); !ok {
		t.Error("KarmadaClientSet() returned client does not implement versioned.Interface")
	}
}

func TestTestFactory_FactoryForMemberCluster(t *testing.T) {
	tf := NewTestFactory()
	defer tf.Cleanup()

	clusterName := "test-cluster"
	factory, err := tf.FactoryForMemberCluster(clusterName)
	if err != nil {
		t.Fatalf("FactoryForMemberCluster() returned an error: %v", err)
	}

	if factory == nil {
		t.Fatal("FactoryForMemberCluster() returned nil factory")
	}

	// Verify that the factory implements the expected interface
	if _, ok := factory.(cmdutil.Factory); !ok {
		t.Error("FactoryForMemberCluster() returned factory does not implement cmdutil.Factory interface")
	}
}

func TestTestFactory_InterfaceCompliance(t *testing.T) {
	// This test ensures that TestFactory implements the util.Factory interface
	var _ util.Factory = (*TestFactory)(nil)
} 