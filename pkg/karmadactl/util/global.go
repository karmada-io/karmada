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

package util

import (
	"fmt"
	"path/filepath"
)

const (
	// KarmadaConfigFieldName the field stores karmada config in karmada config secret
	KarmadaConfigFieldName = "karmada.config"

	// PKIBaseDir the base directory structure for storing certificates
	PKIBaseDir = "/etc/karmada/pki"

	// Server is a constant for server type.
	Server = "server"
	// Client is a constant for client type.
	Client = "client"
	// Etcd is a constant for Etcd type.
	Etcd = "etcd"
	// EtcdClient is a constant for etcd client type.
	EtcdClient = "etcd-client"
	// FrontProxyClient is a constant for front proxy client type.
	FrontProxyClient = "front-proxy-client"
	// ServiceAccountKeyPair is a constant for service account key pair type.
	ServiceAccountKeyPair = "service-account-key-pair"
	// CA is a constant for certificate authority type.
	CA = "ca"
	// SchedulerEstimatorClient is the client type for scheduler estimator.
	SchedulerEstimatorClient = "scheduler-estimator-client"

	// TLSCrtFile is a constant for the TLS certificate file name.
	TLSCrtFile = "tls.crt"
	// TLSKeyFile is a constant for the TLS key file name.
	TLSKeyFile = "tls.key"
	// CACrtFile is a constant for the CA certificate file name.
	CACrtFile = "ca.crt"
	// SAKeyFile is a constant for the Service Account key file name.
	SAKeyFile = "sa.key"
	// SAPubFile is a constant for the Service Account public key file name.
	SAPubFile = "sa.pub"

	// CertSuffix is a constant for the certificate suffix.
	CertSuffix = "-cert"
)

// KarmadaConfigName returns the name of karmada config secret
func KarmadaConfigName(component string) string {
	return component + "-config"
}

// GetDirPath returns the full directory path for a certificate type
func GetDirPath(certType string) string {
	return filepath.Join(PKIBaseDir, certType)
}

// GetFilePath returns the full path for a certificate file
func GetFilePath(certType, filename string) string {
	return filepath.Join(GetDirPath(certType), filename)
}

// GetCertName returns the certificate name for a base name
func GetCertName(baseName string) string {
	return baseName + CertSuffix
}

// GetComponentCertName returns the certificate name for a component
func GetComponentCertName(component string) string {
	return GetCertName(component)
}

// GetComponentClientCertName returns the client certificate name for a component and client type
func GetComponentClientCertName(component, clientType string) string {
	return fmt.Sprintf("%s-%s%s", component, clientType, CertSuffix)
}
