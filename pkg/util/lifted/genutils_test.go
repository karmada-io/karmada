/*
Copyright 2015 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/cmd/genutils/genutils_test.go

package lifted

import (
	"testing"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/cmd/genutils/genutils_test.go#L23-L28

func TestValidDir(t *testing.T) {
	_, err := OutDir("./")
	if err != nil {
		t.Fatal(err)
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/cmd/genutils/genutils_test.go#L30-L35

func TestInvalidDir(t *testing.T) {
	_, err := OutDir("./nondir")
	if err == nil {
		t.Fatal("expected an error")
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/cmd/genutils/genutils_test.go#L37-L42

func TestNotDir(t *testing.T) {
	_, err := OutDir("./genutils_test.go")
	if err == nil {
		t.Fatal("expected an error")
	}
}
