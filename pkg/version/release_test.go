/*
Copyright 2022 The Karmada Authors.

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

package version

import "testing"

func TestReleaseVersion(t *testing.T) {
	tests := []struct {
		name                 string
		gitVersion           string
		expectReleaseVersion string
		expectError          bool
	}{
		{
			name:                 "normal git version",
			gitVersion:           "v1.1.1-6-gf20c721a",
			expectReleaseVersion: "v1.1.1",
			expectError:          false,
		},
		{
			name:        "abnormal version",
			gitVersion:  "vx.y.z-6-gf20c721a",
			expectError: true,
		},
		{
			name:                 "prerelease alpha version",
			gitVersion:           "v1.7.0-alpha.1-3-gf20c721a",
			expectReleaseVersion: "v1.7.0-alpha.1",
			expectError:          false,
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			rv, err := ParseGitVersion(tc.gitVersion)
			if err != nil {
				if !tc.expectError {
					t.Fatalf("No error is expected but got: %v", err)
				}
				// Stop and passes this test as error is expected.
				return
			} else if err == nil {
				if tc.expectError {
					t.Fatalf("Expect error, but got nil")
				}
			}

			if rv.ReleaseVersion() != tc.expectReleaseVersion {
				t.Fatalf("expect patch release: %s, but got: %s", tc.expectReleaseVersion, rv.ReleaseVersion())
			}
		})
	}
}
