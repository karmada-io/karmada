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

	// Verify that the client is not nil and implements the expected interface
	// The method returns a versioned.Interface which is verified by the non-nil check
	_ = client
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

	// Verify that the factory is not nil and implements the expected interface
	// The method returns a cmdutil.Factory which is verified by the non-nil check
	_ = factory
}
