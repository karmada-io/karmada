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
	"context"
	"crypto/x509"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewCertTask(t *testing.T) {
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name: "karmada",
		},
		Spec: operatorv1alpha1.KarmadaSpec{
			Components: &operatorv1alpha1.KarmadaComponents{
				Etcd: &operatorv1alpha1.Etcd{
					Local: &operatorv1alpha1.LocalEtcd{},
				},
			},
		},
	}
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "TestNewCertTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name:        "Certs",
				Run:         runCerts,
				Skip:        skipCerts,
				RunSubTasks: true,
				Tasks:       newCertSubTasks(karmada),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			certTask := NewCertTask(karmada)
			err := util.DeepEqualTasks(certTask, test.wantTask)
			if err != nil {
				t.Errorf("unexpected error, got %v", err)
			}
		})
	}
}

func TestRunCerts(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCerts_InvalidTypeAssertion_TypeAssertionFailed",
			runData: MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "certs task invoked with an invalid data struct",
		},
		{
			name:    "RunCerts_ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runCerts(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
		})
	}
}

func TestSkipCerts(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	tests := []struct {
		name     string
		runData  workflow.RunData
		prep     func() error
		cleanup  func() error
		wantErr  bool
		wantSkip bool
		errMsg   string
	}{
		{
			name:     "SkipCerts_InvalidTypeAssertion_TypeAssertionFailed",
			runData:  MyTestData{Data: "test"},
			prep:     func() error { return nil },
			cleanup:  func() error { return nil },
			wantErr:  true,
			wantSkip: false,
			errMsg:   "certs task invoked with an invalid data struct",
		},
		{
			name: "SkipCerts_ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{
				Name:                  "karmada-demo",
				Namespace:             "test",
				RemoteClientConnector: client,
			},
			prep:     func() error { return nil },
			cleanup:  func() error { return nil },
			wantErr:  false,
			wantSkip: false,
		},
		{
			name: "SkipCerts_WithEmptySecretData_ErrorReturned",
			runData: &TestInitData{
				Name:                  "karmada-demo",
				Namespace:             "test",
				RemoteClientConnector: client,
			},
			prep: func() error {
				_, err := client.CoreV1().Secrets("test").Create(
					context.TODO(), &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.KarmadaCertSecretName("karmada-demo"),
							Namespace: "test",
						},
						Data: map[string][]byte{},
					},
					metav1.CreateOptions{},
				)
				return err
			},
			cleanup: func() error {
				err := client.CoreV1().Secrets("test").Delete(
					context.TODO(), util.KarmadaCertSecretName("karmada-demo"),
					metav1.DeleteOptions{},
				)
				if err != nil {
					return fmt.Errorf("failed to delete %s secret", "test")
				}
				return nil
			},
			wantErr:  true,
			wantSkip: false,
		},
		{
			name: "SkipCerts_SecertCertDataExist_CertsSkipped",
			runData: &TestInitData{
				Name:                  "karmada-demo",
				Namespace:             "test",
				RemoteClientConnector: client,
			},
			prep: func() error {
				var err error
				_, err = client.CoreV1().Secrets("test").Create(
					context.TODO(), &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.KarmadaCertSecretName("karmada-demo"),
							Namespace: "test",
						},
						Data: map[string][]byte{
							"ca.crt": []byte("ca-crt-data"),
							"ca.key": []byte("ca-key-data"),
						},
					},
					metav1.CreateOptions{},
				)
				return err
			},
			cleanup: func() error {
				err := client.CoreV1().Secrets("test").Delete(
					context.TODO(), util.KarmadaCertSecretName("karmada-demo"),
					metav1.DeleteOptions{},
				)
				if err != nil {
					return fmt.Errorf("failed to delete %s secret", "test")
				}
				return nil
			},
			wantErr:  false,
			wantSkip: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.prep()
			if err != nil {
				t.Errorf("failed to prep test: %v", err)
			}

			skip, err := skipCerts(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if skip != test.wantSkip {
				t.Errorf("expected %t skip, but got %t", test.wantSkip, skip)
			}
			err = test.cleanup()
			if err != nil {
				t.Errorf("failed to clean up test: %v", err)
			}
		})
	}
}

