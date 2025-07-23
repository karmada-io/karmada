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

package testing

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// GenerateTestCACertificate generates a self-signed CA certificate and its associated RSA private key.
// The certificate and private key are returned as PEM-encoded strings.
func GenerateTestCACertificate() (string, string, error) {
	// Generate a new RSA private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Set the certificate parameters.
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", "", err
	}

	cert := &x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Create the certificate.
	certDER, err := x509.CreateCertificate(rand.Reader, cert, cert, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	// PEM encode the certificate.
	certPEM := &pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEMData := pem.EncodeToMemory(certPEM)

	// PEM encode the private key.
	privKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
	privKeyPEMData := pem.EncodeToMemory(privKeyPEM)

	return string(certPEMData), string(privKeyPEMData), nil
}

// GetFreePorts attempts to find n available TCP ports on the specified host. It
// returns a slice of allocated port numbers or an error if it fails to acquire
// them.
func GetFreePorts(host string, n int) ([]int, error) {
	ports := make([]int, 0, n)
	listeners := make([]net.Listener, 0, n)

	// Make sure we close all listeners if there's an error.
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	for i := 0; i < n; i++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", host))
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, listener)
		tcpAddr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			return nil, fmt.Errorf("listener address is not a *net.TCPAddr")
		}
		ports = append(ports, tcpAddr.Port)
	}

	// At this point we have all ports, so we can close the listeners.
	for _, l := range listeners {
		l.Close()
	}
	return ports, nil
}
