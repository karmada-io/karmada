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
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

var (
	expectedAPIServerAltDNSNames = []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		fmt.Sprintf("*.%s.svc.cluster.local", constants.KarmadaSystemNamespace),
		fmt.Sprintf("*.%s.svc", constants.KarmadaSystemNamespace),
	}
	expectedAPIServerAltIPs = []net.IP{net.IPv4(127, 0, 0, 1)}
)

func TestCertConfig_defaultPublicKeyAlgorithm(t *testing.T) {
	c := &CertConfig{}
	c.defaultPublicKeyAlgorithm()

	if c.PublicKeyAlgorithm != x509.RSA {
		t.Errorf("expected PublicKeyAlgorithm to be RSA, got %v", c.PublicKeyAlgorithm)
	}
}

func TestCertConfig_defaultNotAfter(t *testing.T) {
	c := &CertConfig{}
	c.defaultNotAfter()

	if c.NotAfter == nil {
		t.Error("expected NotAfter to be set, but it was nil")
	}

	if !c.NotAfter.After(time.Now()) {
		t.Errorf("expected NotAfter to be a future time, got %v", c.NotAfter)
	}

	expectedTime := time.Now().Add(constants.CertificateValidity).UTC()
	if c.NotAfter.Sub(expectedTime) > time.Minute {
		t.Errorf("NotAfter time is too far from expected, got %v, expected %v", c.NotAfter, expectedTime)
	}
}

func TestKarmadaCertRootCA(t *testing.T) {
	certConfig := KarmadaCertRootCA()

	expectedCommonName := "karmada"

	if certConfig.Name != constants.CaCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.CaCertAndKeyName, certConfig.Name)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}
}

func TestKarmadaCertAdmin(t *testing.T) {
	certConfig := KarmadaCertAdmin()

	err := certConfig.AltNamesMutatorFunc(&AltNamesMutatorConfig{}, certConfig)
	if err != nil {
		t.Fatalf("AltNamesMutatorFunc() returned error: %v", err)
	}

	expectedCommonName := "system:admin"
	expectedOrganization := []string{"system:masters"}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}

	if certConfig.Name != constants.KarmadaCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.KarmadaCertAndKeyName, certConfig.Name)
	}

	if certConfig.CAName != constants.CaCertAndKeyName {
		t.Errorf("expected CAName to be %s, got %s", constants.CaCertAndKeyName, certConfig.CAName)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}

	if !util.ContainsAllValues(certConfig.Config.Organization, expectedOrganization) {
		t.Errorf("expected Organization to contain %v, got %v", expectedOrganization, certConfig.Config.Organization)
	}

	if !util.ContainsAllValues(certConfig.Config.Usages, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageServerAuth and ExtKeyUsageClientAuth for mutual TLS, got %v", certConfig.Config.Usages)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.DNSNames, expectedAPIServerAltDNSNames) {
		t.Errorf("expected DNSNames to contain %v, got %v", expectedAPIServerAltDNSNames, certConfig.Config.AltNames.DNSNames)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.IPs, expectedAPIServerAltIPs) {
		t.Errorf("expected IPs to contain %v, got %v", expectedAPIServerAltIPs, certConfig.Config.AltNames.IPs)
	}
}

func TestKarmadaCertApiserver(t *testing.T) {
	certConfig := KarmadaCertApiserver()

	err := certConfig.AltNamesMutatorFunc(&AltNamesMutatorConfig{}, certConfig)
	if err != nil {
		t.Fatalf("AltNamesMutatorFunc() returned error: %v", err)
	}

	expectedCommonName := "karmada-apiserver"
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	if certConfig.Name != constants.ApiserverCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.ApiserverCertAndKeyName, certConfig.Name)
	}

	if certConfig.CAName != constants.CaCertAndKeyName {
		t.Errorf("expected CAName to be %s, got %s", constants.CaCertAndKeyName, certConfig.CAName)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}

	if !util.ContainsAllValues(certConfig.Config.Usages, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageServerAuth, got %v", certConfig.Config.Usages)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.DNSNames, expectedAPIServerAltDNSNames) {
		t.Errorf("expected DNSNames to contain %v, got %v", expectedAPIServerAltDNSNames, certConfig.Config.AltNames.DNSNames)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.IPs, expectedAPIServerAltIPs) {
		t.Errorf("expected IPs to contain %v, got %v", expectedAPIServerAltIPs, certConfig.Config.AltNames.IPs)
	}
}

