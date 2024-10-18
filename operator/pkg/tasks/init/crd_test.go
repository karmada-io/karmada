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

package tasks

import (
	"archive/tar"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckOperatorCrdsTar(t *testing.T) {
	testItems := []struct {
		name        string
		header      *tar.Header
		expectedErr error
	}{
		{
			name: "unclean file dir 'crds/../'",
			header: &tar.Header{
				Name:     "crds/../",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unclean file dir: %s", "crds/../"),
		},
		{
			name: "unexpected file dir '../crds'",
			header: &tar.Header{
				Name:     "../crds",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file dir: %s", "../crds"),
		},
		{
			name: "unexpected file dir '..'",
			header: &tar.Header{
				Name:     "..",
				Typeflag: tar.TypeDir,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file dir: %s", ".."),
		},
		{
			name: "expected file dir 'crds/'",
			header: &tar.Header{
				Name:     "crds/",
				Typeflag: tar.TypeDir,
			},
			expectedErr: nil,
		},
		{
			name: "expected file dir 'crds'",
			header: &tar.Header{
				Name:     "crds",
				Typeflag: tar.TypeDir,
			},
			expectedErr: nil,
		},
		{
			name: "unclean file path 'crds/../a.yaml'",
			header: &tar.Header{
				Name:     "crds/../a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: fmt.Errorf("the given file contains unclean file path: %s", "crds/../a.yaml"),
		},
		{
			name: "unexpected file path '../crds/a.yaml'",
			header: &tar.Header{
				Name:     "../crds/a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file path: %s", "../crds/a.yaml"),
		},
		{
			name: "unexpected file path '../a.yaml'",
			header: &tar.Header{
				Name:     "../a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: fmt.Errorf("the given file contains unexpected file path: %s", "../a.yaml"),
		},
		{
			name: "expected file path 'crds/a.yaml'",
			header: &tar.Header{
				Name:     "crds/a.yaml",
				Typeflag: tar.TypeReg,
			},
			expectedErr: nil,
		},

	}
	for _, item := range testItems {
		assert.Equal(t, item.expectedErr, checkOperatorCrdsTar(item.header))
	}
}
