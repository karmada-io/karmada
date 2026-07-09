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

package dynamic_test

import (
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	"github.com/karmada-io/karmada/pkg/util/dynamic"
	"github.com/karmada-io/karmada/pkg/util/dynamic/dynamicinformer"
	"github.com/karmada-io/karmada/pkg/util/dynamic/dynamiclister"
)

var testGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
var testThingGVR = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "things"}

const rawDeployment = `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"namespace":"default","name":"demo","labels":{"app":"demo"},"resourceVersion":"7"},"spec":{"replicas":3}}`
const rawObjectWithTopLevelItems = `{"apiVersion":"example.io/v1","kind":"Thing","metadata":{"namespace":"default","name":"demo"},"items":[{"name":"embedded"}]}`

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()

	var gotObj any
	if err := stdjson.Unmarshal(got, &gotObj); err != nil {
		t.Fatalf("failed to unmarshal got JSON %q: %v", string(got), err)
	}
	var wantObj any
	if err := stdjson.Unmarshal(want, &wantObj); err != nil {
		t.Fatalf("failed to unmarshal want JSON %q: %v", string(want), err)
	}
	if !reflect.DeepEqual(gotObj, wantObj) {
		t.Fatalf("unexpected JSON (-want +got): want=%s got=%s", string(want), string(got))
	}
}

func assertRawObjectJSONEqual(t *testing.T, obj *dynamic.RawObject, want string) {
	t.Helper()

	got, err := stdjson.Marshal(obj)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	assertJSONEqual(t, got, []byte(want))
}