func TestKarmadaCertClient(t *testing.T) {
	newControlPlaneAddress := "192.168.1.101"
	newCertSAN := "10.0.0.15"

	certConfig := KarmadaCertClient()

	err := certConfig.AltNamesMutatorFunc(&AltNamesMutatorConfig{
		ControlplaneAddress: newControlPlaneAddress,
		Components: &v1alpha1.KarmadaComponents{
			KarmadaAPIServer: &v1alpha1.KarmadaAPIServer{
				CertSANs: []string{newCertSAN},
			},
		},
	}, certConfig)
	if err != nil {
		t.Fatalf("AltNamesMutatorFunc() returned error: %v", err)
	}

	expectedCertName := "karmada-client"
	expectedCommonName := "system:admin"
	expectedOrganization := []string{"system:masters"}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	newIPs := []net.IP{net.ParseIP(newControlPlaneAddress), net.ParseIP(newCertSAN)}
	expectedNewIPs := append(newIPs, expectedAPIServerAltIPs...)

	if certConfig.Name != expectedCertName {
		t.Errorf("expected Name to be %s, got %s", expectedCertName, certConfig.Name)
	}

	if certConfig.CAName != constants.CaCertAndKeyName {
		t.Errorf("expected CAName to be %s, got %s", constants.CaCertAndKeyName, certConfig.CAName)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}

	if !util.ContainsAllValues(certConfig.Config.Organization, expectedOrganization) {
		t.Errorf("expected Organization to contain %v, got %v", expectedOrganization, certConfig.Config.Organization)
	}

	if !util.ContainsAllValues(certConfig.Config.Usages, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageClientAuth, got %v", certConfig.Config.Usages)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.DNSNames, expectedAPIServerAltDNSNames) {
		t.Errorf("expected DNSNames to contain %v, got %v", expectedAPIServerAltDNSNames, certConfig.Config.AltNames.DNSNames)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.IPs, expectedNewIPs) {
		t.Errorf("expected IPs to contain %v, got %v", expectedNewIPs, certConfig.Config.AltNames.IPs)
	}
}

func TestKarmadaCertFrontProxyCA(t *testing.T) {
	certConfig := KarmadaCertFrontProxyCA()

	expectedCommonName := "front-proxy-ca"

	if certConfig.Name != constants.FrontProxyCaCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.FrontProxyCaCertAndKeyName, certConfig.Name)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}
}

func TestKarmadaCertFrontProxyClient(t *testing.T) {
	certConfig := KarmadaCertFrontProxyClient()

	expectedCommonName := "front-proxy-client"
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	if certConfig.Name != constants.FrontProxyClientCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.FrontProxyClientCertAndKeyName, certConfig.Name)
	}

	if certConfig.CAName != constants.FrontProxyCaCertAndKeyName {
		t.Errorf("expected CAName to be %s, got %s", constants.FrontProxyCaCertAndKeyName, certConfig.Name)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}

	if !util.ContainsAllValues(certConfig.Config.Usages, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageClientAuth, got %v", certConfig.Config.Usages)
	}
}

func TestKarmadaCertEtcdCA(t *testing.T) {
	certConfig := KarmadaCertEtcdCA()

	expectedCommonName := "karmada-etcd-ca"

	if certConfig.Name != constants.EtcdCaCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.EtcdCaCertAndKeyName, certConfig.Name)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}
}

