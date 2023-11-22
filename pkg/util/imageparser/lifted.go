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

import "regexp"

/*
This code is directly lifted from the distribution codebase as they haven't been exported.

For reference: https://github.com/distribution/distribution/blob/9329f6a62b67d5e06d50dc93997c7705a075fcd9/reference/regexp.go
*/

var (
	// match compiles the string to a regular expression.
	match = regexp.MustCompile
	// TagRegexp matches valid tag names. From docker/docker:graph/tags.go.
	TagRegexp = match(`[\w][\w.-]{0,127}`)

	// anchoredTagRegexp matches valid tag names, anchored at the start and
	// end of the matched string.
	anchoredTagRegexp = anchored(TagRegexp)

	// DigestRegexp matches valid digests.
	DigestRegexp = match(`[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`)

	// anchoredDigestRegexp matches valid digests, anchored at the start and
	// end of the matched string.
	anchoredDigestRegexp = anchored(DigestRegexp)
)

// anchored anchors the regular expression by adding start and end delimiters.
func anchored(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`^` + expression(res...).String() + `$`)
}

// expression defines a full expression, where each regular expression must
// follow the previous.
func expression(res ...*regexp.Regexp) *regexp.Regexp {
	var s string
	for _, re := range res {
		s += re.String()
	}

	return match(s)
}
