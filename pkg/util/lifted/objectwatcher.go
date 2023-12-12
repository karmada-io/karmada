/*
Copyright 2016 The Kubernetes Authors.

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

// This code is lifted from the kubefed codebase. It's a list of functions to determine whether the provided cluster
// object needs to be updated according to the desired object and the recorded version.
// For reference:
// https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/util/propagatedversion.go#L30-L59
// https://github.com/kubernetes-retired/kubefed/blob/master/pkg/controller/util/meta.go#L82-L103

package lifted

import (
	"fmt"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	generationPrefix      = "gen:"
	resourceVersionPrefix = "rv:"
)

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/util/propagatedversion.go#L35-L43

// ObjectVersion retrieves the field type-prefixed value used for
// determining currency of the given cluster object.
func ObjectVersion(obj *unstructured.Unstructured) string {
	generation := obj.GetGeneration()
	if generation != 0 {
		return fmt.Sprintf("%s%d", generationPrefix, generation)
	}
	return fmt.Sprintf("%s%s", resourceVersionPrefix, obj.GetResourceVersion())
}

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/util/propagatedversion.go#L45-L59

// ObjectNeedsUpdate determines whether the 2 objects provided cluster
// object needs to be updated according to the old object and the
// recorded version.
func ObjectNeedsUpdate(oldObj, currentObj *unstructured.Unstructured, recordedVersion string) bool {
	targetVersion := ObjectVersion(currentObj)

	if recordedVersion != targetVersion {
		return true
	}

	// If versions match and the version is sourced from the
	// generation field, a further check of metadata equivalency is
	// required.
	return strings.HasPrefix(targetVersion, generationPrefix) && !objectMetaObjEquivalent(oldObj, currentObj)
}

// +lifted:source=https://github.com/kubernetes-retired/kubefed/blob/master/pkg/controller/util/meta.go#L82-L103
// +lifted:changed

// objectMetaObjEquivalent checks if cluster-independent, user provided data in two given ObjectMeta are equal. If in
// the future the ObjectMeta structure is expanded then any field that is not populated
// by the api server should be included here.
func objectMetaObjEquivalent(a, b metav1.Object) bool {
	if a.GetName() != b.GetName() {
		return false
	}
	if a.GetNamespace() != b.GetNamespace() {
		return false
	}
	aLabels := a.GetLabels()
	bLabels := b.GetLabels()
	if !reflect.DeepEqual(aLabels, bLabels) && (len(aLabels) != 0 || len(bLabels) != 0) {
		return false
	}
	return true
}
