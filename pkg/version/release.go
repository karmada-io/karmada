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

import (
	"fmt"
	"regexp"

	utilversion "k8s.io/apimachinery/pkg/util/version"
)

var (
	// gitVersionSplitRE is a regexp to split a git version string.
	gitVersionSplitRE = regexp.MustCompile("-[0-9]+-g[0-9a-z]{7}")
)

// ReleaseVersion represents a released version.
type ReleaseVersion struct {
	*utilversion.Version
}

// ParseGitVersion parses a git version string, such as:
// - v1.1.0-73-g7e6d4f69
// - v1.1.0
// - v1.1.0-alpha.1-3-gf20c721a
func ParseGitVersion(gitVersion string) (*ReleaseVersion, error) {
	v, err := utilversion.ParseSemantic(gitVersion)
	if err != nil {
		return nil, err
	}

	return &ReleaseVersion{
		Version: v,
	}, nil
}

// ReleaseVersion returns the parsed version in the following format:
// - v1.2.1-12-g2eb92858 --> v1.2.1
// - v1.2.3-12-g2e860210 --> v1.2.3
// - v1.3.0-alpha.1-12-g2e860210 --> v1.3.0-alpha.1
// It could be patch release or pre-release
func (r *ReleaseVersion) ReleaseVersion() string {
	if r.Version == nil {
		return "<nil>"
	}

	filteredVersion := removeGitVersionCommits(r.String())
	return fmt.Sprintf("v%s", filteredVersion)
}

// removeGitVersionCommits removes the git commit info from the version
// The git version looks like: v1.0.4-14-g2414721
// the current head of my "parent" branch is based on v1.0.4,
// but since it has a few commits on top of that, describe has added the number of additional commits ("14")
// and an abbreviated object name for the commit itself ("2414721") at the end.
func removeGitVersionCommits(gitVersion string) string {
	// This match the commit info part of the git version
	// If the gitVersion is empty, it will return an empty string
	matches := gitVersionSplitRE.Split(gitVersion, 2)

	return matches[0]
}
