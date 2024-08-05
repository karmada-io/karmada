/*
Copyright 2024 The Karmada Authors.

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

package gclient

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestNewSchema(t *testing.T) {
	schema := NewSchema()
	if schema == nil {
		t.Error("NewSchema() returned nil")
	}
}

func TestNewForConfig(t *testing.T) {
	config := &rest.Config{}
	_, err := NewForConfig(config)
	if err != nil {
		t.Errorf("NewForConfig() returned unexpected error: %v", err)
	}
}

func TestNewForConfigOrDieWithValidConfig(t *testing.T) {
	config := &rest.Config{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewForConfigOrDie() panicked unexpectedly: %v", r)
		}
	}()

	client := NewForConfigOrDie(config)
	if client == nil {
		t.Error("NewForConfigOrDie() returned nil")
	}
}

func TestAggregatedScheme(t *testing.T) {
	if aggregatedScheme == nil {
		t.Error("aggregatedScheme is nil")
	}

	gvks := []struct {
		obj  runtime.Object
		kind string
	}{
		{&corev1.Pod{}, "Pod"},
		{&appsv1.Deployment{}, "Deployment"},
		{&clusterv1alpha1.Cluster{}, "Cluster"},
		{&policyv1alpha1.PropagationPolicy{}, "PropagationPolicy"},
		// Add more types here to check if they're registered
	}

	for _, gvk := range gvks {
		kinds, _, err := aggregatedScheme.ObjectKinds(gvk.obj)
		if err != nil {
			t.Errorf("Error getting ObjectKinds for %s: %v", gvk.kind, err)
		}
		if len(kinds) == 0 {
			t.Errorf("No kinds found for %s", gvk.kind)
		}
		found := false
		for _, kind := range kinds {
			if kind.Kind == gvk.kind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected kind %s not found in returned kinds", gvk.kind)
		}
	}
}
