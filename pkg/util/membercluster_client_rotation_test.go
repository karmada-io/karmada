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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

type rotatableTokenServer struct {
	*httptest.Server
	accepted atomic.Value // string
	lastAuth atomic.Value // string
}

func newRotatableTokenServer(initialToken string) *rotatableTokenServer {
	s := &rotatableTokenServer{}
	s.accepted.Store(initialToken)
	s.lastAuth.Store("")
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		s.lastAuth.Store(auth)
		if auth != "Bearer "+s.accepted.Load().(string) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Node","metadata":{"name":"foo"}}`))
	}))
	return s
}

func (s *rotatableTokenServer) accept(token string) { s.accepted.Store(token) }
func (s *rotatableTokenServer) seenAuth() string    { return s.lastAuth.Load().(string) }

const (
	testTokenCacheTTL   = 10 * time.Millisecond
	tokenRotateTimeout  = 100 * time.Millisecond
	tokenRotatePollRate = 5 * time.Millisecond
)

// TestBuildClusterConfig_LongLivedClientPicksUpRotatedToken checks that a client built once (as
// informers are) picks up a rotated token on its own. It:
//  1. builds the client once,
//  2. rotates the token in the Secret and the fake API server,
//  3. asserts the client sends the new token without being rebuilt.
func TestBuildClusterConfig_LongLivedClientPicksUpRotatedToken(t *testing.T) {
	origTTL := tokenCacheTTL
	tokenCacheTTL = testTokenCacheTTL
	t.Cleanup(func() { tokenCacheTTL = origTTL })

	const clusterName = "member-rotate"

	srv := newRotatableTokenServer("token-A")
	defer srv.Close()

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
				clusterv1alpha1.SecretTokenKey:  []byte("token-A"),
				clusterv1alpha1.SecretCADataKey: getCACertFromGTestServer(t, srv.Server),
			},
		},
	).Build()

	clusterClient, err := NewClusterClientSet(clusterName, hostClient, nil)
	assert.NoError(t, err)

	_, err = clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
	assert.NoError(t, err, "baseline request with token-A should succeed")

	// Rotate: update Secret and reject old token on the server.
	secret := &corev1.Secret{}
	assert.NoError(t, hostClient.Get(context.TODO(), types.NamespacedName{Namespace: "ns1", Name: "secret1"}, secret))
	secret.Data[clusterv1alpha1.SecretTokenKey] = []byte("token-B")
	assert.NoError(t, hostClient.Update(context.TODO(), secret))
	srv.accept("token-B")

	assert.Eventually(t, func() bool {
		_, err := clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
		return err == nil
	}, tokenRotateTimeout, tokenRotatePollRate,
		"after rotation the long-lived client must pick up token-B without a rebuild")

	assert.Equal(t, "Bearer token-B", srv.seenAuth(),
		"rotated token must appear on the wire")
}
