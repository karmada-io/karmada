/*
Copyright 2021 The Karmada Authors.

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

package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"path/filepath"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/util/sets"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

const (
	// certificateBlockType is a possible value for pem.Block.Type.
	certificateBlockType = "CERTIFICATE"
	rsaKeySize           = 3072
	// Duration365d Certificate validity period
	Duration365d = time.Hour * 24 * 365
)

// NewPrivateKey returns a new private key.
var NewPrivateKey = GeneratePrivateKey

// GeneratePrivateKey generates a certificate key. It supports both
// ECDSA (using the P-256 elliptic curve) and RSA algorithms. For RSA,
// the key is generated with a size of 3072 bits. If the keyType is
// x509.UnknownPublicKeyAlgorithm, the function defaults to generating
// an RSA key.
func GeneratePrivateKey(keyType x509.PublicKeyAlgorithm) (crypto.Signer, error) {
	switch keyType {
	case x509.ECDSA:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case x509.RSA, x509.UnknownPublicKeyAlgorithm:
		return rsa.GenerateKey(rand.Reader, rsaKeySize)
	default:
		return nil, fmt.Errorf("unsupported key type: %T, supported key types are RSA and ECDSA", keyType)
	}
}

// CertsConfig is a wrapper around certutil.Config extending it with PublicKeyAlgorithm.
type CertsConfig struct {
	certutil.Config
	NotAfter           *time.Time
	PublicKeyAlgorithm x509.PublicKeyAlgorithm
}

// EncodeCertPEM returns PEM-encoded certificate data
func EncodeCertPEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  certificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

// NewCertificateAuthority creates new certificate and private key for the certificate authority
func NewCertificateAuthority(config *CertsConfig) (*x509.Certificate, crypto.Signer, error) {
	key, err := NewPrivateKey(config.PublicKeyAlgorithm)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create private key while generating CA certificate %v", err)
	}

	cert, err := certutil.NewSelfSignedCACert(config.Config, key)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create self-signed CA certificate %v", err)
	}

	return cert, key, nil
}

// NewCACertAndKey The public and private keys of the root certificate are returned
func NewCACertAndKey(cn string) (*x509.Certificate, *crypto.Signer, error) {
	certCfg := &CertsConfig{Config: certutil.Config{
		CommonName: cn,
	},
	}
	caCert, caKey, err := NewCertificateAuthority(certCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failure while generating CA certificate and key: %v", err)
	}

	return caCert, &caKey, nil
}

// NewSignedCert creates a signed certificate using the given CA certificate and key
func NewSignedCert(cfg *CertsConfig, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, isCA bool) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}

	keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if isCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	RemoveDuplicateAltNames(&cfg.AltNames)

	notAfter := time.Now().Add(Duration365d).UTC()
	if cfg.NotAfter != nil {
		notAfter = *cfg.NotAfter
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:              cfg.AltNames.DNSNames,
		IPAddresses:           cfg.AltNames.IPs,
		SerialNumber:          serial,
		NotBefore:             caCert.NotBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           cfg.Usages,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

// RemoveDuplicateAltNames removes duplicate items in altNames.
func RemoveDuplicateAltNames(altNames *certutil.AltNames) {
	if altNames == nil {
		return
	}

	if altNames.DNSNames != nil {
		altNames.DNSNames = sets.NewString(altNames.DNSNames...).List()
	}

	ipsKeys := make(map[string]struct{})
	var ips []net.IP
	for _, one := range altNames.IPs {
		if _, ok := ipsKeys[one.String()]; !ok {
			ipsKeys[one.String()] = struct{}{}
			ips = append(ips, one)
		}
	}
	altNames.IPs = ips
}

// NewCertAndKey creates new certificate and key by passing the certificate authority certificate and key
func NewCertAndKey(caCert *x509.Certificate, caKey crypto.Signer, config *CertsConfig) (*x509.Certificate, crypto.Signer, error) {
	if len(config.Usages) == 0 {
		return nil, nil, errors.New("must specify at least one ExtKeyUsage")
	}

	key, err := NewPrivateKey(config.PublicKeyAlgorithm)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create private key %v", err)
	}

	cert, err := NewSignedCert(config, key, caCert, caKey, false)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to sign certificate. %v", err)
	}

	return cert, key, nil
}

// PathForKey returns the paths for the key given the path and basename.
func PathForKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.key", name))
}

// PathForCert returns the paths for the certificate given the path and basename.
func PathForCert(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.crt", name))
}

// WriteCert stores the given certificate at the given location
func WriteCert(pkiPath, name string, cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("certificate cannot be nil when writing to file")
	}

	certificatePath := PathForCert(pkiPath, name)
	if err := certutil.WriteCert(certificatePath, EncodeCertPEM(cert)); err != nil {
		return fmt.Errorf("unable to write certificate to file %v", err)
	}

	return nil
}

// WriteKey stores the given key at the given location
func WriteKey(pkiPath, name string, key crypto.Signer) error {
	if key == nil {
		return errors.New("private key cannot be nil when writing to file")
	}

	privateKeyPath := PathForKey(pkiPath, name)
	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return fmt.Errorf("unable to marshal private key to PEM %v", err)
	}
	if err := keyutil.WriteKey(privateKeyPath, encoded); err != nil {
		return fmt.Errorf("unable to write private key to file %v", err)
	}

	return nil
}

// WriteKeyPair stores the given key pair at the given location
func WriteKeyPair(pkiPath, name string, key crypto.Signer) error {
	if key == nil {
		return errors.New("private key cannot be nil when writing to file")
	}

	// Write private key
	privateKeyPath := PathForKey(pkiPath, name)
	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return fmt.Errorf("unable to marshal private key to PEM: %w", err)
	}
	if err := keyutil.WriteKey(privateKeyPath, encoded); err != nil {
		return fmt.Errorf("unable to write private key to file: %w", err)
	}

	// Write public key
	publicKeyPath := filepath.Join(pkiPath, fmt.Sprintf("%s.pub", name))
	publicKey, ok := key.Public().(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("expected RSA public key but got %T", key.Public())
	}

	bytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("unable to marshal public key to bytes: %w", err)
	}

	block := &pem.Block{
		Type:  keyutil.PublicKeyBlockType,
		Bytes: bytes,
	}
	publicEncoded := pem.EncodeToMemory(block)
	if err := keyutil.WriteKey(publicKeyPath, publicEncoded); err != nil {
		return fmt.Errorf("unable to write public key to file: %w", err)
	}

	return nil
}

// WriteCertAndKey Write certificate and key to file.
func WriteCertAndKey(pkiPath, pkiName string, ca *x509.Certificate, key *crypto.Signer) error {
	if err := WriteKey(pkiPath, pkiName, *key); err != nil {
		return err
	}

	if err := WriteCert(pkiPath, pkiName, ca); err != nil {
		return err
	}

	klog.Infof("Generate %s certificate success.", pkiName)
	return nil
}

// NewCertConfig create new CertConfig
func NewCertConfig(cn string, org []string, altNames certutil.AltNames, notAfter *time.Time) *CertsConfig {
	return &CertsConfig{
		Config: certutil.Config{
			CommonName:   cn,
			Organization: org,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			AltNames:     altNames,
		},
		NotAfter: notAfter,
	}
}

// GenCerts creates CA certificate and signs certificates for etcd and karmada components.
func GenCerts(pkiPath, caCertFile, caKeyFile string, etcdServerCertCfg, etcdClientCertCfg, serverCertCfg, clientCertCfg, frontProxyClientCertCfg *CertsConfig) error {
	caCert, caKey, err := getCACertAndKey(caCertFile, caKeyFile)
	if err != nil {
		return fmt.Errorf("failed to get CA cert and key: %w", err)
	}

	if err = WriteCertAndKey(pkiPath, util.CA, caCert, caKey); err != nil {
		return fmt.Errorf("failed to write CA cert and key: %w", err)
	}

	// Generate and write core certificates
	certConfigs := map[string]*CertsConfig{
		util.Server: serverCertCfg,
		util.Client: clientCertCfg,
	}

	for name, config := range certConfigs {
		cert, key, err := NewCertAndKey(caCert, *caKey, config)
		if err != nil {
			return fmt.Errorf("failed to generate %s cert and key: %w", name, err)
		}
		if err = WriteCertAndKey(pkiPath, name, cert, &key); err != nil {
			return fmt.Errorf("failed to write %s cert and key: %w", name, err)
		}
	}

	// Generate and write front-proxy certificates
	frontProxyCaCert, frontProxyCaKey, err := NewCACertAndKey(options.FrontProxyCaCertAndKeyName)
	if err != nil {
		return err
	}
	if err = WriteCertAndKey(pkiPath, options.FrontProxyCaCertAndKeyName, frontProxyCaCert, frontProxyCaKey); err != nil {
		return err
	}

	frontProxyClientCert, frontProxyClientKey, err := NewCertAndKey(frontProxyCaCert, *frontProxyCaKey, frontProxyClientCertCfg)
	if err != nil {
		return err
	}
	if err = WriteCertAndKey(pkiPath, util.FrontProxyClient, frontProxyClientCert, &frontProxyClientKey); err != nil {
		return fmt.Errorf("failed to write %s cert and key: %w", util.FrontProxyClient, err)
	}

	// Skip etcd certificate generation if using external etcd
	if etcdServerCertCfg == nil && etcdClientCertCfg == nil {
		return nil
	}

	// Generate and write etcd certificates
	etcdCertConfigs := map[string]*CertsConfig{
		options.EtcdServerCertAndKeyName: etcdServerCertCfg,
		options.EtcdClientCertAndKeyName: etcdClientCertCfg,
	}

	for name, config := range etcdCertConfigs {
		cert, key, err := NewCertAndKey(caCert, *caKey, config)
		if err != nil {
			return fmt.Errorf("failed to generate %s cert and key: %w", name, err)
		}
		if err = WriteCertAndKey(pkiPath, name, cert, &key); err != nil {
			return fmt.Errorf("failed to write %s cert and key: %w", name, err)
		}
	}

	return nil
}

// GenKeyPair creates a key pair with the given name.
func GenKeyPair(pkiPath, name string) error {
	key, err := GeneratePrivateKey(x509.RSA)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	if err := WriteKeyPair(pkiPath, name, key); err != nil {
		return fmt.Errorf("failed to write key pair: %w", err)
	}

	return nil
}

func getCACertAndKey(caCertFile, caKeyFile string) (caCert *x509.Certificate, caKey *crypto.Signer, err error) {
	if caKeyFile != "" && caCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(caCertFile, caKeyFile)
		if err != nil {
			return nil, nil, err
		}
		caCert, err = x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			return nil, nil, err
		}
		key := certificate.PrivateKey.(crypto.Signer)
		caKey = &key
	} else {
		caCert, caKey, err = NewCACertAndKey("karmada")
		if err != nil {
			return nil, nil, err
		}
	}
	return caCert, caKey, nil
}
