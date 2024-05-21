/*
Copyright 2017 The Kubernetes Authors.

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

package pubkeypin

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

// testCertPEM is a simple self-signed test certificate issued with the openssl CLI:
// openssl req -new -newkey rsa:3072 -days 36500 -nodes -x509 -keyout /dev/null -out test.crt
const testCertPEM = `
-----BEGIN CERTIFICATE-----
MIIEbTCCAtWgAwIBAgIULwXa4OSKT/GklPt0JMn2GUZYClMwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAgFw0yNDA1MjAwMjIzNTdaGA8yMTI0
MDQyNjAyMjM1N1owRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUx
ITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDCCAaIwDQYJKoZIhvcN
AQEBBQADggGPADCCAYoCggGBAMXIMrjc7AKSa1Q/gmJzyXUwg0CNtmbWwz9cxWW3
5DYxRsirnOc9EVMMI6hSl9k1dOh8DQ1uIZLM8EHtSol7o/CP3MCBT6SkaniXpFON
UUZkKY3Yo7t8AOcuRoLRrnye2YrpQOEQ8eb+dXnzibFrJuSw6fXBoXdutmaWWMmN
XPICC1s8l/GxT7jjvm7Y5iVFq+sZco/qxv1ZeBNmUcWWXEtT2KppCBRXk/23OcV2
fvCDS/3bUIgeBxphUnASv8r5W5orbtl/HGgn/uv7LyYDVVWgYVxXuXaW6blc+oLB
bFbiPlfg7EIrcbkV1qBl9SesMPrp8lQH3+PEMCxF6Q0kxDfJteiIUQsWhmyV6/VA
t7XVIU0Dl99zN7WoZLsstjI9/7b+TjBqMRWVTtoAeHMzH9lLx75rTUfAXzcM2Zpy
AsEmlNXzcysyTgqhZg1bQwbHZVzH3ctfMxw/DDzpvyhPfHM5eQp06HPSMhv0v4uZ
pz3mNOxCffRxj4A1v9pWn0YyoQIDAQABo1MwUTAdBgNVHQ4EFgQUvBTsnQ7yih79
Jz7d0VZmSvAnRWAwHwYDVR0jBBgwFoAUvBTsnQ7yih79Jz7d0VZmSvAnRWAwDwYD
VR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAYEAoR0w2y1v4QlE4rQ+LPAi
Am576mClL5gCpSEdBP32htSJOEz4VHeUn/zpGWoO6ezEeMOqosuax2LP2sv62zOj
OyYXxZ6uBV6jz+hvpufgIqnFj9sRckS6wXsDb726fOaKvv1PCdQUQPMgF9sX3vQD
YeWqg0ga1OGfeXdJMdS2AmTidR0m83EIE1PvX2nddhh1xOC0XCoUuwv/O8aSvfqg
iBBDZt0FFMlkSsSkNwbYylmaY0MR11bMCx6OZTCQ43zkcxg9k8ItvxOvSkRjfTlh
QaPa/qIkto4XQCrTHPcMRVSYfIFOi5hpiwXVuC1T/uFDKqvhGAijGX8xTi/FYJV6
Bgm0D8TXpEBqhvS7Fkf9mUdOF8FenGhJrJtY8jqCCsietGC6tuabl0tFVgDV+6nu
0sQVRzYWcvk/21vps+LiO/+EifDbq8KpmEALOWp59kZPYalLmFlyavXHNlpxd9Tm
sOVrkbwM/dtdTqzqQ6YW2uRGfTfmb3pfOBN4H6kiu6RD
-----END CERTIFICATE-----`

// expectedHash can be verified using the openssl CLI.
const expectedHash = `sha256:d597b45a039e09054649e094dc3da2997475827404cf67a886459724c4e35d38`

// testCert2PEM is a second test cert generated the same way as testCertPEM
const testCert2PEM = `
-----BEGIN CERTIFICATE-----
MIIEbTCCAtWgAwIBAgIULEFzXomJO9a0Tv+pC6/7L6voZjEwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAgFw0yNDA1MjAwMjI1MzRaGA8yMTI0
MDQyNjAyMjUzNFowRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUx
ITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDCCAaIwDQYJKoZIhvcN
AQEBBQADggGPADCCAYoCggGBANJPAabiDP66oTZGPajoAtwbJKCpixekZdGX4xlO
X+ymnVeKuawiX/pXQB0ZMg0imQ1KgUTNdHzVJzuPtkJDeGvAwGzgPBFrM3XjUs+1
kl2KlG9WEqLViF/J0NjPDCAJRgQNor0ycjiHSr5dg9QlrCduuaro3SCNfT4xuci7
k2UHxzFhguYXn+Ef+6ZqdtsM9x8aQhWa2Aw3I1yEuOSKT8HmcDkYyeI4XaI299Gz
LFz4+lR72jkAJRA2Dk5pKBwYJz8i3Wmr6wyXUizlSly3796tUMfzgXxnNkCRXaLz
4uiqUatnmCd1/pZ47I4oSUCCjj7Mq7EFfKFmXfCoaXAfNGxMdYtwjl299h2PtKKU
wlBwv1BUJICWkJ+OyB3Tv9YuZqNWoYY2x4qPV3/QWaUgtv5/LKdYR6Ivzc4gwMJD
PWjpb27KtnKrlw6VwrrjfyMEDVkVkibXJWAnoJv0KJ275rIre5vQH4DKFC0isfNR
sLVOKfiyWudqIVFiVDUCufDnPQIDAQABo1MwUTAdBgNVHQ4EFgQU14xkgPnKLiqe
gS/blBgvJTc8qO8wHwYDVR0jBBgwFoAU14xkgPnKLiqegS/blBgvJTc8qO8wDwYD
VR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAYEAFf4Fc5ScyZMqkcoVuYvh
1WISAbM6k+5aHX/KA+Br35Zxh4B3s+NGGG48RXVt4xMs/x9fCU1OpWKLiAzJwSBM
hFidIBshCrrVnVKy+ws1tOAcw4f21k5n0S/Sw+uYWNpamcpD7X3kFpTMIzYGdO5c
hffYHQ+oP3tQTD9GMuNVAlagrMqabk6JZXz36ow+aPASdnCjrA0WpuQ+XeldGc+N
4hfU6rTKdFO8Uu9iaMgd5tFlZdIYEBP+wJ9APKtjjwuolsKcQKaHHLorMnNXO5Ct
TA7YgpW7oic1AMtmD+Z+ucT9OpGHEXQa4TEtmr0fiEAUFjwcwyy7LO7x18buUPt4
Rzq+T+QEdf3BmBL4p5ArKyLcOeZNK4MjPbbjKYnibJBGDYgtrz7GLgF//P4S4/lP
0lmm5cBxIDEaeE6WjQZUrx+TvJFIQ8GTu0vG8h7JDKqO6UbBL8OAMZ2fO9iPXlYH
0EZLIWXX1p8W8w7+dmr0CGpLdascEdxXkf/XYIsAP68K
-----END CERTIFICATE-----
`

// testCert is a small helper to get a test x509.Certificate from the PEM constants
func testCert(t *testing.T, pemString string) *x509.Certificate {
	// Decode the example certificate from a PEM file into a PEM block
	pemBlock, _ := pem.Decode([]byte(pemString))
	if pemBlock == nil {
		t.Fatal("failed to parse test certificate PEM")
		return nil
	}

	// Parse the PEM block into an x509.Certificate
	result, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse test certificate: %v", err)
		return nil
	}
	return result
}

func TestSet(t *testing.T) {
	s := NewSet()
	if !s.Empty() {
		t.Error("expected a new set to be empty")
		return
	}
	err := s.Allow("xyz")
	if err == nil || !s.Empty() {
		t.Error("expected allowing junk to fail")
		return
	}

	err = s.Allow("0011223344")
	if err == nil || !s.Empty() {
		t.Error("expected allowing something too short to fail")
		return
	}

	err = s.Allow(expectedHash + expectedHash)
	if err == nil || !s.Empty() {
		t.Error("expected allowing something too long to fail")
		return
	}

	err = s.CheckAny([]*x509.Certificate{testCert(t, testCertPEM)})
	if err == nil {
		t.Error("expected test cert to not be allowed (yet)")
		return
	}

	err = s.Allow(strings.ToUpper(expectedHash))
	if err != nil || s.Empty() {
		t.Error("expected allowing uppercase expectedHash to succeed")
		return
	}

	err = s.CheckAny([]*x509.Certificate{testCert(t, testCertPEM)})
	if err != nil {
		t.Errorf("expected test cert to be allowed, but got back: %v", err)
		return
	}

	err = s.CheckAny([]*x509.Certificate{testCert(t, testCert2PEM)})
	if err == nil {
		t.Error("expected the second test cert to be disallowed")
		return
	}

	s = NewSet() // keep set empty
	hashes := []string{
		`sha256:0000000000000000000000000000000000000000000000000000000000000000`,
		`sha256:0000000000000000000000000000000000000000000000000000000000000001`,
	}
	err = s.Allow(hashes...)
	if err != nil || len(s.sha256Hashes) != 2 {
		t.Error("expected allowing multiple hashes to succeed")
		return
	}
}

func TestHash(t *testing.T) {
	actualHash := Hash(testCert(t, testCertPEM))
	if actualHash != expectedHash {
		t.Errorf(
			"failed to Hash() to the expected value\n\texpected: %q\n\t  actual: %q",
			expectedHash,
			actualHash,
		)
	}
}