//nolint:gocyclo
func TestRawObjectMetadataAndListSemantics(t *testing.T) {
	obj, err := dynamic.NewRawObject([]byte(rawDeployment))
	if err != nil {
		t.Fatalf("NewRawObject() error = %v", err)
	}
	if obj.GetNamespace() != "default" || obj.GetName() != "demo" || obj.GetLabels()["app"] != "demo" {
		t.Fatalf("unexpected object metadata: namespace=%q name=%q labels=%v", obj.GetNamespace(), obj.GetName(), obj.GetLabels())
	}
	assertRawObjectJSONEqual(t, obj, rawDeployment)
	if meta.IsListType(obj) {
		t.Fatalf("RawObject must not be treated as a list type")
	}

	list, err := dynamic.NewRawObjectList(fmt.Appendf(nil, `{"apiVersion":"v1","kind":"List","metadata":{"resourceVersion":"10"},"items":[%s]}`, rawDeployment))
	if err != nil {
		t.Fatalf("NewRawObjectList() error = %v", err)
	}
	if !meta.IsListType(list) {
		t.Fatalf("RawObjectList should be treated as a list type")
	}
	listMeta, err := meta.ListAccessor(list)
	if err != nil {
		t.Fatalf("ListAccessor() error = %v", err)
	}
	if listMeta.GetResourceVersion() != "10" {
		t.Fatalf("unexpected list resourceVersion: %q", listMeta.GetResourceVersion())
	}

	items, err := meta.ExtractListWithAlloc(list)
	if err != nil {
		t.Fatalf("ExtractListWithAlloc() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	rawItem, ok := items[0].(*dynamic.RawObject)
	if !ok {
		t.Fatalf("expected *RawObject item, got %T", items[0])
	}
	assertRawObjectJSONEqual(t, rawItem, rawDeployment)
	if rawItem.APIVersion != "apps/v1" || rawItem.Kind != "Deployment" {
		t.Fatalf("unexpected inferred item gvk: %s", rawItem.GroupVersionKind())
	}
}

func TestRawObjectToUnstructuredPreservesIntegerNumbers(t *testing.T) {
	obj, err := dynamic.NewRawObject([]byte(rawDeployment))
	if err != nil {
		t.Fatalf("NewRawObject() error = %v", err)
	}

	u, err := obj.ToUnstructured()
	if err != nil {
		t.Fatalf("ToUnstructured() error = %v", err)
	}
	replicas, found, err := unstructured.NestedInt64(u.Object, "spec", "replicas")
	if err != nil {
		t.Fatalf("NestedInt64() error = %v", err)
	}
	if !found {
		t.Fatalf("spec.replicas was not found")
	}
	if replicas != 3 {
		t.Fatalf("unexpected spec.replicas: got %d, want 3", replicas)
	}
}

func TestRawObjectMarshalJSONOverlaysTypeAndObjectMeta(t *testing.T) {
	obj, err := dynamic.NewRawObject([]byte(`{"metadata":{"namespace":"default","name":"demo"},"spec":{"replicas":3}}`))
	if err != nil {
		t.Fatalf("NewRawObject() error = %v", err)
	}
	obj.APIVersion = "apps/v1"
	obj.Kind = "Deployment"
	obj.SetAnnotations(map[string]string{"cluster": "member1"})

	raw, err := stdjson.Marshal(obj)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var got struct {
		APIVersion string            `json:"apiVersion"`
		Kind       string            `json:"kind"`
		Metadata   metav1.ObjectMeta `json:"metadata"`
		Spec       struct {
			Replicas int64 `json:"replicas"`
		} `json:"spec"`
	}
	if err := stdjson.Unmarshal(raw, &got); err != nil {
		t.Fatalf("failed to unmarshal marshaled raw object: %v", err)
	}
	if got.APIVersion != "apps/v1" || got.Kind != "Deployment" {
		t.Fatalf("unexpected marshaled gvk: apiVersion=%q kind=%q", got.APIVersion, got.Kind)
	}
	if got.Metadata.Namespace != "default" || got.Metadata.Name != "demo" || got.Metadata.Annotations["cluster"] != "member1" {
		t.Fatalf("unexpected marshaled metadata: %#v", got.Metadata)
	}
	if got.Spec.Replicas != 3 {
		t.Fatalf("unexpected marshaled spec.replicas: %d", got.Spec.Replicas)
	}
}

func TestRawObjectMarshalJSONPreservesRawMetadataWhenSidecarIsEmpty(t *testing.T) {
	obj, err := dynamic.NewRawObject([]byte(rawDeployment))
	if err != nil {
		t.Fatalf("NewRawObject() error = %v", err)
	}
	raw, err := stdjson.Marshal(obj)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var got struct {
		APIVersion string            `json:"apiVersion"`
		Kind       string            `json:"kind"`
		Metadata   metav1.ObjectMeta `json:"metadata"`
	}
	if err := stdjson.Unmarshal(raw, &got); err != nil {
		t.Fatalf("failed to unmarshal marshaled raw object: %v", err)
	}
	if got.APIVersion != "apps/v1" || got.Kind != "Deployment" {
		t.Fatalf("unexpected marshaled gvk: apiVersion=%q kind=%q", got.APIVersion, got.Kind)
	}
	if got.Metadata.Namespace != "default" || got.Metadata.Name != "demo" || got.Metadata.Labels["app"] != "demo" {
		t.Fatalf("raw metadata was not preserved: %#v", got.Metadata)
	}
}

//nolint:gocyclo
func TestDynamicClientListGetAndWatch(t *testing.T) {
	server := newDynamicTestServer(t)
	defer server.Close()

	client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("NewForConfig() error = %v", err)
	}

	resource := client.Resource(testGVR).Namespace("default")
	list, err := resource.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("unexpected list items: %#v", list.Items)
	}
	assertRawObjectJSONEqual(t, &list.Items[0], rawDeployment)

	obj, err := resource.Get(context.Background(), "demo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if obj.GetName() != "demo" {
		t.Fatalf("unexpected get object name: %q", obj.GetName())
	}
	assertRawObjectJSONEqual(t, obj, rawDeployment)

	watcher, err := resource.Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	defer watcher.Stop()

	select {
	case event := <-watcher.ResultChan():
		if event.Type != watch.Added {
			t.Fatalf("unexpected watch event type: %s", event.Type)
		}
		watched, ok := event.Object.(*dynamic.RawObject)
		if !ok {
			t.Fatalf("expected watch object *RawObject, got %T", event.Object)
		}
		assertRawObjectJSONEqual(t, watched, rawDeployment)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for watch event")
	}

	thingWatcher, err := client.Resource(testThingGVR).Namespace("default").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Watch() object with top-level items error = %v", err)
	}
	defer thingWatcher.Stop()

	select {
	case event := <-thingWatcher.ResultChan():
		watched, ok := event.Object.(*dynamic.RawObject)
		if !ok {
			t.Fatalf("object with top-level items should decode as *RawObject, got %T", event.Object)
		}
		assertRawObjectJSONEqual(t, watched, rawObjectWithTopLevelItems)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for top-level items watch event")
	}
}

