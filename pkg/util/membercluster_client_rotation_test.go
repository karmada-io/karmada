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

package util

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

// rotatableTokenServer is a fake member API server that accepts a mutable set of
// bearer tokens. A request whose token is not currently accepted gets a 401,
// which lets tests exercise both the periodic-refresh and 401-reset code paths.
type rotatableTokenServer struct {
	*httptest.Server

	mu           sync.Mutex
	accepted     map[string]bool
	unauthorized atomic.Int32
	lastAuth     atomic.Value // string
}

func newRotatableTokenServer(acceptedTokens ...string) *rotatableTokenServer {
	s := &rotatableTokenServer{accepted: map[string]bool{}}
	for _, tok := range acceptedTokens {
		s.accepted[tok] = true
	}
	s.lastAuth.Store("")
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		s.lastAuth.Store(auth)

		token := strings.TrimPrefix(auth, "Bearer ")
		s.mu.Lock()
		ok := s.accepted[token]
		s.mu.Unlock()
		if !ok {
			s.unauthorized.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Node","metadata":{"name":"foo"}}`))
	}))
	return s
}

// accept replaces the set of tokens the server will accept.
func (s *rotatableTokenServer) accept(tokens ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accepted = map[string]bool{}
	for _, tok := range tokens {
		s.accepted[tok] = true
	}
}

func (s *rotatableTokenServer) seenAuth() string { return s.lastAuth.Load().(string) }

const (
	tokenRotateTimeout  = 2 * time.Second
	tokenRotatePollRate = 5 * time.Millisecond
)

// buildRotationClient builds a long-lived client (as the informers do) pointed at
// srv, backed by a Secret holding initialToken. It returns the client and the
// host client so the test can rotate the Secret afterwards.
func buildRotationClient(t *testing.T, srv *rotatableTokenServer, clusterName, initialToken string) (*ClusterClient, client.Client) {
	t.Helper()
	hostClient := fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: srv.URL,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data: map[string][]byte{
				clusterv1alpha1.SecretTokenKey:  []byte(initialToken),
				clusterv1alpha1.SecretCADataKey: getCACertFromGTestServer(t, srv.Server),
			},
		},
	).Build()

	clusterClient, err := NewClusterClientSet(clusterName, hostClient, nil)
	assert.NoError(t, err)
	return clusterClient, hostClient
}

// rotateSecretToken updates the token stored in the Karmada Secret.
func rotateSecretToken(t *testing.T, hostClient client.Client, newToken string) {
	t.Helper()
	secret := &corev1.Secret{}
	assert.NoError(t, hostClient.Get(context.TODO(), types.NamespacedName{Namespace: "ns1", Name: "secret1"}, secret))
	secret.Data[clusterv1alpha1.SecretTokenKey] = []byte(newToken)
	assert.NoError(t, hostClient.Update(context.TODO(), secret))
}

// TestBuildClusterConfig_PeriodicRefreshPicksUpRotatedToken verifies the common
// rotation case: the new token is written while the old one is still valid
// (overlap), so the periodic re-read swaps to it with no 401 ever occurring.
func TestBuildClusterConfig_PeriodicRefreshPicksUpRotatedToken(t *testing.T) {
	// Short period so the cached token expires quickly within the test.
	orig := tokenRefreshPeriod
	tokenRefreshPeriod = 10 * time.Millisecond
	t.Cleanup(func() { tokenRefreshPeriod = orig })

	// Server accepts both tokens for the whole test: models an overlap window.
	srv := newRotatableTokenServer("token-A", "token-B")
	defer srv.Close()

	clusterClient, host := buildRotationClient(t, srv, "member-periodic", "token-A")

	_, err := clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
	assert.NoError(t, err, "baseline request with token-A should succeed")

	rotateSecretToken(t, host, "token-B")

	assert.Eventually(t, func() bool {
		_, err := clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
		return err == nil && srv.seenAuth() == "Bearer token-B"
	}, tokenRotateTimeout, tokenRotatePollRate,
		"after the period elapses the long-lived client must re-read and send token-B without a rebuild")

	assert.Zero(t, srv.unauthorized.Load(), "with an overlap window no request should ever be rejected")
}

// TestBuildClusterConfig_Recovers401AfterHardRevocation verifies the
// hard-revocation case: the old token is rejected immediately (no overlap). The
// period is long so recovery cannot come from a periodic re-read; it must come
// from the 401-triggered cache reset. Recovery needs a second request because the
// 401'd request itself is not retried inline.
func TestBuildClusterConfig_Recovers401AfterHardRevocation(t *testing.T) {
	// Long period so the periodic path does not fire during the test.
	orig := tokenRefreshPeriod
	tokenRefreshPeriod = 10 * time.Minute
	t.Cleanup(func() { tokenRefreshPeriod = orig })

	srv := newRotatableTokenServer("token-A")
	defer srv.Close()

	clusterClient, host := buildRotationClient(t, srv, "member-revoke", "token-A")

	_, err := clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
	assert.NoError(t, err, "baseline request with token-A should succeed and warm the cache")

	// Hard revocation: server rejects token-A and only accepts token-B.
	srv.accept("token-B")
	rotateSecretToken(t, host, "token-B")

	assert.Eventually(t, func() bool {
		_, err := clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
		return err == nil && srv.seenAuth() == "Bearer token-B"
	}, tokenRotateTimeout, tokenRotatePollRate,
		"a 401 must reset the cached token so a subsequent request re-reads and sends token-B")

	assert.GreaterOrEqual(t, srv.unauthorized.Load(), int32(1),
		"recovery must go through at least one 401 (the reset path), not a periodic refresh")
}

func TestSecretTokenSource_Token(t *testing.T) {
	const clusterName = "member1"

	t.Run("success returns token with a future expiry", func(t *testing.T) {
		getter := func(string, string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("tok")}}, nil
		}
		src := newSecretTokenSource(clusterName, "ns", "name", getter)

		before := time.Now()
		tok, err := src.Token()
		assert.NoError(t, err)
		assert.Equal(t, "tok", tok.AccessToken)
		assert.WithinDuration(t, before.Add(tokenRefreshPeriod), tok.Expiry, time.Second,
			"expiry must be stamped ~tokenRefreshPeriod in the future so the cache re-reads")
	})

	t.Run("getter error is propagated and names the cluster", func(t *testing.T) {
		getter := func(string, string) (*corev1.Secret, error) { return nil, errors.New("temporarily unavailable") }
		src := newSecretTokenSource(clusterName, "ns", "name", getter)

		_, err := src.Token()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), clusterName)
	})

	t.Run("missing token key errors", func(t *testing.T) {
		getter := func(string, string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: map[string][]byte{}}, nil
		}
		src := newSecretTokenSource(clusterName, "ns", "name", getter)

		_, err := src.Token()
		assert.Error(t, err)
	})

	t.Run("empty token value errors", func(t *testing.T) {
		getter := func(string, string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("")}}, nil
		}
		src := newSecretTokenSource(clusterName, "ns", "name", getter)

		_, err := src.Token()
		assert.Error(t, err)
	})
}
