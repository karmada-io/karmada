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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
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

// TestInitData contains the configuration and state required to initialize Karmada components.
type TestInitData struct {
	Name                    string
	Namespace               string
	ControlplaneConfigREST  *rest.Config
	DataDirectory           string
	CrdTarballArchive       operatorv1alpha1.CRDTarball
	CustomCertificateConfig operatorv1alpha1.CustomCertificate
	KarmadaVersionRelease   string
	ComponentsUnits         *operatorv1alpha1.KarmadaComponents
	FeatureGatesOptions     map[string]bool
	RemoteClientConnector   clientset.Interface
	KarmadaClientConnector  clientset.Interface
	ControlplaneAddr        string
	Certs                   []*certs.KarmadaCert
}

// Ensure TestInitData implements InitData interface at compile time.
var _ InitData = &TestInitData{}

// GetName returns the name of the current Karmada installation.
func (t *TestInitData) GetName() string {
	return t.Name
}

// GetNamespace returns the namespace of the current Karmada installation.
func (t *TestInitData) GetNamespace() string {
	return t.Namespace
}

// SetControlplaneConfig sets the control plane configuration for Karmada.
func (t *TestInitData) SetControlplaneConfig(config *rest.Config) {
	t.ControlplaneConfigREST = config
}

// ControlplaneConfig returns the control plane configuration.
func (t *TestInitData) ControlplaneConfig() *rest.Config {
	return t.ControlplaneConfigREST
}

// ControlplaneAddress returns the address of the control plane.
func (t *TestInitData) ControlplaneAddress() string {
	return t.ControlplaneAddr
}

// RemoteClient returns the Kubernetes client for remote interactions.
func (t *TestInitData) RemoteClient() clientset.Interface {
	return t.RemoteClientConnector
}

// KarmadaClient returns the Kubernetes client for interacting with Karmada.
func (t *TestInitData) KarmadaClient() clientset.Interface {
	return t.KarmadaClientConnector
}

// DataDir returns the data directory used by Karmada.
func (t *TestInitData) DataDir() string {
	return t.DataDirectory
}

// CrdTarball returns the CRD tarball used for Karmada installation.
func (t *TestInitData) CrdTarball() operatorv1alpha1.CRDTarball {
	return t.CrdTarballArchive
}

// CustomCertificate returns the custom certificate config.
func (t *TestInitData) CustomCertificate() operatorv1alpha1.CustomCertificate {
	return t.CustomCertificateConfig
}

// KarmadaVersion returns the version of Karmada being used.
func (t *TestInitData) KarmadaVersion() string {
	return t.KarmadaVersionRelease
}

// Components returns the Karmada components used in the current installation.
func (t *TestInitData) Components() *operatorv1alpha1.KarmadaComponents {
	return t.ComponentsUnits
}

// FeatureGates returns the feature gates enabled for the current installation.
func (t *TestInitData) FeatureGates() map[string]bool {
	return t.FeatureGatesOptions
}

// AddCert adds a Karmada certificate to the TestInitData.
func (t *TestInitData) AddCert(cert *certs.KarmadaCert) {
	t.Certs = append(t.Certs, cert)
}

// GetCert retrieves a Karmada certificate by its name.
func (t *TestInitData) GetCert(name string) *certs.KarmadaCert {
	for _, cert := range t.Certs {
		parts := strings.Split(cert.CertName(), ".")
		if parts[0] == name {
			return cert
		}
	}
	return nil
}

// CertList returns a list of all Karmada certificates stored in TestInitData.
func (t *TestInitData) CertList() []*certs.KarmadaCert {
	return t.Certs
}

// LoadCertFromSecret loads a Karmada certificate from a Kubernetes secret.
func (t *TestInitData) LoadCertFromSecret(secret *corev1.Secret) error {
	if len(secret.Data) == 0 {
		return fmt.Errorf("cert data is empty")
	}

	// Dummy implementation: load empty certificate.
	cert := &certs.KarmadaCert{}
	t.AddCert(cert)
	return nil
}
