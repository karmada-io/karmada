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
	"strings"

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

const (
	numbers = "01234567890"
)

func containsOnly(s string, set string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return !strings.ContainsRune(set, r)
	}) == -1
}

type buildIdentifiers []buildIdentifier

func newBuildIdentifiers(ids []string) buildIdentifiers {
	bis := make(buildIdentifiers, 0, len(ids))
	for _, id := range ids {
		bis = append(bis, newBuildIdentifier(id))
	}
	return bis
}

// compare compares 2 builidentifiers v and 0.
// -1 == v is less than o.
// 0 == v is equal to o.
// 1 == v is greater than o.
// Note: If everything else is equal the longer build identifier is greater.
func (v buildIdentifiers) compare(o buildIdentifiers) int {
	i := 0
	for ; i < len(v) && i < len(o); i++ {
		if comp := v[i].compare(o[i]); comp == 0 {
			continue
		} else {
			return comp
		}
	}

	// if everything is equal till now the longer is greater
	if i == len(v) && i == len(o) { //nolint: gocritic
		return 0
	} else if i == len(v) && i < len(o) {
		return -1
	} else {
		return 1
	}
}

type buildIdentifier struct {
	IdentifierInt uint64
	IdentifierStr string
	IsNum         bool
}

func newBuildIdentifier(s string) buildIdentifier {
	bi := buildIdentifier{}
	if containsOnly(s, numbers) {
		num, _ := strconv.ParseUint(s, 10, 64)
		bi.IdentifierInt = num
		bi.IsNum = true
	} else {
		bi.IdentifierStr = s
		bi.IsNum = false
	}
	return bi
}

// compare compares v and o.
// -1 == v is less than o.
// 0 == v is equal to o.
// 1 == v is greater than o.
// Note: number is considered lower than string.
func (v buildIdentifier) compare(o buildIdentifier) int {
	if v.IsNum && !o.IsNum {
		return -1
	}
	if !v.IsNum && o.IsNum {
		return 1
	}
	if v.IsNum && o.IsNum { // both are numbers
		switch {
		case v.IdentifierInt < o.IdentifierInt:
			return -1
		case v.IdentifierInt == o.IdentifierInt:
			return 0
		default:
			return 1
		}
	} else { // both are strings
		switch {
		case v.IdentifierStr < o.IdentifierStr:
			return -1
		case v.IdentifierStr == o.IdentifierStr:
			return 0
		default:
			return 1
		}
	}
}

// CompareWithBuildIdentifiers compares two versions a and b.
// Perfoms a standard version compare between a and b. If the versions
// are equal, build identifiers will be used to compare further.
//   -1 == a is less than b.
//   0 == a is equal to b.
//   1 == a is greater than b.
// Deprecated: Use Compare(a, b, WithBuildTags()) instead.
func CompareWithBuildIdentifiers(a semver.Version, b semver.Version) int {
	if comp := a.Compare(b); comp != 0 {
		return comp
	}
	biA := newBuildIdentifiers(a.Build)
	biB := newBuildIdentifiers(b.Build)
	return biA.compare(biB)
}

type comparer struct {
	buildTags bool
}

// CompareOption is a configuration option for Compare.
type CompareOption func(*comparer)

// WithBuildTags modifies the version comparison to also consider build tags
// when comparing versions.
// Performs a standard version compare between a and b. If the versions
// are equal, build identifiers will be used to compare further.
//   -1 == a is less than b.
//   0 == a is equal to b.
//   1 == a is greater than b.
func WithBuildTags() CompareOption {
	return func(c *comparer) {
		c.buildTags = true
	}
}

// Compare 2 semver versions.
// Defaults to doing the standard semver comparison when no options are specified.
// The comparison logic can be modified by passing additional compare options.
// Example: using the WithBuildTags() option modifies the compare logic to also
// consider build tags when comparing versions.
func Compare(a, b semver.Version, options ...CompareOption) int {
	c := &comparer{}
	for _, o := range options {
		o(c)
	}

	if c.buildTags {
		return CompareWithBuildIdentifiers(a, b)
	}
	return a.Compare(b)
}
