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

package scheme

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

func TestSchemeInitialization(t *testing.T) {
	// Ensure that the Kubernetes core scheme (for example, Pod) is added.
	coreGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	if !isGVKRegistered(Scheme, coreGVK) {
		t.Errorf("K8s core scheme should be registered for GVK: %v", coreGVK)
	}

	// Ensure that the Karmada operator v1alpha1 scheme (for example, Karmada) is added.
	karmadaGVK := schema.GroupVersionKind{
		Group:   operatorv1alpha1.GroupVersion.Group,
		Version: operatorv1alpha1.GroupVersion.Version,
		Kind:    "Karmada",
	}
	if !isGVKRegistered(Scheme, karmadaGVK) {
		t.Errorf("Karmada v1alpha1 scheme should be registered for GVK: %v", karmadaGVK)
	}
}

// isGVKRegistered verifies if the scheme contains a specific GVK.
func isGVKRegistered(s *runtime.Scheme, gvk schema.GroupVersionKind) bool {
	_, err := s.New(gvk)
	if err != nil {
		fmt.Printf("Failed to find GVK: %v, Error: %v\n", gvk, err)
	}
	return err == nil
}
