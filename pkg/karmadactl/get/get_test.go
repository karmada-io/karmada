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

package get

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

func TestWatchEmptyWatchObjs(t *testing.T) {
	g := &CommandGetOptions{}
	err := g.watch(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not to find obj that is watched")

	err = g.watch([]WatchObj{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not to find obj that is watched")
}

func TestGetWatchMapping(t *testing.T) {
	g := &CommandGetOptions{}

	// Case 1: single resource via watchObjs[0].r.ResourceMapping()
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     cmdtesting.DefaultHeader(),
			Body:       cmdtesting.ObjBody(codec, &corev1.PodList{}),
		},
	}

	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{
		{
			Cluster: "Karmada",
			r:       r,
		},
	}

	mapping, err := g.getWatchMapping(watchObjs, nil)
	assert.NoError(t, err)
	assert.NotNil(t, mapping)
	assert.Equal(t, "pods", mapping.Resource.Resource)

	// Case 2: mixed resource request on watchObjs[0].r returns an error
	tf2 := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf2.Cleanup()

	tf2.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     cmdtesting.DefaultHeader(),
			Body:       cmdtesting.ObjBody(codec, &corev1.PodList{}),
		},
	}

	rMixed := tf2.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods,services").
		Do()

	watchObjsMixed := []WatchObj{
		{
			Cluster: "Karmada",
			r:       rMixed,
		},
	}
	_, err = g.getWatchMapping(watchObjsMixed, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get resource mapping")

	// Case 3: fallback when watchObjs has no r, uses infos[0].ResourceMapping()
	podMapping := &meta.RESTMapping{
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
	}
	info := &resource.Info{
		Mapping: podMapping,
	}
	mapping, err = g.getWatchMapping(nil, []*resource.Info{info})
	assert.NoError(t, err)
	assert.Equal(t, podMapping, mapping)

	// Case 4: both infos and watchObjs empty
	_, err = g.getWatchMapping(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no resource mapping found")
}

func TestWatchEmptyResourceCollection(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("watch") == "true" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body: cmdtesting.ObjBody(codec, &corev1.PodList{
					ListMeta: metav1.ListMeta{
						ResourceVersion: "100",
					},
				}),
			}, nil
		}),
	}

	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{
		{
			Cluster: "Karmada",
			r:       r,
		},
	}

	streams, _, buf, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	g.IsHumanReadablePrinter = true
	g.ToPrinter = g.getResourcePrinter()

	err := g.watch(watchObjs)
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "CLUSTER")
	assert.Equal(t, 1, strings.Count(output, "NAME"), "Header NAME should appear exactly once: %s", output)
}

func TestWatchEmptyResourceCollection_NoHeaders(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("watch") == "true" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       io.NopCloser(bytes.NewReader([]byte{})),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body: cmdtesting.ObjBody(codec, &corev1.PodList{
					ListMeta: metav1.ListMeta{
						ResourceVersion: "100",
					},
				}),
			}, nil
		}),
	}

	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{
		{
			Cluster: "Karmada",
			r:       r,
		},
	}

	streams, _, buf, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	g.IsHumanReadablePrinter = true
	noHeaders := true
	g.PrintFlags.NoHeaders = &noHeaders
	g.NoHeaders = true
	g.ToPrinter = g.getResourcePrinter()

	err := g.watch(watchObjs)
	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestWatchEmptyResourceCollection_WithWatchEvent(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foo",
			Namespace:       "default",
			ResourceVersion: "101",
		},
	}
	podRaw, _ := json.Marshal(pod)
	eventTable := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "meta.k8s.io/v1",
			Kind:       "Table",
		},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string"},
		},
		Rows: []metav1.TableRow{
			{
				Cells: []any{"foo"},
				Object: runtime.RawExtension{
					Raw: podRaw,
				},
			},
		},
	}
	tableRaw, _ := json.Marshal(eventTable)
	var tableMap map[string]any
	_ = json.Unmarshal(tableRaw, &tableMap)

	watchEvent := metav1.WatchEvent{
		Type: string(watch.Added),
		Object: runtime.RawExtension{
			Object: &unstructured.Unstructured{Object: tableMap},
		},
	}
	watchBody, _ := json.Marshal(watchEvent)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("watch") == "true" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       io.NopCloser(bytes.NewReader(append(watchBody, '\n'))),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body: cmdtesting.ObjBody(codec, &corev1.PodList{
					ListMeta: metav1.ListMeta{
						ResourceVersion: "100",
					},
				}),
			}, nil
		}),
	}

	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{
		{
			Cluster: "member1",
			r:       r,
		},
	}

	streams, _, buf, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	g.IsHumanReadablePrinter = true
	g.ToPrinter = g.getResourcePrinter()

	err := g.watch(watchObjs)
	assert.NoError(t, err)
	output := buf.String()
	assert.Equal(t, 1, strings.Count(output, "NAME"), "Header NAME should appear exactly once, not duplicated: %s", output)
	assert.Contains(t, output, "foo")
	assert.Contains(t, output, "member1")
}

