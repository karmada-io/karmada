/*
Copyright 2026 The Karmada Authors.

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

package kubernetes

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/cert"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	globaloptions "github.com/karmada-io/karmada/pkg/karmadactl/options"
)

type caMaterial struct {
	cert    *x509.Certificate
	key     crypto.Signer
	certPEM []byte
	keyPEM  []byte
}

// runCertRotate executes certificate rotation for the cert material managed by karmadactl init.
func (i *CommandInitOption) runCertRotate() error {
	i.CertAndKeyFileData = map[string][]byte{}
	if err := i.prepareRotatedCertAndKeyData(); err != nil {
		return err
	}

	if err := i.refreshLocalAdminKubeconfigIfExists(); err != nil {
		return err
	}

	if err := i.updateCertsSecrets(); err != nil {
		return err
	}

	klog.Infof("Certificate Secrets in namespace %q have been updated. Restart Karmada control plane components to load the rotated certificates.", i.Namespace)
	return nil
}

// prepareRotatedCertAndKeyData rebuilds CertAndKeyFileData from existing CA material.
func (i *CommandInitOption) prepareRotatedCertAndKeyData() error {
	karmadaSecret, err := i.getExistingSecret(globaloptions.KarmadaCertsName)
	if err != nil {
		return err
	}
	certConfigs, err := i.buildRotateCertConfigs(karmadaSecret)
	if err != nil {
		return err
	}

	rootCA, err := i.loadRootCAMaterial(karmadaSecret)
	if err != nil {
		return err
	}
	i.setCertAndKeyData(globaloptions.CaCertAndKeyName, rootCA.certPEM, rootCA.keyPEM)

	frontProxyCA, err := loadCAMaterialFromSecret(karmadaSecret, options.FrontProxyCaCertAndKeyName)
	if err != nil {
		return err
	}
	i.setCertAndKeyData(options.FrontProxyCaCertAndKeyName, frontProxyCA.certPEM, frontProxyCA.keyPEM)

	// karmada.key is also the ServiceAccount signing key, so keep it stable during TLS certificate rotation.
	if err := i.signLeafCertWithExistingKey(options.KarmadaCertAndKeyName, rootCA, certConfigs.karmada, karmadaSecret); err != nil {
		return err
	}
	if err := i.signLeafCert(options.ApiserverCertAndKeyName, rootCA, certConfigs.apiserver); err != nil {
		return err
	}
	if err := i.signLeafCert(options.FrontProxyClientCertAndKeyName, frontProxyCA, certConfigs.frontProxyClient); err != nil {
		return err
	}

	if i.isExternalEtcdProvided() {
		return i.prepareExternalEtcdCertAndKeyData(karmadaSecret)
	}
	return i.prepareInternalEtcdCertAndKeyData(karmadaSecret, certConfigs)
}

// buildRotateCertConfigs builds deterministic configs and preserves identities from existing leaf certificates.
func (i *CommandInitOption) buildRotateCertConfigs(karmadaSecret *corev1.Secret) (*initCertConfigs, error) {
	certConfigs, err := i.buildCertConfigs(false)
	if err != nil {
		return nil, err
	}

	existingKarmadaCert, err := loadCertificateFromSecret(karmadaSecret, options.KarmadaCertAndKeyName)
	if err != nil {
		return nil, err
	}
	mergeCertificateAltNames(certConfigs.karmada, existingKarmadaCert)

	existingAPIServerCert, err := loadCertificateFromSecret(karmadaSecret, options.ApiserverCertAndKeyName)
	if err != nil {
		return nil, err
	}
	mergeCertificateAltNames(certConfigs.apiserver, existingAPIServerCert)
	return certConfigs, nil
}

// mergeCertificateAltNames adds persisted DNS and IP identities without sharing mutable config slices.
func mergeCertificateAltNames(certConfig *cert.CertsConfig, existingCert *x509.Certificate) {
	certConfig.AltNames.DNSNames = append(append([]string(nil), certConfig.AltNames.DNSNames...), existingCert.DNSNames...)
	certConfig.AltNames.IPs = append(append([]net.IP(nil), certConfig.AltNames.IPs...), existingCert.IPAddresses...)
	cert.RemoveDuplicateAltNames(&certConfig.AltNames)
}

func (i *CommandInitOption) prepareInternalEtcdCertAndKeyData(karmadaSecret *corev1.Secret, certConfigs *initCertConfigs) error {
	etcdSecret, err := i.getExistingSecret(etcdCertName)
	if err != nil {
		return err
	}
	if !hasExistingEtcdServerCert(etcdSecret) {
		return fmt.Errorf("internal etcd parameters cannot be used to rotate certificates for an existing external etcd installation")
	}

	etcdCA, err := loadCAMaterialFromSecret(etcdSecret, options.EtcdCaCertAndKeyName)
	if err != nil {
		etcdCA, err = loadCAMaterialFromSecret(karmadaSecret, options.EtcdCaCertAndKeyName)
		if err != nil {
			return err
		}
	}
	i.setCertAndKeyData(options.EtcdCaCertAndKeyName, etcdCA.certPEM, etcdCA.keyPEM)

	existingEtcdServerCert, err := loadCertificateFromSecret(etcdSecret, options.EtcdServerCertAndKeyName)
	if err != nil {
		return err
	}
	mergeCertificateAltNames(certConfigs.etcdServer, existingEtcdServerCert)

	if err := i.signLeafCert(options.EtcdServerCertAndKeyName, etcdCA, certConfigs.etcdServer); err != nil {
		return err
	}
	return i.signLeafCert(options.EtcdClientCertAndKeyName, etcdCA, certConfigs.etcdClient)
}

func (i *CommandInitOption) prepareExternalEtcdCertAndKeyData(karmadaSecret *corev1.Secret) error {
	etcdSecret, err := i.getExistingSecret(etcdCertName)
	if err != nil {
		return err
	}
	if hasExistingEtcdServerCert(etcdSecret) {
		return fmt.Errorf("external etcd parameters cannot be used to rotate certificates for an existing internal etcd installation")
	}

	existingCACertPEM, err := firstSecretDataValue(certFileName(options.EtcdCaCertAndKeyName), etcdSecret, karmadaSecret)
	if err != nil {
		return err
	}
	if err := i.validateExternalEtcdCA(existingCACertPEM); err != nil {
		return err
	}
	i.setCertAndKeyData(options.EtcdCaCertAndKeyName, existingCACertPEM, emptyByteSlice)

	i.setCertAndKeyData(options.EtcdServerCertAndKeyName, emptyByteSlice, emptyByteSlice)

	existingClientCertPEM, err := firstSecretDataValue(certFileName(options.EtcdClientCertAndKeyName), karmadaSecret)
	if err != nil {
		return err
	}
	existingClientKeyPEM, err := firstSecretDataValue(keyFileName(options.EtcdClientCertAndKeyName), karmadaSecret)
	if err != nil {
		return err
	}
	if err := i.validateExternalEtcdClientCredential(existingClientCertPEM, existingClientKeyPEM); err != nil {
		return err
	}
	i.setCertAndKeyData(options.EtcdClientCertAndKeyName, existingClientCertPEM, existingClientKeyPEM)
	return nil
}

// validateExternalEtcdCA verifies that supplied external etcd files do not replace the existing trust root.
func (i *CommandInitOption) validateExternalEtcdCA(existingCertPEM []byte) error {
	existingCert, err := cert.LoadCertificatePEM(existingCertPEM)
	if err != nil {
		return fmt.Errorf("failed to load existing external etcd CA certificate: %w", err)
	}
	if i.ExternalEtcdCACertPath == "" {
		return nil
	}

	suppliedCertPEM, err := os.ReadFile(i.ExternalEtcdCACertPath)
	if err != nil {
		return err
	}
	suppliedCert, err := cert.LoadCertificatePEM(suppliedCertPEM)
	if err != nil {
		return fmt.Errorf("failed to load external etcd CA certificate file %q: %w", i.ExternalEtcdCACertPath, err)
	}
	if !bytes.Equal(suppliedCert.Raw, existingCert.Raw) {
		return errors.New("external etcd CA certificate cannot be changed by certificate rotation; rotate external etcd credentials separately")
	}
	return nil
}

// validateExternalEtcdClientCredential verifies the existing pair and rejects supplied replacement credentials.
func (i *CommandInitOption) validateExternalEtcdClientCredential(existingCertPEM, existingKeyPEM []byte) error {
	existingCert, _, err := cert.LoadCertAndKeyPEM(existingCertPEM, existingKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to load existing external etcd client certificate and key: %w", err)
	}
	if i.ExternalEtcdClientCertPath == "" || i.ExternalEtcdClientKeyPath == "" {
		return nil
	}

	suppliedCertPEM, err := os.ReadFile(i.ExternalEtcdClientCertPath)
	if err != nil {
		return err
	}
	suppliedKeyPEM, err := os.ReadFile(i.ExternalEtcdClientKeyPath)
	if err != nil {
		return err
	}
	suppliedCert, _, err := cert.LoadCertAndKeyPEM(suppliedCertPEM, suppliedKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to load external etcd client certificate and key files: %w", err)
	}
	if !bytes.Equal(suppliedCert.Raw, existingCert.Raw) {
		return errors.New("external etcd client certificate cannot be changed by certificate rotation; rotate external etcd credentials separately")
	}
	return nil
}

// loadRootCAMaterial loads the root CA from files or the existing karmada-cert Secret.
func (i *CommandInitOption) loadRootCAMaterial(karmadaSecret *corev1.Secret) (*caMaterial, error) {
	existingCert, err := loadCertificateFromSecret(karmadaSecret, globaloptions.CaCertAndKeyName)
	if err != nil {
		return nil, err
	}

	if i.CaCertFile != "" && i.CaKeyFile != "" {
		fileCA, err := loadCAMaterialFromFiles(i.CaCertFile, i.CaKeyFile)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(fileCA.cert.Raw, existingCert.Raw) {
			return nil, fmt.Errorf("ca-cert-file must match the existing %s/%s Secret CA certificate during certificate rotation", i.Namespace, globaloptions.KarmadaCertsName)
		}
		return fileCA, nil
	}

	return loadCAMaterialFromSecret(karmadaSecret, globaloptions.CaCertAndKeyName)
}

// signLeafCert signs one leaf certificate using existing CA material.
func (i *CommandInitOption) signLeafCert(name string, ca *caMaterial, certConfig *cert.CertsConfig) error {
	if certConfig == nil {
		return fmt.Errorf("certificate config for %s is nil", name)
	}
	if err := validateCAForLeaf(ca, certConfig, time.Now().UTC()); err != nil {
		return fmt.Errorf("cannot sign certificate %q: %w", name, err)
	}

	leafCert, leafKey, err := cert.NewCertAndKey(ca.cert, ca.key, certConfig)
	if err != nil {
		return err
	}
	leafKeyPEM, err := cert.EncodePrivateKeyPEM(leafKey)
	if err != nil {
		return err
	}
	i.setCertAndKeyData(name, cert.EncodeCertPEM(leafCert), leafKeyPEM)
	return nil
}

// signLeafCertWithExistingKey renews a leaf certificate without replacing its private key.
func (i *CommandInitOption) signLeafCertWithExistingKey(name string, ca *caMaterial, certConfig *cert.CertsConfig, secret *corev1.Secret) error {
	if certConfig == nil {
		return fmt.Errorf("certificate config for %s is nil", name)
	}
	if len(certConfig.Usages) == 0 {
		return fmt.Errorf("certificate config for %s must specify at least one ExtKeyUsage", name)
	}
	if err := validateCAForLeaf(ca, certConfig, time.Now().UTC()); err != nil {
		return fmt.Errorf("cannot sign certificate %q: %w", name, err)
	}

	certPEM, err := secretDataValue(secret, certFileName(name))
	if err != nil {
		return err
	}
	keyPEM, err := secretDataValue(secret, keyFileName(name))
	if err != nil {
		return err
	}
	_, existingKey, err := cert.LoadCertAndKeyPEM(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to load existing certificate and key for %q: %w", name, err)
	}

	leafCert, err := cert.NewSignedCert(certConfig, existingKey, ca.cert, ca.key, false)
	if err != nil {
		return fmt.Errorf("unable to sign certificate: %w", err)
	}
	i.setCertAndKeyData(name, cert.EncodeCertPEM(leafCert), keyPEM)
	return nil
}

// validateCAForLeaf verifies that the existing CA can issue the requested leaf certificate.
func validateCAForLeaf(ca *caMaterial, certConfig *cert.CertsConfig, now time.Time) error {
	if ca == nil || ca.cert == nil || ca.key == nil {
		return errors.New("ca certificate and private key must not be nil")
	}
	if !ca.cert.BasicConstraintsValid || !ca.cert.IsCA {
		return fmt.Errorf("certificate %q is not a valid certificate authority", ca.cert.Subject.CommonName)
	}
	// RFC 5280 requires keyCertSign only when the optional keyUsage extension is present.
	if ca.cert.KeyUsage != 0 && ca.cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return fmt.Errorf("ca certificate %q is not authorized to sign certificates", ca.cert.Subject.CommonName)
	}
	if now.Before(ca.cert.NotBefore) {
		return fmt.Errorf("ca certificate %q is not valid before %s", ca.cert.Subject.CommonName, ca.cert.NotBefore.UTC().Format(time.RFC3339))
	}
	if now.After(ca.cert.NotAfter) {
		return fmt.Errorf("ca certificate %q expired at %s", ca.cert.Subject.CommonName, ca.cert.NotAfter.UTC().Format(time.RFC3339))
	}

	leafNotAfter := now.Add(cert.Duration365d)
	if certConfig != nil && certConfig.NotAfter != nil {
		leafNotAfter = certConfig.NotAfter.UTC()
	}
	if !leafNotAfter.After(now) {
		return fmt.Errorf("requested certificate expiry %s is not after the current time", leafNotAfter.Format(time.RFC3339))
	}
	if leafNotAfter.After(ca.cert.NotAfter) {
		return fmt.Errorf("requested certificate expiry %s exceeds CA expiry %s", leafNotAfter.Format(time.RFC3339), ca.cert.NotAfter.UTC().Format(time.RFC3339))
	}
	return nil
}

// refreshLocalAdminKubeconfigIfExists updates the generated local kubeconfig without changing its endpoint or context.
func (i *CommandInitOption) refreshLocalAdminKubeconfigIfExists() error {
	if i.KarmadaDataPath == "" {
		return nil
	}

	kubeconfigPath := filepath.Join(i.KarmadaDataPath, options.KarmadaKubeConfigName)
	fileInfo, err := os.Lstat(kubeconfigPath)
	if os.IsNotExist(err) {
		klog.Warningf("Local Karmada kubeconfig %q was not found; only certificate Secrets will be updated.", kubeconfigPath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to inspect local Karmada kubeconfig %q: %w", kubeconfigPath, err)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("local Karmada kubeconfig %q must be a regular file", kubeconfigPath)
	}

	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load local Karmada kubeconfig %q: %w", kubeconfigPath, err)
	}
	cluster, ok := kubeconfig.Clusters[options.ClusterName]
	if !ok || cluster == nil {
		return fmt.Errorf("local Karmada kubeconfig %q does not contain cluster %q", kubeconfigPath, options.ClusterName)
	}
	authInfo, ok := kubeconfig.AuthInfos[options.UserName]
	if !ok || authInfo == nil {
		return fmt.Errorf("local Karmada kubeconfig %q does not contain user %q", kubeconfigPath, options.UserName)
	}
	if err := i.validateLocalAdminKubeconfigIdentity(kubeconfigPath, cluster.CertificateAuthorityData, cluster.CertificateAuthority,
		authInfo.ClientCertificateData, authInfo.ClientCertificate); err != nil {
		return err
	}

	cluster.CertificateAuthority = ""
	cluster.CertificateAuthorityData = cloneBytes(i.CertAndKeyFileData[certFileName(globaloptions.CaCertAndKeyName)])
	authInfo.ClientCertificate = ""
	authInfo.ClientCertificateData = cloneBytes(i.CertAndKeyFileData[certFileName(options.KarmadaCertAndKeyName)])
	authInfo.ClientKey = ""
	authInfo.ClientKeyData = cloneBytes(i.CertAndKeyFileData[keyFileName(options.KarmadaCertAndKeyName)])

	configBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to serialize local Karmada kubeconfig %q: %w", kubeconfigPath, err)
	}
	if err := writeFileAtomically(kubeconfigPath, configBytes, fileInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to update local Karmada kubeconfig %q: %w", kubeconfigPath, err)
	}
	return nil
}

// validateLocalAdminKubeconfigIdentity binds the local endpoint to the selected remote certificate identity.
func (i *CommandInitOption) validateLocalAdminKubeconfigIdentity(kubeconfigPath string, caData []byte, caFile string, clientCertData []byte, clientCertFile string) error {
	localCA, err := loadKubeconfigCA(kubeconfigPath, caData, caFile)
	if err != nil {
		return err
	}
	targetCA, err := cert.LoadCertificatePEM(i.CertAndKeyFileData[certFileName(globaloptions.CaCertAndKeyName)])
	if err != nil {
		return fmt.Errorf("failed to load target CA certificate from Secret %s/%s: %w", i.Namespace, globaloptions.KarmadaCertsName, err)
	}
	if !bytes.Equal(localCA.Raw, targetCA.Raw) {
		return fmt.Errorf("local Karmada kubeconfig %q CA does not match Secret %s/%s CA", kubeconfigPath, i.Namespace, globaloptions.KarmadaCertsName)
	}

	localClientCert, err := loadKubeconfigClientCertificate(kubeconfigPath, clientCertData, clientCertFile)
	if err != nil {
		return err
	}
	targetClientCert, err := cert.LoadCertificatePEM(i.CertAndKeyFileData[certFileName(options.KarmadaCertAndKeyName)])
	if err != nil {
		return fmt.Errorf("failed to load target client certificate from Secret %s/%s: %w", i.Namespace, globaloptions.KarmadaCertsName, err)
	}
	if !bytes.Equal(localClientCert.RawSubjectPublicKeyInfo, targetClientCert.RawSubjectPublicKeyInfo) {
		return fmt.Errorf("local Karmada kubeconfig %q client certificate does not match Secret %s/%s client identity", kubeconfigPath, i.Namespace, globaloptions.KarmadaCertsName)
	}
	return nil
}

// loadKubeconfigCA loads an embedded CA or resolves its file relative to the kubeconfig.
func loadKubeconfigCA(kubeconfigPath string, caData []byte, caFile string) (*x509.Certificate, error) {
	if len(caData) > 0 {
		loadedCA, err := cert.LoadCertificatePEM(caData)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate embedded in local Karmada kubeconfig %q: %w", kubeconfigPath, err)
		}
		return loadedCA, nil
	}
	if caFile == "" {
		return nil, fmt.Errorf("local Karmada kubeconfig %q does not contain CA certificate data or a CA certificate file", kubeconfigPath)
	}
	if !filepath.IsAbs(caFile) {
		caFile = filepath.Join(filepath.Dir(kubeconfigPath), caFile)
	}
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate file %q referenced by local Karmada kubeconfig: %w", caFile, err)
	}
	loadedCA, err := cert.LoadCertificatePEM(caData)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate file %q referenced by local Karmada kubeconfig: %w", caFile, err)
	}
	return loadedCA, nil
}

// loadKubeconfigClientCertificate loads an embedded client certificate or resolves its file relative to the kubeconfig.
func loadKubeconfigClientCertificate(kubeconfigPath string, certData []byte, certFile string) (*x509.Certificate, error) {
	if len(certData) > 0 {
		loadedCert, err := cert.LoadCertificatePEM(certData)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate embedded in local Karmada kubeconfig %q: %w", kubeconfigPath, err)
		}
		return loadedCert, nil
	}
	if certFile == "" {
		return nil, fmt.Errorf("local Karmada kubeconfig %q does not contain client certificate data or a client certificate file", kubeconfigPath)
	}
	if !filepath.IsAbs(certFile) {
		certFile = filepath.Join(filepath.Dir(kubeconfigPath), certFile)
	}
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read client certificate file %q referenced by local Karmada kubeconfig: %w", certFile, err)
	}
	loadedCert, err := cert.LoadCertificatePEM(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate file %q referenced by local Karmada kubeconfig: %w", certFile, err)
	}
	return loadedCert, nil
}

// writeFileAtomically replaces a file using a temporary file in the same directory.
func writeFileAtomically(filename string, data []byte, mode os.FileMode) error {
	tempFile, err := os.CreateTemp(filepath.Dir(filename), "."+filepath.Base(filename)+"-*")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempName)
	}()

	if err := tempFile.Chmod(mode); err != nil {
		return err
	}
	if _, err := tempFile.Write(data); err != nil {
		return err
	}
	if err := tempFile.Sync(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, filename)
}

func (i *CommandInitOption) setCertAndKeyData(name string, certPEM, keyPEM []byte) {
	i.CertAndKeyFileData[certFileName(name)] = cloneBytes(certPEM)
	i.CertAndKeyFileData[keyFileName(name)] = cloneBytes(keyPEM)
}

func (i *CommandInitOption) getExistingSecret(name string) (*corev1.Secret, error) {
	existingSecret, err := i.KubeClientSet.CoreV1().Secrets(i.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get existing Secret %s/%s: %w", i.Namespace, name, err)
	}
	return existingSecret, nil
}

// loadCAMaterialFromFiles loads a CA certificate and private key from local files.
func loadCAMaterialFromFiles(certFile, keyFile string) (*caMaterial, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	loadedCert, loadedKey, err := cert.LoadCertAndKeyPEM(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &caMaterial{
		cert:    loadedCert,
		key:     loadedKey,
		certPEM: cloneBytes(certPEM),
		keyPEM:  cloneBytes(keyPEM),
	}, nil
}

// loadCAMaterialFromSecret loads a CA certificate and private key from a Secret.
func loadCAMaterialFromSecret(secret *corev1.Secret, name string) (*caMaterial, error) {
	certPEM, err := secretDataValue(secret, certFileName(name))
	if err != nil {
		return nil, err
	}
	keyPEM, err := secretDataValue(secret, keyFileName(name))
	if err != nil {
		return nil, err
	}
	loadedCert, loadedKey, err := cert.LoadCertAndKeyPEM(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &caMaterial{
		cert:    loadedCert,
		key:     loadedKey,
		certPEM: certPEM,
		keyPEM:  keyPEM,
	}, nil
}

func loadCertificateFromSecret(secret *corev1.Secret, name string) (*x509.Certificate, error) {
	certPEM, err := secretDataValue(secret, certFileName(name))
	if err != nil {
		return nil, err
	}
	return cert.LoadCertificatePEM(certPEM)
}

// hasExistingEtcdServerCert checks whether an existing etcd-cert Secret belongs to an internal etcd installation.
func hasExistingEtcdServerCert(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}
	cert, err := secretDataValue(secret, certFileName(options.EtcdServerCertAndKeyName))
	if err != nil || len(cert) == 0 {
		return false
	}
	key, err := secretDataValue(secret, keyFileName(options.EtcdServerCertAndKeyName))
	return err == nil && len(key) > 0
}

func firstSecretDataValue(key string, secrets ...*corev1.Secret) ([]byte, error) {
	for _, secret := range secrets {
		value, err := secretDataValue(secret, key)
		if err == nil {
			return value, nil
		}
	}
	return nil, fmt.Errorf("failed to find non-empty %q in existing certificate Secrets", key)
}

func secretDataValue(secret *corev1.Secret, key string) ([]byte, error) {
	if secret == nil {
		return nil, fmt.Errorf("secret is nil")
	}
	if value, ok := secret.Data[key]; ok && len(value) > 0 {
		return cloneBytes(value), nil
	}
	if value, ok := secret.StringData[key]; ok && value != "" {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("failed to find non-empty %q in Secret %s/%s", key, secret.Namespace, secret.Name)
}

func certFileName(name string) string {
	return fmt.Sprintf("%s.crt", name)
}

func keyFileName(name string) string {
	return fmt.Sprintf("%s.key", name)
}

func cloneBytes(value []byte) []byte {
	return append([]byte(nil), value...)
}