func TestDynamicInformerStoresRawObjects(t *testing.T) {
	server := newDynamicTestServer(t)
	defer server.Close()

	client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("NewForConfig() error = %v", err)
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(client, 0)
	informer := factory.ForResource(testGVR)

	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)

	syncedCh := make(chan map[schema.GroupVersionResource]bool, 1)
	go func() {
		syncedCh <- factory.WaitForCacheSync(stopCh)
	}()

	select {
	case synced := <-syncedCh:
		if !synced[testGVR] {
			t.Fatalf("informer cache was not synced: %v", synced)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for informer sync")
	}

	runtimeObj, err := informer.Lister().ByNamespace("default").Get("demo")
	if err != nil {
		t.Fatalf("lister Get() error = %v", err)
	}
	rawObj, ok := runtimeObj.(*dynamic.RawObject)
	if !ok {
		t.Fatalf("expected cache object *RawObject, got %T", runtimeObj)
	}
	assertRawObjectJSONEqual(t, rawObj, rawDeployment)

	typedItems, err := dynamiclister.New(informer.Informer().GetIndexer(), testGVR).Namespace("default").List(labels.Everything())
	if err != nil {
		t.Fatalf("typed raw lister List() error = %v", err)
	}
	if len(typedItems) != 1 || typedItems[0] != rawObj {
		t.Fatalf("unexpected typed lister items: %#v", typedItems)
	}
}

func TestDynamicClientWritesRawObjectBytes(t *testing.T) {
	obj, err := dynamic.NewRawObject([]byte(rawDeployment))
	if err != nil {
		t.Fatalf("NewRawObject() error = %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		invokeFunc func(client dynamic.ResourceInterface) (*dynamic.RawObject, error)
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/apis/apps/v1/namespaces/default/deployments",
			invokeFunc: func(client dynamic.ResourceInterface) (*dynamic.RawObject, error) {
				return client.Create(context.Background(), obj, metav1.CreateOptions{})
			},
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/apis/apps/v1/namespaces/default/deployments/demo",
			invokeFunc: func(client dynamic.ResourceInterface) (*dynamic.RawObject, error) {
				return client.Update(context.Background(), obj, metav1.UpdateOptions{})
			},
		},
		{
			name:   "update status",
			method: http.MethodPut,
			path:   "/apis/apps/v1/namespaces/default/deployments/demo/status",
			invokeFunc: func(client dynamic.ResourceInterface) (*dynamic.RawObject, error) {
				return client.UpdateStatus(context.Background(), obj, metav1.UpdateOptions{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte
			var gotContentType string
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if req.Method != tt.method {
					http.Error(rw, "unexpected method", http.StatusMethodNotAllowed)
					return
				}
				if req.URL.Path != tt.path {
					http.NotFound(rw, req)
					return
				}
				gotContentType = req.Header.Get("Content-Type")
				var err error
				gotBody, err = io.ReadAll(req.Body)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}
				rw.Header().Set("Content-Type", runtime.ContentTypeJSON)
				_, _ = fmt.Fprint(rw, rawDeployment)
			}))
			defer server.Close()

			client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL})
			if err != nil {
				t.Fatalf("NewForConfig() error = %v", err)
			}
			out, err := tt.invokeFunc(client.Resource(testGVR).Namespace("default"))
			if err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			assertRawObjectJSONEqual(t, out, rawDeployment)
			if gotContentType != runtime.ContentTypeJSON {
				t.Fatalf("unexpected content type: %q", gotContentType)
			}
			assertJSONEqual(t, gotBody, []byte(rawDeployment))
		})
	}
}

