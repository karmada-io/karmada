/*
Copyright 2022 The Karmada Authors.

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
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/karmada-io/karmada/pkg/util/lifted"
)

func TestRequestURL(t *testing.T) {
	tests := []struct {
		name      string
		urlString string
		request   http.Request
		want      string
	}{
		{
			name:      "without slash in the end",
			urlString: "https://0.0.0.0:6443",
			request: http.Request{
				Method: "GET",
				URL: &url.URL{
					Path: "/api/v1/namespaces/test/pods/",
				},
			},
			want: "https://0.0.0.0:6443/api/v1/namespaces/test/pods",
		},
		{
			name:      "with slash in the end",
			urlString: "https://0.0.0.0:6443/",
			request: http.Request{
				Method: "GET",
				URL: &url.URL{
					Path: "/api/v1/namespaces/test/pods/",
				},
			},
			want: "https://0.0.0.0:6443/api/v1/namespaces/test/pods",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyRequestInfo := lifted.NewRequestInfo(&tt.request)
			location, _ := url.Parse(tt.urlString)
			requestURL := requestURLStr(location, proxyRequestInfo)
			require.Equal(t, tt.want, requestURL)
		})
	}
}
