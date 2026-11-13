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

package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/component-base/metrics/legacyregistry"

	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestController_Connect_Metrics(t *testing.T) {
	metrics.Register()
	t.Cleanup(metrics.Reset)

	ctl := &Controller{
		proxy:                pluginruntime.NewFramework(convertPluginSlice([]*mockPlugin{{IsSupportRequest: true}})),
		negotiatedSerializer: scheme.Codecs.WithoutConversion(),
	}

	tests := []struct {
		name      string
		proxyPath string
		query     string
		verb      string
		scope     string
	}{
		{name: "list all namespaces", proxyPath: "/api/v1/pods", verb: "LIST", scope: "cluster"},
		{name: "list one namespace", proxyPath: "/api/v1/namespaces/default/pods", verb: "LIST", scope: "namespace"},
		{name: "get one pod", proxyPath: "/api/v1/namespaces/default/pods/test", verb: "GET", scope: "resource"},
		{name: "watch pods", proxyPath: "/api/v1/pods", query: "?watch=true", verb: "WATCH", scope: "cluster"},
	}

	// The apiserver metrics registry is global, so keep cases sequential and isolated.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.Reset()
			handler, err := ctl.Connect(t.Context(), tt.proxyPath, nil)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/prefix"+tt.proxyPath+tt.query, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code)

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
