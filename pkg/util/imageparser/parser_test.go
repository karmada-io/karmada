/*
Copyright 2021 The Karmada Authors.

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

package imageparser

import (
	"fmt"
	"testing"

	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name             string
		image            string
		expectHostname   string
		expectRepository string
		expectTag        string
		expectDigest     string
	}{
		{
			name:             "simple",
			image:            "pause",
			expectHostname:   "",
			expectRepository: "pause",
			expectTag:        "",
		},
		{
			name:             "repository",
			image:            "subpath/imagename:v1.0.0",
			expectHostname:   "",
			expectRepository: "subpath/imagename",
			expectTag:        "v1.0.0",
		},
		{
			name:             "normal",
			image:            "fictional.registry.example/imagename:v1.0.0",
			expectHostname:   "fictional.registry.example",
			expectRepository: "imagename",
			expectTag:        "v1.0.0",
		},
		{
			name:             "hostname with port",
			image:            "fictional.registry.example:10443/subpath/imagename:v1.0.0",
			expectHostname:   "fictional.registry.example:10443",
			expectRepository: "subpath/imagename",
			expectTag:        "v1.0.0",
		},
		{
			name:             "digest",
			image:            "fictional.registry.example:10443/subpath/imagename@sha256:50d858e0985ecc7f60418aaf0cc5ab587f42c2570a884095a9e8ccacd0f6545c",
			expectHostname:   "fictional.registry.example:10443",
			expectRepository: "subpath/imagename",
			expectDigest:     "sha256:50d858e0985ecc7f60418aaf0cc5ab587f42c2570a884095a9e8ccacd0f6545c",
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			comp, err := Parse(tc.image)
			if err != nil {
				t.Fatalf("unexpected error but got: %v", err)
			}
			if comp.String() != tc.image {
				t.Fatalf("full name changed from %s to %s", tc.image, comp.String())
			}
			if comp.Hostname() != tc.expectHostname {
				t.Fatalf("expected registry: %s, but got: %s", tc.expectHostname, comp.Hostname())
			}
			if comp.Repository() != tc.expectRepository {
				t.Fatalf("expected name: %s, but got: %s", tc.expectRepository, comp.Repository())
			}
			if comp.Tag() != tc.expectTag {
				t.Fatalf("expected tag: %s, but got: %s", tc.expectTag, comp.Tag())
			}
			if comp.Digest() != tc.expectDigest {
				t.Fatalf("expected digest: %s, but got: %s", tc.expectDigest, comp.Digest())
			}
		})
	}
}

func TestSplitHostname(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		expectedHostname   string
		expectedRepository string
	}{
		{
			name:               "empty image got nothing",
			input:              "",
			expectedHostname:   "",
			expectedRepository: "",
		},
		{
			name:               "simple repository",
			input:              "imagename",
			expectedHostname:   "",
			expectedRepository: "imagename",
		},
		{
			name:               "repository with sub-path",
			input:              "subpath/imagename",
			expectedHostname:   "",
			expectedRepository: "subpath/imagename",
		},
		{
			name:               "canonical image",
			input:              "fictional.registry.example/subpath/imagename",
			expectedHostname:   "fictional.registry.example",
			expectedRepository: "subpath/imagename",
		},
		{
			name:               "hostname with port",
			input:              "fictional.registry.example:10443/subpath/imagename",
			expectedHostname:   "fictional.registry.example:10443",
			expectedRepository: "subpath/imagename",
		},
		{
			name:               "hostname with IP",
			input:              "10.10.10.10:10443/subpath/imagename",
			expectedHostname:   "10.10.10.10:10443",
			expectedRepository: "subpath/imagename",
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			hostname, repository := SplitHostname(tc.input)
			if hostname != tc.expectedHostname {
				t.Fatalf("expected hostname: %s, but got: %s", tc.expectedHostname, hostname)
			}
			if repository != tc.expectedRepository {
				t.Fatalf("expected repository: %s, but got: %s", tc.expectedRepository, repository)
			}
		})
	}
}

func ExampleComponents_SetHostname() {
	image := "imagename:v1.0.0"
	comp, err := Parse(image)
	if err != nil {
		panic(err)
	}

	comp.SetHostname("fictional.registry.example") // add hostname
	fmt.Println(comp.String())
	comp.SetHostname("gcr.io") // update hostname
	fmt.Println(comp.String())
	comp.RemoveHostname() // remove hostname
	fmt.Println(comp.String())
	// Output:
	// fictional.registry.example/imagename:v1.0.0
	// gcr.io/imagename:v1.0.0
	// imagename:v1.0.0
}

func ExampleComponents_SetRepository() {
	image := "gcr.io/kube-apiserver:v1.19.0"
	comp, err := Parse(image)
	if err != nil {
		panic(err)
	}

	comp.SetRepository("kube-controller-manager") // update
	fmt.Println(comp.String())
	// Output:
	// gcr.io/kube-controller-manager:v1.19.0
}

func ExampleComponents_SetTagOrDigest() {
	image := "gcr.io/kube-apiserver"
	comp, err := Parse(image)
	if err != nil {
		panic(err)
	}

	comp.SetTagOrDigest("v1.19.0") // set
	fmt.Println(comp.String())
	comp.RemoveTagOrDigest() // remove tag
	fmt.Println(comp.String())
	comp.SetTagOrDigest("sha256:50d858e0985ecc7f60418aaf0cc5ab587f42c2570a884095a9e8ccacd0f6545c") // update
	fmt.Println(comp.String())
	comp.RemoveTagOrDigest() // remove digest
	fmt.Println(comp.String())
	// Output:
	// gcr.io/kube-apiserver:v1.19.0
	// gcr.io/kube-apiserver
	// gcr.io/kube-apiserver@sha256:50d858e0985ecc7f60418aaf0cc5ab587f42c2570a884095a9e8ccacd0f6545c
	// gcr.io/kube-apiserver
}

func ExampleComponents_String() {
	key := Components{
		hostname:   "fictional.registry.example",
		repository: names.KarmadaSchedulerComponentName,
		tag:        "v1.0.0",
	}
	pKey := &key
	fmt.Printf("%s\n", key)
	fmt.Printf("%v\n", key)
	fmt.Printf("%s\n", key.String())
	fmt.Printf("%s\n", pKey)
	fmt.Printf("%v\n", pKey)
	fmt.Printf("%s\n", pKey.String())
	// Output:
	// fictional.registry.example/karmada-scheduler:v1.0.0
	// fictional.registry.example/karmada-scheduler:v1.0.0
	// fictional.registry.example/karmada-scheduler:v1.0.0
	// fictional.registry.example/karmada-scheduler:v1.0.0
	// fictional.registry.example/karmada-scheduler:v1.0.0
	// fictional.registry.example/karmada-scheduler:v1.0.0
}
