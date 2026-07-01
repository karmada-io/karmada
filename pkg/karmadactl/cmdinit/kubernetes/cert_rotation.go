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
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	certConfigs, err := i.buildInitCertConfigs()
	if err != nil {
		return err
	}

	i.CertAndKeyFileData = map[string][]byte{}
	if err := i.prepareRotatedCertAndKeyData(certConfigs); err != nil {
		return err
	}

	if err := i.updateCertsSecrets(); err != nil {
		return err
	}

	klog.Infof("Certificate Secrets in namespace %q have been updated. Restart Karmada control plane components to load the rotated certificates.", i.Namespace)
	return nil
}

// prepareRotatedCertAndKeyData rebuilds CertAndKeyFileData from existing CA material.
func (i *CommandInitOption) prepareRotatedCertAndKeyData(certConfigs *initCertConfigs) error {
	karmadaSecret, err := i.getExistingSecret(globaloptions.KarmadaCertsName)
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

	if err := i.signLeafCert(options.KarmadaCertAndKeyName, rootCA, certConfigs.karmada); err != nil {
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

	if i.ExternalEtcdCACertPath != "" {
		certPEM, err := os.ReadFile(i.ExternalEtcdCACertPath)
		if err != nil {
			return err
		}
		i.setCertAndKeyData(options.EtcdCaCertAndKeyName, certPEM, emptyByteSlice)
	} else {
		certPEM, err := firstSecretDataValue(certFileName(options.EtcdCaCertAndKeyName), etcdSecret, karmadaSecret)
		if err != nil {
			return err
		}
		i.setCertAndKeyData(options.EtcdCaCertAndKeyName, certPEM, emptyByteSlice)
	}

	i.setCertAndKeyData(options.EtcdServerCertAndKeyName, emptyByteSlice, emptyByteSlice)

	if i.ExternalEtcdClientCertPath != "" && i.ExternalEtcdClientKeyPath != "" {
		if _, err := i.readExternalEtcdCert(options.EtcdClientCertAndKeyName); err != nil {
			return err
		}
		return nil
	}

	clientCert, err := firstSecretDataValue(certFileName(options.EtcdClientCertAndKeyName), karmadaSecret)
	if err != nil {
		return err
	}
	clientKey, err := firstSecretDataValue(keyFileName(options.EtcdClientCertAndKeyName), karmadaSecret)
	if err != nil {
		return err
	}
	i.setCertAndKeyData(options.EtcdClientCertAndKeyName, clientCert, clientKey)
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
