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

package cache

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructuredscheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestCacheSerializerStreamsJSONLists(t *testing.T) {
	tests := []struct {
		name   string
		object *unstructured.UnstructuredList
	}{
		{
			name: "pods with list metadata",
			object: &unstructured.UnstructuredList{
				Object: map[string]any{
					"apiVersion": "v1", "kind": "PodList",
					"metadata": map[string]any{"resourceVersion": "123", "continue": "next-page"},
				},
				Items: []unstructured.Unstructured{
					{Object: map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "pod-a", "annotations": map[string]any{"example.com/source": "cluster-a"}}}},
					{Object: map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "pod-b"}}},
				},
			},
		},
		{name: "nil items", object: &unstructured.UnstructuredList{Object: map[string]any{"apiVersion": "v1", "kind": "PodList"}}},
		{name: "empty items", object: &unstructured.UnstructuredList{Object: map[string]any{"apiVersion": "v1", "kind": "PodList"}, Items: []unstructured.Unstructured{}}},
		{
			name: "custom resource list",
			object: &unstructured.UnstructuredList{
				Object: map[string]any{"apiVersion": "example.com/v1", "kind": "WidgetList", "extra": "preserved"},
				Items:  []unstructured.Unstructured{{Object: map[string]any{"apiVersion": "example.com/v1", "kind": "Widget", "spec": map[string]any{"count": int64(42), "text": "<value>&中文"}}}},
			},
		},
	}
	upstream := originalCacheSerializer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, accept := range []string{"", "application/json"} {
				var want bytes.Buffer
				encodeNegotiatedObject(t, upstream, accept, "", tt.object, &want)
				got := &writeTrackingBuffer{}
				encodeNegotiatedObject(t, cacheSerializer, accept, "", tt.object, got)
				if !bytes.Equal(got.Bytes(), want.Bytes()) {
					t.Fatalf("LIST response changed for Accept %q:\nwant: %s\ngot: %s", accept, want.Bytes(), got.Bytes())
				}
				if got.writes < 2 || got.maxWrite >= got.Len() {
					t.Fatalf("LIST was buffered as a single write: writes=%d, maxWrite=%d, total=%d", got.writes, got.maxWrite, got.Len())
				}
			}
		})
	}
}

func TestCacheSerializerPreservesOtherFormats(t *testing.T) {
	upstream := originalCacheSerializer()
	pod := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "pod-a"}}}
	list := &unstructured.UnstructuredList{Object: map[string]any{"apiVersion": "v1", "kind": "PodList"}, Items: []unstructured.Unstructured{*pod}}
	for _, tt := range []struct {
		name, accept, query string
		object              runtime.Object
	}{
		{"single object", "application/json", "", pod},
		{"pretty list", "application/json", "?pretty=true", list},
		{"yaml list", "application/yaml", "", list},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var want, got bytes.Buffer
			encodeNegotiatedObject(t, upstream, tt.accept, tt.query, tt.object, &want)
			encodeNegotiatedObject(t, cacheSerializer, tt.accept, tt.query, tt.object, &got)
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatalf("response changed:\nwant: %s\ngot: %s", want.Bytes(), got.Bytes())
			}
		})
	}
	t.Run("watch event", func(t *testing.T) {
		wantInfo := upstream.SupportedMediaTypes()[0].StreamSerializer
		gotInfo := cacheSerializer.SupportedMediaTypes()[0].StreamSerializer
		var want, got bytes.Buffer
		if err := wantInfo.Serializer.Encode(pod, &want); err != nil {
			t.Fatal(err)
		}
		if err := gotInfo.Serializer.Encode(pod, &got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want.Bytes()) || gotInfo.Framer != wantInfo.Framer || gotInfo.EncodesAsText != wantInfo.EncodesAsText {
			t.Fatal("watch serialization changed")
		}
	})
}

func originalCacheSerializer() runtime.NegotiatedSerializer {
	return &customSerializer{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		supportedMediaTypes:  unstructuredscheme.NewUnstructuredNegotiatedSerializer().SupportedMediaTypes(),
	}
}

func encodeNegotiatedObject(t *testing.T, ns runtime.NegotiatedSerializer, accept, query string, object runtime.Object, w io.Writer) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/pods"+query, nil)
	req.Header.Set("Accept", accept)
	_, info, err := negotiation.NegotiateOutputMediaType(req, ns, negotiation.DefaultEndpointRestrictions)
	if err != nil {
		t.Fatal(err)
	}
	encoder := ns.EncoderForVersion(info.Serializer, object.GetObjectKind().GroupVersionKind().GroupVersion())
	if err := encoder.Encode(object, w); err != nil {
		t.Fatal(err)
	}
}

type writeTrackingBuffer struct {
	bytes.Buffer
	writes   int
	maxWrite int
}

func (b *writeTrackingBuffer) Write(p []byte) (int, error) {
	b.writes++
	if len(p) > b.maxWrite {
		b.maxWrite = len(p)
	}
	return b.Buffer.Write(p)
}