func TestWatchMultiClusterObj_ErrorPropagation(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("watch") == "true" {
				return nil, fmt.Errorf("connection refused")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body: cmdtesting.ObjBody(codec, &corev1.PodList{
					ListMeta: metav1.ListMeta{
						ResourceVersion: "100",
					},
				}),
			}, nil
		}),
	}

	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{
		{
			Cluster: "failedCluster",
			r:       r,
		},
	}

	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	g.IsHumanReadablePrinter = true
	g.ToPrinter = g.getResourcePrinter()

	err := g.watch(watchObjs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failedCluster")
}

func TestMultipleGVKsRequested(t *testing.T) {
	tests := []struct {
		name     string
		objs     []Obj
		expected bool
	}{
		{
			name:     "empty objs",
			objs:     []Obj{},
			expected: false,
		},
		{
			name: "single obj",
			objs: []Obj{
				{
					Info: &resource.Info{
						Mapping: &meta.RESTMapping{
							GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "multiple objs with same GVK",
			objs: []Obj{
				{
					Info: &resource.Info{
						Mapping: &meta.RESTMapping{
							GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						},
					},
				},
				{
					Info: &resource.Info{
						Mapping: &meta.RESTMapping{
							GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "multiple objs with different GVKs",
			objs: []Obj{
				{
					Info: &resource.Info{
						Mapping: &meta.RESTMapping{
							GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
						},
					},
				},
				{
					Info: &resource.Info{
						Mapping: &meta.RESTMapping{
							GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := multipleGVKsRequested(tc.objs)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCheckPrintWithNamespace(t *testing.T) {
	clusterScopedMapping := &meta.RESTMapping{
		Scope: meta.RESTScopeRoot,
	}
	namespacedMapping := &meta.RESTMapping{
		Scope: meta.RESTScopeNamespace,
	}

	tests := []struct {
		name          string
		mapping       *meta.RESTMapping
		allNamespaces bool
		expected      bool
	}{
		{
			name:          "cluster-scoped with allNamespaces true",
			mapping:       clusterScopedMapping,
			allNamespaces: true,
			expected:      false,
		},
		{
			name:          "namespaced with allNamespaces true",
			mapping:       namespacedMapping,
			allNamespaces: true,
			expected:      true,
		},
		{
			name:          "namespaced with allNamespaces false",
			mapping:       namespacedMapping,
			allNamespaces: false,
			expected:      false,
		},
		{
			name:          "nil mapping with allNamespaces true",
			mapping:       nil,
			allNamespaces: true,
			expected:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := &CommandGetOptions{AllNamespaces: tc.allNamespaces}
			actual := g.checkPrintWithNamespace(tc.mapping)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestSetNoAdoption(t *testing.T) {
	podMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Resource: "pods"},
	}
	deployMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Resource: "deployments"},
	}

	origPriority := podColumns[printColumnClusterNum].Priority
	t.Cleanup(func() {
		podColumns[printColumnClusterNum].Priority = origPriority
	})

	podColumns[printColumnClusterNum].Priority = 0

	setNoAdoption(nil)
	assert.Equal(t, int32(0), podColumns[printColumnClusterNum].Priority)

	setNoAdoption(deployMapping)
	assert.Equal(t, int32(0), podColumns[printColumnClusterNum].Priority)

	setNoAdoption(podMapping)
	assert.Equal(t, int32(1), podColumns[printColumnClusterNum].Priority)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		opts       *CommandGetOptions
		showLabels bool
		output     string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "output-watch-events without watch or watch-only",
			opts: &CommandGetOptions{
				OutputWatchEvents: true,
				Watch:             false,
				WatchOnly:         false,
			},
			wantErr: true,
			errMsg:  "--output-watch-events option can only be used with --watch or --watch-only",
		},
		{
			name:       "show-labels with json output",
			opts:       &CommandGetOptions{},
			showLabels: true,
			output:     "json",
			wantErr:    true,
			errMsg:     "--show-labels option cannot be used with json printer",
		},
		{
			name: "invalid operation scope",
			opts: &CommandGetOptions{
				OperationScope: options.OperationScope("invalid-scope"),
			},
			wantErr: true,
			errMsg:  "not support operation scope: invalid-scope",
		},
		{
			name: "valid options",
			opts: &CommandGetOptions{
				OperationScope: options.KarmadaControlPlane,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().Bool("show-labels", tc.showLabels, "")
			cmd.Flags().String("output", tc.output, "")
			err := tc.opts.Validate(cmd)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errMsg))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetWatchResourceVersion(t *testing.T) {
	// Case 1: Non-list object returns "0", nil
	pod := &corev1.Pod{}
	rv, err := getWatchResourceVersion(WatchObj{}, pod)
	assert.NoError(t, err)
	assert.Equal(t, "0", rv)

	// Case 2: List object with ResourceVersion
	podList := &corev1.PodList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "123",
		},
	}
	rv, err = getWatchResourceVersion(WatchObj{}, podList)
	assert.NoError(t, err)
	assert.Equal(t, "123", rv)

	// Case 3: List object with empty ResourceVersion falls back to infos
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()
	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body: cmdtesting.ObjBody(codec, &corev1.PodList{
					ListMeta: metav1.ListMeta{
						ResourceVersion: "456",
					},
				}),
			}, nil
		}),
	}
	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()
	watchObj := WatchObj{r: r}
	emptyRvList := &corev1.PodList{}
	rv, err = getWatchResourceVersion(watchObj, emptyRvList)
	assert.NoError(t, err)
	assert.Equal(t, "456", rv)
}

func TestBuildEmptyWatchTable(t *testing.T) {
	g := &CommandGetOptions{}
	podMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	// Case 1: watchObjs is empty
	table := g.buildEmptyWatchTable(nil, podMapping)
	assert.NotNil(t, table)
	assert.Equal(t, "Table", table.Kind)
	assert.NotEmpty(t, table.ColumnDefinitions)

	// Case 2: watchObjs returns an unstructured Table
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	tableObj := &metav1.Table{
		TypeMeta: metav1.TypeMeta{APIVersion: "meta.k8s.io/v1", Kind: "Table"},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "CustomCol", Type: "string"},
		},
	}
	tableRaw, _ := json.Marshal(tableObj)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body:       io.NopCloser(bytes.NewReader(tableRaw)),
			}, nil
		}),
	}
	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{{Cluster: "c1", r: r}}
	table2 := g.buildEmptyWatchTable(watchObjs, podMapping)
	assert.NotNil(t, table2)
	assert.Equal(t, "Table", table2.Kind)
}

func TestEmitEmptyWatchHeader_Variants(t *testing.T) {
	podMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	// Case 1: WatchOnly = true should NOT emit header
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	buf := &bytes.Buffer{}
	g := NewCommandGetOptions(streams)
	g.WatchOnly = true
	g.IsHumanReadablePrinter = true
	g.ToPrinter = g.getResourcePrinter()

	outputObjects := false
	p, err := g.emitEmptyWatchHeader(nil, podMapping, &outputObjects, buf)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Empty(t, buf.String())

	// Case 2: OutputFormat = "wide" should include wide columns
	bufWide := &bytes.Buffer{}
	wideFormat := "wide"
	streamsWide, _, _, _ := genericiooptions.NewTestIOStreams()
	gWide := NewCommandGetOptions(streamsWide)
	gWide.IsHumanReadablePrinter = true
	gWide.PrintFlags.OutputFormat = &wideFormat
	gWide.ToPrinter = gWide.getResourcePrinter()

	pWide, err := gWide.emitEmptyWatchHeader(nil, podMapping, &outputObjects, bufWide)
	assert.NoError(t, err)
	assert.NotNil(t, pWide)
	assert.Contains(t, bufWide.String(), "NAME")
	assert.Contains(t, bufWide.String(), "CLUSTER")
}

func TestWatchCluster_Errors(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	// Error getting object
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("object fetch failed")
		}),
	}
	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	g := &CommandGetOptions{}
	outObjs := false
	err := g.watchCluster(WatchObj{Cluster: "c1", r: r}, nil, &outObjs, nil, &bytes.Buffer{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get object")

	// Error starting watch
	codec := clientsetscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("watch") == "true" {
				return nil, fmt.Errorf("watch failed to establish")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body: cmdtesting.ObjBody(codec, &corev1.PodList{
					ListMeta: metav1.ListMeta{
						ResourceVersion: "100",
					},
				}),
			}, nil
		}),
	}
	rWatchErr := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods").
		SingleResourceType().
		Do()

	err = g.watchCluster(WatchObj{Cluster: "c1", r: rWatchErr}, nil, &outObjs, nil, &bytes.Buffer{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to watch")
}

func TestPrintInitialObjects_NonList(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("default")
	defer tf.Cleanup()

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}
	podRaw, _ := json.Marshal(pod)
	tableObj := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "meta.k8s.io/v1",
			Kind:       "Table",
		},
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string"},
		},
		Rows: []metav1.TableRow{
			{
				Cells: []any{"test-pod"},
				Object: runtime.RawExtension{
					Raw: podRaw,
				},
			},
		},
	}
	tableRaw, _ := json.Marshal(tableObj)

	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     cmdtesting.DefaultHeader(),
				Body:       io.NopCloser(bytes.NewReader(tableRaw)),
			}, nil
		}),
	}

	r := tf.NewBuilder().
		Unstructured().
		NamespaceParam("default").DefaultNamespace().
		ResourceTypeOrNameArgs(true, "pods/test-pod").
		SingleResourceType().
		Do()

	watchObjs := []WatchObj{{Cluster: "c1", r: r}}
	podMapping := &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}

	streams, _, buf, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	g.IsHumanReadablePrinter = true
	g.ToPrinter = g.getResourcePrinter()
	printer, err := g.ToPrinter(podMapping, nil, false, false)
	assert.NoError(t, err)

	printed, err := g.printInitialObjects(watchObjs, podMapping, printer, buf)
	assert.NoError(t, err)
	assert.True(t, printed)
	assert.Contains(t, buf.String(), "test-pod")
	assert.Contains(t, buf.String(), "c1")
}

func TestRun_WatchModeBranches(t *testing.T) {
	// Case 1: Watch mode with no watch objects and no errors returns error
	g := &CommandGetOptions{
		Watch:          true,
		OperationScope: options.OperationScope("none"),
	}
	err := g.Run(nil, []string{"pods"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not to find obj that is watched")
}
