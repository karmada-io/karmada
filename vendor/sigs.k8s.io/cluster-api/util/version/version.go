/*
Copyright 2021 The Kubernetes Authors.

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

// Package version implements version handling.
package version

import (
	"regexp"
	"strconv"

	"github.com/blang/semver"
	"github.com/pkg/errors"
)

var (
	// KubeSemver is the regex for Kubernetes versions. It requires the "v" prefix.
	KubeSemver = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)
	// KubeSemverTolerant is the regex for Kubernetes versions with an optional "v" prefix.
	KubeSemverTolerant = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)
)

// ParseMajorMinorPatch returns a semver.Version from the string provided
// by looking only at major.minor.patch and stripping everything else out.
// It requires the version to have a "v" prefix.
func ParseMajorMinorPatch(version string) (semver.Version, error) {
	return parseMajorMinorPatch(version, false)
}

// ParseMajorMinorPatchTolerant returns a semver.Version from the string provided
// by looking only at major.minor.patch and stripping everything else out.
// It does not require the version to have a "v" prefix.
func ParseMajorMinorPatchTolerant(version string) (semver.Version, error) {
	return parseMajorMinorPatch(version, true)
}

// parseMajorMinorPatch returns a semver.Version from the string provided
// by looking only at major.minor.patch and stripping everything else out.
func parseMajorMinorPatch(version string, tolerant bool) (semver.Version, error) {
	groups := KubeSemver.FindStringSubmatch(version)
	if tolerant {
		groups = KubeSemverTolerant.FindStringSubmatch(version)
	}
	if len(groups) < 4 {
		return semver.Version{}, errors.Errorf("failed to parse major.minor.patch from %q", version)
	}
	major, err := strconv.ParseUint(groups[1], 10, 64)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to parse major version from %q", version)
	}
	minor, err := strconv.ParseUint(groups[2], 10, 64)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to parse minor version from %q", version)
	}
	patch, err := strconv.ParseUint(groups[3], 10, 64)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to parse patch version from %q", version)
	}
	return semver.Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}