func TestRunCATask(t *testing.T) {
	tests := []struct {
		name    string
		kc      *certs.CertConfig
		runData workflow.RunData
		prep    func(*certs.CertConfig) error
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCATask_InvalidTypeAssertion_TypeAssertionFailed",
			kc:      certs.KarmadaCertRootCA(),
			runData: MyTestData{Data: "test"},
			prep:    func(*certs.CertConfig) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "certs task invoked with an invalid data struct",
		},
		{
			name: "RunCATask_WithNonCACertificate_CACertificateExpected",
			kc:   certs.KarmadaCertAdmin(),
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			prep:    func(*certs.CertConfig) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  fmt.Sprintf("this function should only be used for CAs, but cert %s has CA %s", constants.KarmadaCertAndKeyName, constants.CaCertAndKeyName),
		},
		{
			name: "RunCATask_WithEd25519UnsupportedPublicKeyAlgorithm_UnsupportedKeyType",
			kc:   certs.KarmadaCertRootCA(),
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			prep: func(cc *certs.CertConfig) error {
				cc.PublicKeyAlgorithm = x509.Ed25519
				return nil
			},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  fmt.Sprintf("unsupported key type: %T", x509.Ed25519),
		},
		{
			name: "RunCATask_GenerateCACertificate_CACertificateSuccessfullyGenerated",
			kc:   certs.KarmadaCertRootCA(),
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			prep: func(*certs.CertConfig) error { return nil },
			verify: func(rd workflow.RunData) error {
				certData := rd.(*TestInitData).CertList()
				if len(certData) != 1 {
					return fmt.Errorf("expected cert store to contain the generated CA certificate but found %d certs", len(certData))
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.kc); err != nil {
				t.Errorf("failed to prep cert config data: %v", err)
			}
			caTask := runCATask(test.kc)
			err := caTask(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.runData); err != nil {
				t.Errorf("failed to verify run data: %v", err)
			}
		})
	}
}

func TestRunCertTask(t *testing.T) {
	tests := []struct {
		name    string
		kc      *certs.CertConfig
		caCert  *certs.CertConfig
		runData workflow.RunData
		prep    func(*certs.CertConfig, *certs.CertConfig, workflow.RunData) error
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCertTask_InvalidTypeAssertion_TypeAssertionFailed",
			kc:      certs.KarmadaCertAdmin(),
			runData: MyTestData{Data: "test"},
			prep:    func(*certs.CertConfig, *certs.CertConfig, workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			caCert:  nil,
			wantErr: true,
			errMsg:  "certs task invoked with an invalid data struct",
		},
		{
			name: "RunCertTask_NilCACert_CACertIsNil",
			kc:   certs.KarmadaCertAdmin(),
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			prep:    func(*certs.CertConfig, *certs.CertConfig, workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			caCert:  nil,
			wantErr: true,
			errMsg:  fmt.Sprintf("unexpected empty ca cert for %s", constants.KarmadaCertAndKeyName),
		},
		{
			name: "RunCertTask_MismatchCAName_CANameIsMismatch",
			kc:   certs.KarmadaCertAdmin(),
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			prep:   func(*certs.CertConfig, *certs.CertConfig, workflow.RunData) error { return nil },
			verify: func(workflow.RunData) error { return nil },
			caCert: &certs.CertConfig{
				Name: "invalid",
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("mismatched CA name: expected %s but got %s", constants.CaCertAndKeyName, "invalid"),
		},
		{
			name:   "RunCertTask_CreateCertAndKeyFileWithCA_SuccessfullyGeneratedCertificate",
			kc:     certs.KarmadaCertAdmin(),
			caCert: certs.KarmadaCertRootCA(),
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{},
				},
			},
			prep: func(ca *certs.CertConfig, _ *certs.CertConfig, rd workflow.RunData) error {
				newCA, err := certs.NewCertificateAuthority(ca)
				if err != nil {
					return fmt.Errorf("failed to create certificate authority: %v", err)
				}
				rd.(*TestInitData).AddCert(newCA)
				return nil
			},
			verify: func(rd workflow.RunData) error {
				certData := rd.(*TestInitData).CertList()
				if len(certData) != 2 {
					return fmt.Errorf("expected cert store to contain the Certificate Authority and the associated certificate but found %d certs", len(certData))
				}
				if rd.(*TestInitData).GetCert(constants.CaCertAndKeyName) == nil {
					return fmt.Errorf("expected %s Karmada Root CA to exist in the certificates store", constants.CaCertAndKeyName)
				}
				if rd.(*TestInitData).GetCert(constants.KarmadaCertAndKeyName) == nil {
					return fmt.Errorf("expected %s karmada admin certificate to exist in the certificate store", constants.KarmadaCertAndKeyName)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.caCert, test.kc, test.runData); err != nil {
				t.Errorf("failed to prep cert config data: %v", err)
			}
			certTask := runCertTask(test.kc, test.caCert)
			err := certTask(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.runData); err != nil {
				t.Errorf("failed to verify run data: %v", err)
			}
		})
	}
}
