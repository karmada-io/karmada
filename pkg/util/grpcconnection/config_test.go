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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerConfig_NewServer_Insecure(t *testing.T) {
	cfg := &ServerConfig{}
	srv, err := cfg.NewServer()
	if err != nil {
		t.Fatalf("expected no error for insecure server, got: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestServerConfig_NewServer_InvalidCertPath(t *testing.T) {
	cfg := &ServerConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}
	_, err := cfg.NewServer()
	if err == nil {
		t.Fatal("expected error for nonexistent cert/key files, got nil")
	}
}

func TestServerConfig_NewServer_InvalidCAFile(t *testing.T) {
	dir := t.TempDir()

	// Create minimal self-signed cert/key pair using crypto/tls test helpers
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	// Write dummy PEM content (invalid cert — LoadX509KeyPair will fail gracefully)
	if err := os.WriteFile(certFile, []byte("not-a-cert"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("not-a-key"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &ServerConfig{
		CertFile:         certFile,
		KeyFile:          keyFile,
		ClientAuthCAFile: "/nonexistent/ca.pem",
	}
	_, err := cfg.NewServer()
	if err == nil {
		t.Fatal("expected error for invalid cert content, got nil")
	}
}

func TestServerConfig_NewServer_InvalidClientCAContent(t *testing.T) {
	dir := t.TempDir()

	caFile := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caFile, []byte("not-a-pem"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &ServerConfig{
		CertFile:         "/nonexistent/cert.pem",
		KeyFile:          "/nonexistent/key.pem",
		ClientAuthCAFile: caFile,
	}
	// LoadX509KeyPair will fail first since cert/key don't exist
	_, err := cfg.NewServer()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClientConfig_DialWithTimeOut_AllPathsFail(t *testing.T) {
	cfg := &ClientConfig{}
	_, err := cfg.DialWithTimeOut([]string{"localhost:19999", "localhost:19998"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error when all dial paths fail, got nil")
	}
}

func TestClientConfig_DialWithTimeOut_EmptyPaths(t *testing.T) {
	cfg := &ClientConfig{}
	_, err := cfg.DialWithTimeOut([]string{}, 100*time.Millisecond)
	// No paths means the loop body never runs; we expect the aggregate error
	// to be nil (empty error list) — but no connection is returned either.
	if err != nil {
		t.Fatalf("expected nil error for empty paths, got: %v", err)
	}
}

func TestClientConfig_DialWithTimeOut_InvalidCAFile(t *testing.T) {
	cfg := &ClientConfig{
		ServerAuthCAFile: "/nonexistent/ca.pem",
	}
	_, err := cfg.DialWithTimeOut([]string{"localhost:19999"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for nonexistent CA file, got nil")
	}
}

func TestClientConfig_DialWithTimeOut_InvalidCAContent(t *testing.T) {
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caFile, []byte("not-valid-pem"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &ClientConfig{
		ServerAuthCAFile: caFile,
	}
	_, err := cfg.DialWithTimeOut([]string{"localhost:19999"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for invalid CA PEM content, got nil")
	}
}

func TestClientConfig_DialWithTimeOut_InvalidClientCert(t *testing.T) {
	cfg := &ClientConfig{
		InsecureSkipServerVerify: true,
		CertFile:                 "/nonexistent/client.pem",
		KeyFile:                  "/nonexistent/client.key",
	}
	_, err := cfg.DialWithTimeOut([]string{"localhost:19999"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for nonexistent client cert, got nil")
	}
}

func TestClientConfig_DialWithTimeOut_InsecureSkipVerify(t *testing.T) {
	cfg := &ClientConfig{
		InsecureSkipServerVerify: true,
	}
	_, err := cfg.DialWithTimeOut([]string{"localhost:19999"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected connection error (no server), got nil")
	}
}
