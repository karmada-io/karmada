/*
Copyright 2023 The Karmada Authors.

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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// CertStore is an Interface that define the cert read  and store operator to a cache.
// And we can load a set of certs form a k8s secret.
type CertStore interface {
	AddCert(cert *KarmadaCert)
	GetCert(name string) *KarmadaCert
	CertList() []*KarmadaCert
	LoadCertFromSecret(secret *corev1.Secret) error
}

type splitToPairNameFunc func(name string) string

// SplitToPairName is default function to split cert pair name
// from a secret data key. It only works in this format:
// karmada.crt, karmada.key.
func SplitToPairName(name string) string {
	if strings.Contains(name, keyExtension) {
		strArr := strings.Split(name, keyExtension)
		return strArr[0]
	}

	if strings.Contains(name, certExtension) {
		strArr := strings.Split(name, certExtension)
		return strArr[0]
	}

	return name
}

// KarmadaCertStore is a cache to store karmada certificate. the key is cert baseName by default.
type KarmadaCertStore struct {
	certs        map[string]*KarmadaCert
	pairNameFunc splitToPairNameFunc
}

// NewCertStore returns a cert store. It use default SplitToPairName function to
// get cert pair name form cert file name.
func NewCertStore() CertStore {
	return &KarmadaCertStore{
		certs:        make(map[string]*KarmadaCert),
		pairNameFunc: SplitToPairName,
	}
}

// AddCert adds a cert to cert store, the cache key is cert pairName by default.
func (store *KarmadaCertStore) AddCert(cert *KarmadaCert) {
	store.certs[cert.pairName] = cert
}

// GetCert get cert from store by cert pairName.
func (store *KarmadaCertStore) GetCert(name string) *KarmadaCert {
	for _, c := range store.certs {
		if c.pairName == name {
			return c
		}
	}
	return nil
}

// CertList lists all of karmada certs in the cert cache.
func (store *KarmadaCertStore) CertList() []*KarmadaCert {
	certs := make([]*KarmadaCert, 0, len(store.certs))

	for _, c := range store.certs {
		certs = append(certs, c)
	}

	return certs
}

// LoadCertFromSecret loads a set of certs form k8s secret resource. we get cert
// cache key by calling the pairNameFunc function. if the secret data key suffix is ".crt",
// it be considered cert data. if the suffix is ".key", it be considered cert key data.
func (store *KarmadaCertStore) LoadCertFromSecret(secret *corev1.Secret) error {
	if len(secret.Data) == 0 {
		return fmt.Errorf("cert data is empty")
	}

	for name, data := range secret.Data {
		pairName := store.pairNameFunc(name)
		kc := store.GetCert(pairName)
		if kc == nil {
			kc = &KarmadaCert{
				pairName: pairName,
			}
		}

		if strings.Contains(name, certExtension) {
			kc.cert = data
		}
		if strings.Contains(name, keyExtension) {
			kc.key = data
		}

		store.AddCert(kc)
	}

	return nil
}
