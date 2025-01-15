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

package grpcconnection

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// ServerConfig the config of GRPC server side.
type ServerConfig struct {
	// ServerPort The secure port on which to serve gRPC.
	ServerPort int
	// InsecureSkipClientVerify Controls whether verifies the client's certificate chain and host name.
	// When this is set to false, server will check all incoming HTTPS requests for a client certificate signed by the trusted CA,
	// requests that don’t supply a valid client certificate will fail. If authentication is enabled,
	// the certificate provides credentials for the user name given by the Common Name field.
	InsecureSkipClientVerify bool
	// ClientAuthCAFile SSL Certificate Authority file used to verify grpc client certificates on incoming requests.
	ClientAuthCAFile string
	// CertFile SSL certification file used for grpc SSL/TLS connections.
	CertFile string
	// KeyFile SSL key file used for grpc SSL/TLS connections.
	KeyFile string
}

// ClientConfig the config of GRPC client side.
type ClientConfig struct {
	// TargetPort the target port on which to establish a gRPC connection.
	TargetPort int
	// InsecureSkipServerVerify controls whether a client verifies the server's
	// certificate chain and host name. If InsecureSkipServerVerify is true, crypto/tls
	// accepts any certificate presented by the server and any host name in that
	// certificate. In this mode, TLS is susceptible to machine-in-the-middle
	// attacks unless custom verification is used. This should be used only for
	// testing or in combination with VerifyConnection or VerifyPeerCertificate.
	InsecureSkipServerVerify bool
	// ServerAuthCAFile SSL Certificate Authority file used to verify grpc server certificates.
	ServerAuthCAFile string
	// SSL certification file used for grpc SSL/TLS connections.
	CertFile string
	// SSL key file used for grpc SSL/TLS connections.
	KeyFile string
}

// NewServer creates a gRPC server which has no service registered and has not
// started to accept requests yet.
func (s *ServerConfig) NewServer() (*grpc.Server, error) {
	if s.CertFile == "" || s.KeyFile == "" {
		return grpc.NewServer(), nil
	}

	cert, err := tls.LoadX509KeyPair(s.CertFile, s.KeyFile)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}

	if s.ClientAuthCAFile != "" {
		certPool := x509.NewCertPool()
		ca, err := os.ReadFile(s.ClientAuthCAFile)
		if err != nil {
			return nil, err
		}
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("failed to append ca into certPool")
		}
		config.ClientCAs = certPool
		if !s.InsecureSkipClientVerify {
			config.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}

	return grpc.NewServer(grpc.Creds(grpccredentials.NewTLS(config))), nil
}

// DialWithTimeOut will attempt to create a client connection based on the given targets, one at a time, until a client connection is successfully established.
func (c *ClientConfig) DialWithTimeOut(paths []string, timeout time.Duration) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	var cred grpccredentials.TransportCredentials
	if c.ServerAuthCAFile == "" && !c.InsecureSkipServerVerify {
		// insecure connection
		cred = insecure.NewCredentials()
	} else {
		// server-side TLS
		config := &tls.Config{InsecureSkipVerify: c.InsecureSkipServerVerify} // nolint:gosec // G402: TLS InsecureSkipEstimatorVerify may be true.
		if c.ServerAuthCAFile != "" {
			certPool := x509.NewCertPool()
			ca, err := os.ReadFile(c.ServerAuthCAFile)
			if err != nil {
				return nil, err
			}
			if ok := certPool.AppendCertsFromPEM(ca); !ok {
				return nil, fmt.Errorf("failed to append ca certs")
			}
			config.RootCAs = certPool
		}
		if c.CertFile != "" && c.KeyFile != "" {
			// mutual TLS
			certificate, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
			if err != nil {
				return nil, err
			}
			config.Certificates = []tls.Certificate{certificate}
		}
		cred = grpccredentials.NewTLS(config)
	}
	opts = append(opts, grpc.WithTransportCredentials(cred))
	var cc *grpc.ClientConn
	var err error
	var allErrs []error
	for _, path := range paths {
		cc, err = createGRPCConnection(path, timeout, opts...)
		if err == nil {
			return cc, nil
		}
		allErrs = append(allErrs, fmt.Errorf("dial %s error: %v", path, err))
	}

	return nil, utilerrors.NewAggregate(allErrs)
}

func createGRPCConnection(path string, timeout time.Duration, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cc, err := grpc.NewClient(path, opts...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			cc.Close()
		}
	}()

	// A blocking dial blocks until the clientConn is ready.
	for {
		state := cc.GetState()
		if state == connectivity.Idle {
			cc.Connect()
		}
		if state == connectivity.Ready {
			return cc, nil
		}
		if !cc.WaitForStateChange(ctx, state) {
			return nil, fmt.Errorf("timeout waiting for connection to %s, state is %s", path, state)
		}
	}
}