func TestKarmadaCertEtcdServer(t *testing.T) {
	certConfig := KarmadaCertEtcdServer()

	cfg := &AltNamesMutatorConfig{Namespace: constants.KarmadaSystemNamespace}
	err := certConfig.AltNamesMutatorFunc(cfg, certConfig)
	if err != nil {
		t.Fatalf("AltNamesMutatorFunc() returned error: %v", err)
	}

	expectedCommonName := "karmada-etcd-server"
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	expectedDNSNames := []string{
		"localhost",
		fmt.Sprintf("%s.%s.svc.cluster.local", util.KarmadaEtcdClientName(cfg.Name), cfg.Namespace),
		fmt.Sprintf("*.%s.%s.svc.cluster.local", util.KarmadaEtcdName(cfg.Name), cfg.Namespace),
	}
	expectedIPs := []net.IP{net.IPv4(127, 0, 0, 1)}

	if certConfig.Name != constants.EtcdServerCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.EtcdServerCertAndKeyName, certConfig.Name)
	}

	if certConfig.CAName != constants.EtcdCaCertAndKeyName {
		t.Errorf("expected CAName to be %s, got %s", constants.EtcdCaCertAndKeyName, certConfig.CAName)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}

	if !util.ContainsAllValues(certConfig.Config.Usages, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageServerAuth and ExtKeyUsageClientAuth, got %v", certConfig.Config.Usages)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.DNSNames, expectedDNSNames) {
		t.Errorf("expected DNSNames to contain %v, got %v", expectedDNSNames, certConfig.Config.AltNames.DNSNames)
	}

	if !util.ContainsAllValues(certConfig.Config.AltNames.IPs, expectedIPs) {
		t.Errorf("expected IPs to contain %v, got %v", expectedIPs, certConfig.Config.AltNames.IPs)
	}
}

func TestKarmadaCertEtcdClient(t *testing.T) {
	certConfig := KarmadaCertEtcdClient()

	expectedCommonName := "karmada-etcd-client"
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}

	if certConfig.Name != constants.EtcdClientCertAndKeyName {
		t.Errorf("expected Name to be %s, got %s", constants.EtcdClientCertAndKeyName, certConfig.Name)
	}

	if certConfig.CAName != constants.EtcdCaCertAndKeyName {
		t.Errorf("expected CAName to be %s, got %s", constants.EtcdCaCertAndKeyName, certConfig.CAName)
	}

	if certConfig.Config.CommonName != expectedCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCommonName, certConfig.Config.CommonName)
	}

	if !util.ContainsAllValues(certConfig.Config.Usages, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageServerAuth and ExtKeyUsageClientAuth, got %v", certConfig.Config.Usages)
	}
}

func TestGeneratePrivateKey_ECDSAKey_ShouldReturnValidECDSAKey(t *testing.T) {
	key, err := GeneratePrivateKey(x509.ECDSA)
	if err != nil {
		t.Fatalf("GeneratePrivateKey() returned error: %v", err)
	}

	// Check that the key is of type *ecdsa.PrivateKey.
	if _, ok := key.(*ecdsa.PrivateKey); !ok {
		t.Errorf("GeneratePrivateKey() returned key of type %T, expected *ecdsa.PrivateKey", key)
	}

	// Verify that the elliptic curve is P-256 (secp256r1).
	ecdsaKey := key.(*ecdsa.PrivateKey)
	if ecdsaKey.Curve != elliptic.P256() {
		t.Errorf("GeneratePrivateKey() returned ECDSA key with elliptic curve %v, expected P-256", ecdsaKey.Curve)
	}
}

func TestGeneratePrivateKey_RSAKey_ShouldReturnValidRSAKey(t *testing.T) {
	expectedRSAKeySizeInBytes := rsaKeySize / 8

	key, err := GeneratePrivateKey(x509.RSA)
	if err != nil {
		t.Fatalf("GeneratePrivateKey() returned error: %v", err)
	}

	// Check that the key is of type *rsa.PrivateKey.
	if _, ok := key.(*rsa.PrivateKey); !ok {
		t.Errorf("GeneratePrivateKey() returned key of type %T, expected *rsa.PrivateKey", key)
	}

	// Verify that the key size is correct.
	rsaKey := key.(*rsa.PrivateKey)
	if rsaKey.Size() != expectedRSAKeySizeInBytes {
		t.Errorf("GeneratePrivateKey() returned RSA key of size %d, expected %d", rsaKey.Size()*8, rsaKeySize)
	}
}

func TestGeneratePrivateKey_InvalidKeyType_ShouldReturnError(t *testing.T) {
	_, err := GeneratePrivateKey(x509.DSA)
	if err == nil {
		t.Error("GeneratePrivateKey() expected error for unsupported key type 'DSA', got nil")
	}
}

