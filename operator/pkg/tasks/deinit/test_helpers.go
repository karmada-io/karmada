/*
Copyright 2024 The Karmada Authors.

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

package tasks

import (
	clientset "k8s.io/client-go/kubernetes"
)

// TestInterface defines the interface for retrieving test data.
type TestInterface interface {
	// Get returns the data from the test instance.
	Get() string
}

// MyTestData is a struct that implements the TestInterface.
type MyTestData struct {
	Data string
}

// Get returns the data stored in the MyTestData struct.
func (m *MyTestData) Get() string {
	return m.Data
}

// TestDeInitData contains the configuration and state required to deinitialize Karmada components.
type TestDeInitData struct {
	name         string
	namespace    string
	remoteClient clientset.Interface
}

// Ensure TestDeInitData implements InitData interface at compile time.
var _ DeInitData = &TestDeInitData{}

// GetName returns the name of the current Karmada installation.
func (t *TestDeInitData) GetName() string {
	return t.name
}

// GetNamespace returns the namespace of the current Karmada installation.
func (t *TestDeInitData) GetNamespace() string {
	return t.namespace
}

// RemoteClient returns the Kubernetes client for remote interactions.
func (t *TestDeInitData) RemoteClient() clientset.Interface {
	return t.remoteClient
}