func TestDynamicClientApplyUsesJSONApplyPatch(t *testing.T) {
	var appliedBody []byte
	var contentType string

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch {
			http.Error(rw, "expected PATCH", http.StatusMethodNotAllowed)
			return
		}
		if req.URL.Path != "/apis/apps/v1/namespaces/default/deployments/demo" {
			http.NotFound(rw, req)
			return
		}
		if req.URL.Query().Get("fieldManager") != "rawdynamic" {
			http.Error(rw, "missing fieldManager", http.StatusBadRequest)
			return
		}
		contentType = req.Header.Get("Content-Type")
		if strings.Contains(req.Header.Get("Accept"), "cbor") || strings.Contains(contentType, "cbor") {
			http.Error(rw, "CBOR should not be requested by rawdynamic apply", http.StatusNotAcceptable)
			return
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		appliedBody = body
		rw.Header().Set("Content-Type", runtime.ContentTypeJSON)
		_, _ = fmt.Fprint(rw, rawDeployment)
	}))
	defer server.Close()

	client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("NewForConfig() error = %v", err)
	}
	obj, err := dynamic.NewRawObject([]byte(rawDeployment))
	if err != nil {
		t.Fatalf("NewRawObject() error = %v", err)
	}

	out, err := client.Resource(testGVR).Namespace("default").Apply(context.Background(), "demo", obj, metav1.ApplyOptions{FieldManager: "rawdynamic"})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if out.GetName() != "demo" {
		t.Fatalf("unexpected apply response name: %q", out.GetName())
	}
	if contentType != string(types.ApplyPatchType) {
		t.Fatalf("unexpected apply content type: %q", contentType)
	}
	assertJSONEqual(t, appliedBody, []byte(rawDeployment))

	withManagedFields := obj.DeepCopy()
	withManagedFields.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "existing"}}
	if _, err := client.Resource(testGVR).Namespace("default").Apply(context.Background(), "demo", withManagedFields, metav1.ApplyOptions{FieldManager: "rawdynamic"}); err == nil || !strings.Contains(err.Error(), "managed fields") {
		t.Fatalf("expected managed fields apply error, got %v", err)
	}
}

func newDynamicTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.Header.Get("Accept"), "cbor") || strings.Contains(req.Header.Get("Content-Type"), "cbor") {
			http.Error(rw, "CBOR should not be requested by rawdynamic", http.StatusNotAcceptable)
			return
		}

		switch req.URL.Path {
		case "/apis/apps/v1/deployments":
			rw.Header().Set("Content-Type", runtime.ContentTypeJSON)
			if req.URL.Query().Get("watch") == "true" {
				_, _ = fmt.Fprintf(rw, `{"type":"ADDED","object":%s}`+"\n", rawDeployment)
				if req.URL.Query().Get("sendInitialEvents") == "true" {
					_, _ = fmt.Fprint(rw, `{"type":"BOOKMARK","object":{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"resourceVersion":"10","annotations":{"k8s.io/initial-events-end":"true"}}}}`+"\n")
				}
				return
			}
			_, _ = fmt.Fprintf(rw, `{"apiVersion":"apps/v1","kind":"DeploymentList","metadata":{"resourceVersion":"10"},"items":[%s]}`, rawDeployment)
		case "/apis/apps/v1/namespaces/default/deployments":
			rw.Header().Set("Content-Type", runtime.ContentTypeJSON)
			if req.URL.Query().Get("watch") == "true" {
				_, _ = fmt.Fprintf(rw, `{"type":"ADDED","object":%s}`+"\n", rawDeployment)
				if req.URL.Query().Get("sendInitialEvents") == "true" {
					_, _ = fmt.Fprint(rw, `{"type":"BOOKMARK","object":{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"resourceVersion":"10","annotations":{"k8s.io/initial-events-end":"true"}}}}`+"\n")
				}
				return
			}
			_, _ = fmt.Fprintf(rw, `{"apiVersion":"apps/v1","kind":"DeploymentList","metadata":{"resourceVersion":"10"},"items":[%s]}`, rawDeployment)
		case "/apis/apps/v1/namespaces/default/deployments/demo":
			rw.Header().Set("Content-Type", runtime.ContentTypeJSON)
			_, _ = fmt.Fprint(rw, rawDeployment)
		case "/apis/example.io/v1/namespaces/default/things":
			rw.Header().Set("Content-Type", runtime.ContentTypeJSON)
			if req.URL.Query().Get("watch") == "true" {
				_, _ = fmt.Fprintf(rw, `{"type":"ADDED","object":%s}`+"\n", rawObjectWithTopLevelItems)
				return
			}
			http.NotFound(rw, req)
		default:
			http.NotFound(rw, req)
		}
	}))
}