func TestEncodeCertPEM_CorrectEncoding(t *testing.T) {
	cert := &x509.Certificate{
		Raw: []byte("dummy_certificate_data"),
	}

	// Base64 encoding of the dummy certificate data.
	base64EncodedCert := base64.StdEncoding.EncodeToString(cert.Raw)

	// Construct the expected PEM format.
	expectedPEM := fmt.Sprintf(
		"-----BEGIN %s-----\n%s\n-----END %s-----\n",
		CertificateBlockType,
		base64EncodedCert,
		CertificateBlockType,
	)
	got := EncodeCertPEM(cert)

	// Check if the encoded PEM matches the expected PEM format.
	if !bytes.Equal(got, []byte(expectedPEM)) {
		t.Errorf("EncodeCertPEM() = %s; want %s", got, expectedPEM)
	}
}

func TestNewCertificateAuthority(t *testing.T) {
	cc := &CertConfig{
		Name:               "test-ca",
		CAName:             "test-ca",
		PublicKeyAlgorithm: x509.RSA,
		Config: certutil.Config{
			CommonName:   "test-ca",
			Organization: []string{"test-org"},
		},
	}

	cert, err := NewCertificateAuthority(cc)
	if err != nil {
		t.Fatalf("NewCertificateAuthority() returned an error: %v", err)
	}

	if cert == nil {
		t.Fatal("NewCertificateAuthority() returned nil cert")
	}

	if cert.pairName != cc.Name {
		t.Errorf("expected pairName to be %s, got %s", cc.Name, cert.pairName)
	}

	if cert.caName != cc.CAName {
		t.Errorf("expected caName to be %s, got %s", cc.CAName, cert.caName)
	}

	if cert.cert == nil {
		t.Error("expected cert to be non-nil")
	}

	if cert.key == nil {
		t.Error("expected key to be non-nil")
	}

	block, _ := pem.Decode(cert.cert)
	if block == nil || block.Type != CertificateBlockType {
		t.Errorf("expected PEM block type to be %s, got %v", CertificateBlockType, block)
	}
}

func TestParsePrivateKeyPEM_ValidRSAKey(t *testing.T) {
	key, err := GeneratePrivateKey(x509.RSA)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	keyPEM, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		t.Errorf("unable to marshal private key to PEM, err: %v", err)
	}

	signer, err := ParsePrivateKeyPEM(keyPEM)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM() returned an error: %v", err)
	}

	if _, ok := signer.(*rsa.PrivateKey); !ok {
		t.Errorf("ParsePrivateKeyPEM() returned key of type %T, expected *rsa.PrivateKey", signer)
	}
}

func TestParsePrivateKeyPEM_InvalidKeyType(t *testing.T) {
	invalidKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "INVALID PRIVATE KEY",
		Bytes: []byte("invalid"),
	})

	signer, err := ParsePrivateKeyPEM(invalidKeyPEM)
	if err == nil {
		t.Errorf("ParsePrivateKeyPEM() expected error for unsupported key type, got nil")
	}
	if signer != nil {
		t.Errorf("ParsePrivateKeyPEM() expected nil signer, got %v", signer)
	}
}

