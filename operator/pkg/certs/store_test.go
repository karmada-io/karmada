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

package certs

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// Helper function to create a new KarmadaCert with given PairName.
func newKarmadaCert(pairName string, certData, keyData []byte) *KarmadaCert {
	return &KarmadaCert{
		PairName: pairName,
		Cert:     certData,
		Key:      keyData,
	}
}

func TestNewCertStore(t *testing.T) {
	store := NewCertStore()
	if store == nil {
		t.Fatalf("expected a non-nil CertStore")
	}
	if len(store.(*KarmadaCertStore).certs) != 0 {
		t.Errorf("expected an empty cert store")
	}
}

func TestAddAndGetCert(t *testing.T) {
	store := NewCertStore()

	cert := newKarmadaCert("testCert", []byte("certData"), []byte("keyData"))
	store.AddCert(cert)

	retrievedCert := store.GetCert("testCert")
	if retrievedCert == nil {
		t.Fatalf("expected to retrieve cert but got nil")
	}
	if string(retrievedCert.Cert) != "certData" {
		t.Errorf("expected certData but got %s", string(retrievedCert.Cert))
	}
	if string(retrievedCert.Key) != "keyData" {
		t.Errorf("expected keyData but got %s", string(retrievedCert.Key))
	}
}

func TestCertList(t *testing.T) {
	cert1Name, cert2Name := "cert1", "cert2"
	store := NewCertStore()

	cert1 := newKarmadaCert(cert1Name, []byte("cert1Data"), []byte("key1Data"))
	cert2 := newKarmadaCert(cert2Name, []byte("cert2Data"), []byte("key2Data"))
	store.AddCert(cert1)
	store.AddCert(cert2)

	certs := store.CertList()
	if len(certs) != 2 {
		t.Errorf("expected 2 certs but got %d", len(certs))
	}
	if cert1 = store.GetCert(cert1Name); cert1 == nil {
		t.Errorf("expected to retrieve %s, but got nil", cert1Name)
	}
	if cert2 = store.GetCert(cert2Name); cert2 == nil {
		t.Errorf("expected to retrieve %s, but got nil", cert2Name)
	}
}

func TestLoadCertFromSecret(t *testing.T) {
	store := NewCertStore()

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"cert1.crt": []byte("cert1CertData"),
			"cert1.key": []byte("cert1KeyData"),
			"cert2.crt": []byte("cert2CertData"),
			"cert2.key": []byte("cert2KeyData"),
		},
	}

	err := store.LoadCertFromSecret(secret)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	cert1 := store.GetCert("cert1")
	if cert1 == nil || string(cert1.Cert) != "cert1CertData" || string(cert1.Key) != "cert1KeyData" {
		t.Errorf("cert1 content is incorrect expected cert %s key %s, got cert %s key %s", "cert1CertData", "cert1KeyData", string(cert1.Cert), string(cert1.Key))
	}

	cert2 := store.GetCert("cert2")
	if cert2 == nil || string(cert2.Cert) != "cert2CertData" || string(cert2.Key) != "cert2KeyData" {
		t.Errorf("cert2 content is incorrect expected cert %s key %s, got cert %s key %s", "cert2CertData", "cert2KeyData", string(cert2.Cert), string(cert2.Key))
	}
}

func TestLoadCertFromSecret_EmptyData(t *testing.T) {
	store := NewCertStore()

	secret := &corev1.Secret{
		Data: map[string][]byte{},
	}

	err := store.LoadCertFromSecret(secret)
	if err == nil {
		t.Error("expected error that cert data is empty")
	}
	if len(store.CertList()) != 0 {
		t.Errorf("expected 0 certs but got %d", len(store.CertList()))
	}
}

func TestLoadCertFromSecret_InvalidFormat(t *testing.T) {
	pairName := "invalid.data"

	store := NewCertStore()

	secret := &corev1.Secret{
		Data: map[string][]byte{
			pairName: []byte("invalidData"),
		},
	}

	err := store.LoadCertFromSecret(secret)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(store.CertList()) != 1 {
		t.Errorf("expected 1 cert but got %d", len(store.CertList()))
	}

	karmadaCert := store.GetCert(pairName)
	if len(karmadaCert.Key) != 0 {
		t.Errorf("expected the cert data content to be empty but got %v", karmadaCert.Cert)
	}
	if len(karmadaCert.Key) != 0 {
		t.Errorf("expected the key data content to be empty but got %v", karmadaCert.Key)
	}
}
