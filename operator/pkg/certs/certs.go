package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	netutils "k8s.io/utils/net"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

const (
	// CertificateBlockType is a possible value for pem.Block.Type.
	CertificateBlockType = "CERTIFICATE"
	rsaKeySize           = 2048
	keyExtension         = ".key"
	certExtension        = ".crt"
)

// AltNamesMutatorConfig is a config to AltNamesMutator. It includes necessary
// configs to AltNamesMutator.
type AltNamesMutatorConfig struct {
	Name       string
	Namespace  string
	Components *operatorv1alpha1.KarmadaComponents
}

type altNamesMutatorFunc func(*AltNamesMutatorConfig, *CertConfig) error

// CertConfig represents a config to generate certificate by karmada.
type CertConfig struct {
	Name                string
	CAName              string
	NotAfter            *time.Time
	PublicKeyAlgorithm  x509.PublicKeyAlgorithm // TODO: All public key of karmada cert use the RSA algorithm by default
	Config              certutil.Config
	AltNamesMutatorFunc altNamesMutatorFunc
}

func (config *CertConfig) defaultPublicKeyAlgorithm() {
	if config.PublicKeyAlgorithm == x509.UnknownPublicKeyAlgorithm {
		config.PublicKeyAlgorithm = x509.RSA
	}
}

func (config *CertConfig) defaultNotAfter() {
	if config.NotAfter == nil {
		notAfter := time.Now().Add(constants.CertificateValidity).UTC()
		config.NotAfter = &notAfter
	}
}

// GetDefaultCertList returns all of karmada certConfigs, it include karmada, front and etcd.
func GetDefaultCertList() []*CertConfig {
	return []*CertConfig{
		// karmada cert config.
		KarmadaCertRootCA(),
		KarmadaCertAdmin(),
		KarmadaCertApiserver(),
		// front proxy cert config.
		KarmadaCertFrontProxyCA(),
		KarmadaCertFrontProxyClient(),
		// ETCD cert config.
		KarmadaCertEtcdCA(),
		KarmadaCertEtcdServer(),
		KarmadaCertEtcdClient(),
	}
}

// KarmadaCertRootCA returns karmada ca cert config.
func KarmadaCertRootCA() *CertConfig {
	return &CertConfig{
		Name: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "karmada",
		},
	}
}

// KarmadaCertAdmin returns karmada client cert config.
func KarmadaCertAdmin() *CertConfig {
	return &CertConfig{
		Name:   constants.KarmadaCertAndKeyName,
		CAName: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName:   "system:admin",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}

// KarmadaCertApiserver returns karmada apiserver cert config.
func KarmadaCertApiserver() *CertConfig {
	return &CertConfig{
		Name:   constants.ApiserverCertAndKeyName,
		CAName: constants.CaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "karmada-apiserver",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(apiServerAltNamesMutator),
	}
}

// KarmadaCertFrontProxyCA returns karmada front proxy cert config.
func KarmadaCertFrontProxyCA() *CertConfig {
	return &CertConfig{
		Name: constants.FrontProxyCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "front-proxy-ca",
		},
	}
}

// KarmadaCertFrontProxyClient returns karmada front proxy client cert config.
func KarmadaCertFrontProxyClient() *CertConfig {
	return &CertConfig{
		Name:   constants.FrontProxyClientCertAndKeyName,
		CAName: constants.FrontProxyCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "front-proxy-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
}

// KarmadaCertEtcdCA returns karmada front proxy client cert config.
func KarmadaCertEtcdCA() *CertConfig {
	return &CertConfig{
		Name: constants.EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "karmada-etcd-ca",
		},
	}
}

// KarmadaCertEtcdServer returns etcd server cert config.
func KarmadaCertEtcdServer() *CertConfig {
	return &CertConfig{
		Name:   constants.EtcdServerCertAndKeyName,
		CAName: constants.EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "karmada-etcd-server",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		AltNamesMutatorFunc: makeAltNamesMutator(etcdServerAltNamesMutator),
	}
}

// KarmadaCertEtcdClient returns etcd client cert config.
func KarmadaCertEtcdClient() *CertConfig {
	return &CertConfig{
		Name:   constants.EtcdClientCertAndKeyName,
		CAName: constants.EtcdCaCertAndKeyName,
		Config: certutil.Config{
			CommonName: "karmada-etcd-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
	}
}

// KarmadaCert is karmada certificate, it includes certificate basic message.
// we can directly get the byte array of certificate key and cert from the object.
type KarmadaCert struct {
	pairName string
	caName   string
	cert     []byte
	key      []byte
}

// CertData returns certificate cert data.
func (cert *KarmadaCert) CertData() []byte {
	return cert.cert
}

// KeyData returns certificate key data.
func (cert *KarmadaCert) KeyData() []byte {
	return cert.key
}

// CertName returns cert file name. its default suffix is ".crt".
func (cert *KarmadaCert) CertName() string {
	pair := cert.pairName
	if len(pair) == 0 {
		pair = "cert"
	}
	return pair + certExtension
}

// KeyName returns cert key file name. its default suffix is ".key".
func (cert *KarmadaCert) KeyName() string {
	pair := cert.pairName
	if len(pair) == 0 {
		pair = "cert"
	}
	return pair + keyExtension
}

// GeneratePrivateKey generates cert key with default size if 1024. it support
// ECDSA and RAS algorithm.
func GeneratePrivateKey(keyType x509.PublicKeyAlgorithm) (crypto.Signer, error) {
	if keyType == x509.ECDSA {
		return ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	}

	return rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
}

// NewCertificateAuthority creates new certificate and private key for the certificate authority
func NewCertificateAuthority(cc *CertConfig) (*KarmadaCert, error) {
	cc.defaultPublicKeyAlgorithm()

	key, err := GeneratePrivateKey(cc.PublicKeyAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("unable to create private key while generating CA certificate, err: %w", err)
	}

	cert, err := certutil.NewSelfSignedCACert(cc.Config, key)
	if err != nil {
		return nil, fmt.Errorf("unable to create self-signed CA certificate, err: %w", err)
	}

	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal private key to PEM, err: %w", err)
	}

	return &KarmadaCert{
		pairName: cc.Name,
		caName:   cc.CAName,
		cert:     EncodeCertPEM(cert),
		key:      encoded,
	}, nil
}

