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

package imageparser

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagRegexp(t *testing.T) {
	validTags := []string{
		"latest",
		"v1.0.0",
		"v1.0.0-alpha",
		"my-tag",
	}

	for _, tag := range validTags {
		assert.True(t, TagRegexp.MatchString(tag), "expected tag '%s' to be valid", tag)
	}
}

func TestDigestRegexp(t *testing.T) {
	validDigests := []string{
		"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"sha384:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	for _, digest := range validDigests {
		assert.True(t, DigestRegexp.MatchString(digest), "expected digest '%s' to be valid", digest)
	}
}

func TestAnchored(t *testing.T) {
	tests := []struct {
		name     string
		regexps  []*regexp.Regexp
		input    string
		expected bool
	}{
		{
			name:     "Anchored single regex match",
			regexps:  []*regexp.Regexp{TagRegexp},
			input:    "v1.0.0",
			expected: true,
		},
		{
			name:     "Anchored single regex mismatch",
			regexps:  []*regexp.Regexp{TagRegexp},
			input:    " v1.0.0 ",
			expected: false,
		},
		{
			name:     "Anchored multiple regex match",
			regexps:  []*regexp.Regexp{TagRegexp, DigestRegexp},
			input:    "v1.0.0sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: true,
		},
		{
			name:     "Anchored multiple regex mismatch",
			regexps:  []*regexp.Regexp{TagRegexp, DigestRegexp},
			input:    "v1.0.0 sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchoredRegexp := anchored(tt.regexps...)
			assert.Equal(t, tt.expected, anchoredRegexp.MatchString(tt.input))
		})
	}
}

func TestExpression(t *testing.T) {
	tests := []struct {
		name     string
		regexps  []*regexp.Regexp
		input    string
		expected bool
	}{
		{
			name:     "Expression single regex match",
			regexps:  []*regexp.Regexp{TagRegexp},
			input:    "v1.0.0",
			expected: true,
		},
		{
			name:     "Expression single regex mismatch",
			regexps:  []*regexp.Regexp{anchoredTagRegexp},
			input:    " v1.0.0 ",
			expected: false,
		},
		{
			name:     "Expression multiple regex match",
			regexps:  []*regexp.Regexp{TagRegexp, DigestRegexp},
			input:    "v1.0.0sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: true,
		},
		{
			name:     "Expression multiple regex mismatch",
			regexps:  []*regexp.Regexp{anchoredTagRegexp, DigestRegexp},
			input:    "v1.0.0 sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionRegexp := expression(tt.regexps...)
			assert.Equal(t, tt.expected, expressionRegexp.MatchString(tt.input))
		})
	}
}
