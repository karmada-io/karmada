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

package adapter_test

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/karmada-io/karmada/pkg/features"
	utildynamic "github.com/karmada-io/karmada/pkg/util/dynamic/adapter"
	utildynamicfake "github.com/karmada-io/karmada/pkg/util/dynamic/adapter/fake"
)

func TestDynamicInformerEnabled(t *testing.T) {
	defer setDynamicInformerFeatureGate(t, false)()
	if utildynamic.DynamicInformerEnabled() {
		t.Fatalf("DynamicInformerEnabled() = true, want false")
	}

	defer setDynamicInformerFeatureGate(t, true)()
	if !utildynamic.DynamicInformerEnabled() {
		t.Fatalf("DynamicInformerEnabled() = false, want true")
	}
}

func TestNewDynamicSharedInformerFactory(t *testing.T) {
	const (
		clientGoDynamicInformerPkg = "k8s.io/client-go/dynamic/dynamicinformer"
		karmadaDynamicInformerPkg  = "github.com/karmada-io/karmada/pkg/util/dynamic/dynamicinformer"
	)

	tests := []struct {
		name          string
		featureEnable bool
		client        func() utildynamic.Interface
		wantFactory   string
	}{
		{
			name:          "rawdynamic mode delegate uses Karmada dynamic informer",
			featureEnable: true,
			client: func() utildynamic.Interface {
				scheme := runtime.NewScheme()
				clientGoClient := dynamicfake.NewSimpleDynamicClient(scheme)
				dynamicClient := utildynamicfake.NewSimpleDynamicClient(scheme)
				return utildynamic.MustNewForClients(clientGoClient, dynamicClient.DynamicClient())
			},
			wantFactory: karmadaDynamicInformerPkg,
		},
		{
			name:          "unstructured mode delegate uses client-go dynamic informer",
			featureEnable: false,
			client: func() utildynamic.Interface {
				clientGoClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
				return utildynamic.MustNewForClients(clientGoClient, nil)
			},
			wantFactory: clientGoDynamicInformerPkg,
		},
		{
			name:          "plain client-go client uses client-go dynamic informer",
			featureEnable: true,
			client: func() utildynamic.Interface {
				return dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
			},
			wantFactory: clientGoDynamicInformerPkg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(setDynamicInformerFeatureGate(t, tt.featureEnable))

			factory := utildynamic.NewDynamicSharedInformerFactory(tt.client(), 0)
			factoryType := reflect.TypeOf(factory)
			if factoryType.Kind() == reflect.Pointer {
				factoryType = factoryType.Elem()
			}
			if got := factoryType.PkgPath(); got != tt.wantFactory {
				t.Fatalf("factory package = %q, want %q", got, tt.wantFactory)
			}
		})
	}
}

func setDynamicInformerFeatureGate(t *testing.T, enabled bool) func() {
	t.Helper()

	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.RawDynamicInformer, enabled)); err != nil {
		t.Fatalf("failed to set feature gate %s to %v: %v", features.RawDynamicInformer, enabled, err)
	}
	return func() {
		features.FeatureGate = originalFeatureGates
	}
}
