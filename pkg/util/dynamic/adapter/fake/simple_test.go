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

package fake

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"

	"github.com/karmada-io/karmada/pkg/features"
	utildynamicadapter "github.com/karmada-io/karmada/pkg/util/dynamic/adapter"
)

func TestNewSimpleDynamicClientExposesClientGoFakeSurface(t *testing.T) {
	client := NewSimpleDynamicClient(runtime.NewScheme())

	if client.Tracker() == nil {
		t.Fatal("Tracker() = nil")
	}

	gvr := schema.GroupVersionResource{Group: "test.karmada.io", Version: "v1", Resource: "widgets"}
	client.PrependReactor("get", "widgets", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "test.karmada.io/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name": "demo",
			},
		}}, nil
	})

	obj, err := client.Resource(gvr).Get(context.Background(), "demo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if obj.GetName() != "demo" {
		t.Fatalf("Get() name = %q, want demo", obj.GetName())
	}
	if got := len(client.Actions()); got != 1 {
		t.Fatalf("Actions() length = %d, want 1", got)
	}

	client.Fake.ClearActions()
	if got := len(client.Actions()); got != 0 {
		t.Fatalf("Actions() length after ClearActions = %d, want 0", got)
	}
}

func TestNewSimpleDynamicClientSupportsAdapterInformerSelection(t *testing.T) {
	const karmadaDynamicInformerPkg = "github.com/karmada-io/karmada/pkg/util/dynamic/dynamicinformer"

	setDynamicInformerFeatureGate(t, true)

	client := NewSimpleDynamicClient(runtime.NewScheme())
	if got := client.InformerMode(); got != utildynamicadapter.InformerModeRawDynamic {
		t.Fatalf("InformerMode() = %q, want %q", got, utildynamicadapter.InformerModeRawDynamic)
	}
	if client.DynamicClient() == nil {
		t.Fatal("DynamicClient() = nil")
	}

	factory := utildynamicadapter.NewDynamicSharedInformerFactory(client, 0)
	factoryType := reflect.TypeOf(factory)
	if factoryType.Kind() == reflect.Pointer {
		factoryType = factoryType.Elem()
	}
	if got := factoryType.PkgPath(); got != karmadaDynamicInformerPkg {
		t.Fatalf("factory package = %q, want %q", got, karmadaDynamicInformerPkg)
	}
}

func setDynamicInformerFeatureGate(t *testing.T, enabled bool) {
	t.Helper()

	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.RawDynamicInformer, enabled)); err != nil {
		t.Fatalf("failed to set feature gate %s to %v: %v", features.RawDynamicInformer, enabled, err)
	}
	t.Cleanup(func() {
		features.FeatureGate = originalFeatureGates
	})
}
