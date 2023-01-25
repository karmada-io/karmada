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

// Package labels contains functions to validate and compare values used in Kubernetes labels.
package labels

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/util/validation"
)

// MustFormatValue returns the passed inputLabelValue if it meets the standards for a Kubernetes label value.
// If the name is not a valid label value this function returns a hash which meets the requirements.
func MustFormatValue(str string) string {
	// a valid Kubernetes label value must:
	// - be less than 64 characters long.
	// - be an empty string OR consist of alphanumeric characters, '-', '_' or '.'.
	// - start and end with an alphanumeric character
	if len(validation.IsValidLabelValue(str)) == 0 {
		return str
	}
	hasher := fnv.New32a()
	_, err := hasher.Write([]byte(str))
	if err != nil {
		// At time of writing the implementation of fnv's Write function can never return an error.
		// If this changes in a future go version this function will panic.
		panic(err)
	}
	return fmt.Sprintf("hash_%s_z", base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hasher.Sum(nil)))
}

// MustEqualValue returns true if the actualLabelValue equals either the inputLabelValue or the hashed
// value of the inputLabelValue.
func MustEqualValue(str, labelValue string) bool {
	return labelValue == MustFormatValue(str)
}