// CreateCertAndKeyFilesWithCA loads the given certificate authority from disk, then generates and writes out the given certificate and key.
// The certSpec and caCertSpec should both be one of the variables from this package.
func CreateCertAndKeyFilesWithCA(cc *CertConfig, caCertData, caKeyData []byte) (*KarmadaCert, error) {
	if len(cc.Config.Usages) == 0 {
		return nil, fmt.Errorf("must specify at least one ExtKeyUsage")
	}

	cc.defaultNotAfter()
	cc.defaultPublicKeyAlgorithm()

	key, err := GeneratePrivateKey(cc.PublicKeyAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("unable to create private key, err: %w", err)
	}

	caCerts, err := certutil.ParseCertsPEM(caCertData)
	if err != nil {
		return nil, err
	}

	caKey, err := ParsePrivateKeyPEM(caKeyData)
	if err != nil {
		return nil, err
	}

	// Safely pick the first one because the sender's certificate must come first in the list.
	// For details, see: https://www.rfc-editor.org/rfc/rfc4346#section-7.4.2
	caCert := caCerts[0]

	cert, err := NewSignedCert(cc, key, caCert, caKey, false)
	if err != nil {
		return nil, err
	}

	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal private key to PEM, err: %w", err)
	}

	return &KarmadaCert{
		pairName: cc.Name,
		caName:   cc.CAName,
		cert:     EncodeCertPEM(cert),
		key:      encoded,
	}, nil
}

// NewSignedCert creates a signed certificate using the given CA certificate and key
func NewSignedCert(cc *CertConfig, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, isCA bool) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cc.Config.CommonName) == 0 {
		return nil, fmt.Errorf("must specify a CommonName")
	}

	keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if isCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	RemoveDuplicateAltNames(&cc.Config.AltNames)
	notAfter := time.Now().Add(constants.CertificateValidity).UTC()
	if cc.NotAfter != nil {
		notAfter = *cc.NotAfter
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cc.Config.CommonName,
			Organization: cc.Config.Organization,
		},
		DNSNames:              cc.Config.AltNames.DNSNames,
		IPAddresses:           cc.Config.AltNames.IPs,
		SerialNumber:          serial,
		NotBefore:             caCert.NotBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           cc.Config.Usages,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
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

func appendSANsToAltNames(altNames *certutil.AltNames, SANs []string) {
	for _, altname := range SANs {
		if ip := netutils.ParseIPSloppy(altname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else if len(validation.IsDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		} else if len(validation.IsWildcardDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		}
	}
}

// EncodeCertPEM returns PEM-endcoded certificate data
func EncodeCertPEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  CertificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

// ParsePrivateKeyPEM parses crypto.Signer from byte array. the key
// must be encryption by ECDSA and RAS.
func ParsePrivateKeyPEM(keyData []byte) (crypto.Signer, error) {
	caPrivateKey, err := keyutil.ParsePrivateKeyPEM(keyData)
	if err != nil {
		return nil, err
	}

	// Allow RSA and ECDSA formats only
	var key crypto.Signer
	switch k := caPrivateKey.(type) {
	case *rsa.PrivateKey:
		key = k
	case *ecdsa.PrivateKey:
		key = k
	default:
		return nil, errors.New("the private key is neither in RSA nor ECDSA format")
	}

	return key, nil
}

func makeAltNamesMutator(f func(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error)) altNamesMutatorFunc {
	return func(cfg *AltNamesMutatorConfig, cc *CertConfig) error {
		altNames, err := f(cfg)
		if err != nil {
			return err
		}

		cc.Config.AltNames = *altNames
		return nil
	}
}

func etcdServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	etcdClientServiceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", util.KarmadaEtcdClientName(cfg.Name), cfg.Namespace)
	etcdPeerServiceDNS := fmt.Sprintf("*.%s.%s.svc.cluster.local", util.KarmadaEtcdName(cfg.Name), cfg.Namespace)

	altNames := &certutil.AltNames{
		DNSNames: []string{"localhost", etcdClientServiceDNS, etcdPeerServiceDNS},
		IPs:      []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	if cfg.Components.Etcd.Local != nil {
		appendSANsToAltNames(altNames, cfg.Components.Etcd.Local.ServerCertSANs)
	}

	return altNames, nil
}

func apiServerAltNamesMutator(cfg *AltNamesMutatorConfig) (*certutil.AltNames, error) {
	altNames := &certutil.AltNames{
		DNSNames: []string{
			"localhost",
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			fmt.Sprintf("*.%s.svc.cluster.local", cfg.Namespace),
			fmt.Sprintf("*.%s.svc", cfg.Namespace),
		},
		IPs: []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	if len(cfg.Components.KarmadaAPIServer.CertSANs) > 0 {
		appendSANsToAltNames(altNames, cfg.Components.KarmadaAPIServer.CertSANs)
	}

	return altNames, nil
}
