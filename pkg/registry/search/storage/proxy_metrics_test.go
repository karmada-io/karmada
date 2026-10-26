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

package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/metrics"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/component-base/metrics/legacyregistry"

	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	karmadainformers "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/search/proxy"
	proxytest "github.com/karmada-io/karmada/pkg/search/proxy/testing"
	"github.com/karmada-io/karmada/pkg/util/names"
	utiltest "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestProxyingREST_Connect_Metrics(t *testing.T) {
	metrics.Register()
	t.Cleanup(metrics.Reset)

	ctl, err := proxy.NewController(proxy.NewControllerOption{
		RestConfig: &restclient.Config{
			Host: "https://karmada.example",
			Transport: proxyMetricsTransport(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Proxied-Uri": {req.URL.RequestURI()}},
					Body:       io.NopCloser(strings.NewReader("{}")),
				}, nil
			}),
		},
		RestMapper:     proxytest.RestMapper,
		KubeFactory:    informers.NewSharedInformerFactory(fake.NewClientset(), 0),
		KarmadaFactory: karmadainformers.NewSharedInformerFactory(karmadafake.NewClientset(), 0),
	})
	require.NoError(t, err)
	ctl.Start(t.Context())
	t.Cleanup(ctl.Stop)
	r := NewProxyingREST(ctl)
	resolver := &genericrequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}

	tests := []struct {
		name  string
		path  string
		verb  string
		scope string
	}{
		{name: "list all namespaces", path: "/api/v1/pods", verb: "LIST", scope: "cluster"},
		{name: "list one namespace", path: "/api/v1/namespaces/default/pods", verb: "LIST", scope: "namespace"},
		{name: "get one pod", path: "/api/v1/namespaces/default/pods/test", verb: "GET", scope: "resource"},
		{name: "watch pods", path: "/api/v1/pods?watch=true", verb: "WATCH", scope: "cluster"},
	}

	// The apiserver metrics registry is global, so keep cases sequential and isolated.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.Reset()
			req := httptest.NewRequest(http.MethodGet, "/apis/search.karmada.io/v1alpha1/proxying/karmada/proxy"+tt.path, nil)
			info, err := resolver.NewRequestInfo(req)
			require.NoError(t, err)
			req = req.WithContext(genericrequest.WithRequestInfo(t.Context(), info))

			recorder := httptest.NewRecorder()
			handler, err := r.Connect(req.Context(), "karmada", nil, utiltest.NewResponder(recorder))
			require.NoError(t, err)
			handler.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, tt.path, recorder.Header().Get("X-Proxied-Uri"))

			families, err := legacyregistry.DefaultGatherer.Gather()
			require.NoError(t, err)
			found := map[string]bool{
				"apiserver_request_total":            false,
				"apiserver_request_duration_seconds": false,
			}
			for _, family := range families {
				metricName := family.GetName()
				if _, ok := found[metricName]; !ok {
					continue
				}
				found[metricName] = true
				require.Len(t, family.Metric, 1, metricName)
				metric := family.Metric[0]
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				wantLabels := map[string]string{
					"component":   names.KarmadaSearchComponentName,
					"dry_run":     "",
					"group":       "",
					"version":     "v1",
					"resource":    "pods",
					"subresource": "",
					"scope":       tt.scope,
					"verb":        tt.verb,
				}
				if metricName == "apiserver_request_total" {
					wantLabels["code"] = "200"
					assert.Equal(t, float64(1), metric.GetCounter().GetValue())
				} else {
					assert.Equal(t, uint64(1), metric.GetHistogram().GetSampleCount())
				}
				assert.Equal(t, wantLabels, labels, metricName)
			}
			for metricName, present := range found {
				assert.True(t, present, "missing metric %s", metricName)
			}
		})
	}
}

type proxyMetricsTransport func(*http.Request) (*http.Response, error)

func (f proxyMetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