func TestCreateCertAndKeyFilesWithCA(t *testing.T) {
	caName, caCommonName := "test-ca", "test-ca"
	caCertConfig := &CertConfig{
		Name:               caName,
		CAName:             caName,
		PublicKeyAlgorithm: x509.RSA,
		Config: certutil.Config{
			CommonName:   caCommonName,
			Organization: []string{"test-org"},
		},
	}

	caCert, err := NewCertificateAuthority(caCertConfig)
	if err != nil {
		t.Fatalf("NewCertificateAuthority() returned an error: %v", err)
	}

	certName, certCommonName := "test-cert", "test-common-name"
	certConfig := &CertConfig{
		Name:               certName,
		CAName:             caName,
		PublicKeyAlgorithm: x509.RSA,
		Config: certutil.Config{
			CommonName:   certCommonName,
			Organization: []string{"test-org"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
	}

	cert, err := CreateCertAndKeyFilesWithCA(certConfig, caCert.CertData(), caCert.KeyData())
	if err != nil {
		t.Fatalf("CreateCertAndKeyFilesWithCA() returned an error: %v", err)
	}

	if cert == nil {
		t.Fatal("CreateCertAndKeyFilesWithCA() returned nil cert")
	}

	if cert.cert == nil || cert.key == nil {
		t.Error("Expected cert and key to be non-nil")
	}

	if cert.pairName != certConfig.Name {
		t.Errorf("expected pairName to be %s, got %s", certConfig.Name, cert.pairName)
	}

	if cert.caName != certConfig.CAName {
		t.Errorf("expected caName to be %s, got %s", certConfig.CAName, cert.caName)
	}

	block, _ := pem.Decode(cert.cert)
	if block == nil || block.Type != CertificateBlockType {
		t.Errorf("expected PEM block type to be %s, got %v", CertificateBlockType, block)
	}
}

func TestNewSignedCert_Success(t *testing.T) {
	// Create a CA certificate.
	caName, caCommonName := "test-ca", "test-ca"
	caCertConfig := &CertConfig{
		Name:               caName,
		CAName:             caName,
		PublicKeyAlgorithm: x509.RSA,
		Config: certutil.Config{
			CommonName:   caCommonName,
			Organization: []string{"test-org"},
		},
	}

	caKarmadaCert, err := NewCertificateAuthority(caCertConfig)
	if err != nil {
		t.Fatalf("NewCertificateAuthority() returned an error: %v", err)
	}

	caCerts, err := certutil.ParseCertsPEM(caKarmadaCert.CertData())
	if err != nil {
		t.Error(err)
	}
	caCert := caCerts[0]

	caKey, err := ParsePrivateKeyPEM(caKarmadaCert.key)
	if err != nil {
		t.Error(err)
	}

	// Create a key pair for the certificate.
	key, err := GeneratePrivateKey(x509.RSA)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Create a CertConfig for the test certificate.
	expectedCertCommonName := "test-cert"
	expectedCertOrganization := []string{"test-org"}
	expectedCertDNSNames := []string{"localhost"}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	certNotAfter := time.Now().Add(constants.CertificateValidity)
	cc := &CertConfig{
		Config: certutil.Config{
			CommonName:   expectedCertCommonName,
			Organization: expectedCertOrganization,
			AltNames:     certutil.AltNames{DNSNames: expectedCertDNSNames},
			Usages:       expectedUsages,
		},
		NotAfter: &certNotAfter,
	}

	cert, err := NewSignedCert(cc, key, caCert, caKey, false)
	if err != nil {
		t.Fatalf("NewSignedCert returned an error: %v", err)
	}

	if cert.IsCA {
		t.Errorf("expected certificate to not be a CA, but it was")
	}

	if cert.Subject.CommonName != expectedCertCommonName {
		t.Errorf("expected CommonName to be %s, got %s", expectedCertCommonName, cert.Subject.CommonName)
	}

	if !util.ContainsAllValues(cert.Subject.Organization, expectedCertOrganization) {
		t.Errorf("expected Organization to contain %v, got %v", expectedCertOrganization, cert.Subject.Organization)
	}

	if !util.ContainsAllValues(cert.DNSNames, expectedCertDNSNames) {
		t.Errorf("expected DNSNames to contain %v, got %v", expectedCertDNSNames, cert.DNSNames)
	}

	if !util.ContainsAllValues(cert.ExtKeyUsage, expectedUsages) {
		t.Errorf("expected Usages to contain ExtKeyUsageServerAuth, got %v", cert.ExtKeyUsage)
	}
}

func TestNewSignedCert_ErrorOnEmptyCommonName(t *testing.T) {
	// Create a key pair for the certificate and CA.
	key, err := GeneratePrivateKey(x509.RSA)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Create a CertConfig for the test certificate.
	expectedCertOrganization := []string{"test-org"}
	expectedCertDNSNames := []string{"localhost"}
	expectedUsages := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	certNotAfter := time.Now().Add(constants.CertificateValidity)
	cc := &CertConfig{
		Config: certutil.Config{
			CommonName:   "",
			Organization: expectedCertOrganization,
			AltNames:     certutil.AltNames{DNSNames: expectedCertDNSNames},
			Usages:       expectedUsages,
		},
		NotAfter: &certNotAfter,
	}

	_, err = NewSignedCert(cc, key, &x509.Certificate{}, key, false)
	if err == nil {
		t.Fatalf("expected error for empty CommonName, but got nil")
	}

	expectedErrorMsg := "must specify a CommonName"
	if err.Error() != expectedErrorMsg {
		t.Errorf("expected error %s, got %v", expectedErrorMsg, err)
	}
}

func TestAppendSANsToAltNames(t *testing.T) {
	tests := []struct {
		name    string
		SANs    []string
		wantIPs []net.IP
		wantDNS []string
	}{
		{
			name:    "AppendSANsToAltNames_ValidIPAddress_ShouldReturnCorrectIPs",
			SANs:    []string{"192.168.1.1"},
			wantIPs: []net.IP{net.ParseIP("192.168.1.1")},
			wantDNS: nil,
		},
		{
			name:    "AppendSANsToAltNames_ValidDNSName_ShouldReturnCorrectDNS",
			SANs:    []string{"example.com"},
			wantIPs: nil,
			wantDNS: []string{"example.com"},
		},
		{
			name:    "AppendSANsToAltNames_InvalidDNSAndValidIP_ShouldIgnoreInvalidDNSAndReturnCorrectIP",
			SANs:    []string{"invalid!.com", "10.0.0.1"},
			wantIPs: []net.IP{net.ParseIP("10.0.0.1")},
			wantDNS: nil,
		},
		{
			name:    "AppendSANsToAltNames_ValidWildcardDNS_ShouldReturnCorrectWildcardDNS",
			SANs:    []string{"*.example.com"},
			wantIPs: nil,
			wantDNS: []string{"*.example.com"},
		},
		{
			name:    "AppendSANsToAltNames_MixedValidIPAndDNS_ShouldReturnCorrectIPAndDNS",
			SANs:    []string{"example.com", "10.0.0.1"},
			wantIPs: []net.IP{net.ParseIP("10.0.0.1")},
			wantDNS: []string{"example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			altNames := &certutil.AltNames{}
			appendSANsToAltNames(altNames, tt.SANs)

			if !reflect.DeepEqual(altNames.IPs, tt.wantIPs) {
				t.Errorf("AppendSANsToAltNames() IPs = %+v, want %+v", altNames.IPs, tt.wantIPs)
			}
			if !reflect.DeepEqual(altNames.DNSNames, tt.wantDNS) {
				t.Errorf("AppendSANsToAltNames() DNS = %+v, want %+v", altNames.IPs, tt.wantIPs)
			}
		})
	}
}

func TestRemoveDuplicateAltNames(t *testing.T) {
	tests := []struct {
		name  string
		input *certutil.AltNames
		want  *certutil.AltNames
	}{
		{
			name: "RemoveDuplicateAltNames_DuplicateDNSNames_ShouldReturnUniqueDNSNames",
			input: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2")},
			},
			want: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2")},
			},
		},
		{
			name: "RemoveDuplicateAltNames_DuplicateIPs_ShouldReturnUniqueIPs",
			input: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.1")},
			},
			want: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1")},
			},
		},
		{
			name: "RemoveDuplicateAltNames_DuplicateDNSNamesAndIPs_ShouldReturnUniqueDNSNamesAndIPs",
			input: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2")},
			},
			want: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2")},
			},
		},
		{
			name: "RemoveDuplicateAltNames_NoDuplicates_ShouldReturnSameAltNames",
			input: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2")},
			},
			want: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.org"},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.2")},
			},
		},
		{
			name: "RemoveDuplicateAltNames_EmptyAltNames_ShouldReturnEmptyAltNames",
			input: &certutil.AltNames{
				DNSNames: []string{},
				IPs:      []net.IP{},
			},
			want: &certutil.AltNames{
				DNSNames: []string{},
				IPs:      nil,
			},
		},
		{
			name:  "RemoveDuplicateAltNames_NilAltNames_ShouldReturnNil",
			input: nil,
			want:  nil,
		},
		{
			name: "RemoveDuplicateAltNames_EmptyDNSNamesWithNonEmptyIPs_ShouldReturnUniqueIPs",
			input: &certutil.AltNames{
				DNSNames: []string{},
				IPs:      []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.1.1")},
			},
			want: &certutil.AltNames{
				DNSNames: []string{},
				IPs:      []net.IP{net.ParseIP("192.168.1.1")},
			},
		},
		{
			name: "RemoveDuplicateAltNames_EmptyIPsWithNonEmptyDNSNames_ShouldReturnUniqueDNSNames",
			input: &certutil.AltNames{
				DNSNames: []string{"example.com", "example.com"},
				IPs:      []net.IP{},
			},
			want: &certutil.AltNames{
				DNSNames: []string{"example.com"},
				IPs:      nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RemoveDuplicateAltNames(tt.input)
			if !reflect.DeepEqual(tt.input, tt.want) {
				t.Errorf("RemoveDuplicateAltNames() = %+v, want %+v", tt.input, tt.want)
			}
		})
	}
}
