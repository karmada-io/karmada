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

package certificate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	certificatesv1 "k8s.io/api/certificates/v1"
)

func TestGetCertApprovalCondition(t *testing.T) {
	testItems := []struct {
		name     string
		status   *certificatesv1.CertificateSigningRequestStatus
		approved bool
		denied   bool
	}{
		{
			name: "csr has been approved",
			status: &certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{
					{Type: certificatesv1.CertificateApproved},
				},
			},
			approved: true,
			denied:   false,
		},
		{
			name: "csr has been denied",
			status: &certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{
					{Type: certificatesv1.CertificateDenied},
				},
			},
			approved: false,
			denied:   true,
		},
		{
			name: "the signer failed to issue the certificate",
			status: &certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{
					{Type: certificatesv1.CertificateFailed},
				},
			},
			approved: false,
			denied:   false,
		},
		{
			name: "csr with no conditions",
			status: &certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{},
			},
			approved: false,
			denied:   false,
		},
	}

	for _, item := range testItems {
		approved, denied := GetCertApprovalCondition(item.status)
		assert.Equal(t, item.approved, approved)
		assert.Equal(t, item.denied, denied)
	}
}
