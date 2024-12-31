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

package helper

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// GenMergePatch will return a merge patch document capable of converting the
// original object to the modified object.
// The merge patch format is primarily intended for use with the HTTP PATCH method
// as a means of describing a set of modifications to a target resource's content.
func GenMergePatch(originalObj interface{}, modifiedObj interface{}) ([]byte, error) {
	originalBytes, err := json.Marshal(originalObj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original object: %w", err)
	}
	modifiedBytes, err := json.Marshal(modifiedObj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified object: %w", err)
	}
	patchBytes, err := jsonpatch.CreateMergePatch(originalBytes, modifiedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create a merge patch: %w", err)
	}

	// It's unnecessary to patch.
	if string(patchBytes) == "{}" {
		return nil, nil
	}

	return patchBytes, nil
}

// GenFieldMergePatch will return a merge patch document capable of converting the
// original field to the modified field.
// The merge patch format is primarily intended for use with the HTTP PATCH method
// as a means of describing a set of modifications to a target resource's content.
func GenFieldMergePatch(fieldName string, originField interface{}, modifiedField interface{}) ([]byte, error) {
	patchBytes, err := GenMergePatch(originField, modifiedField)
	if err != nil {
		return nil, err
	}
	if len(patchBytes) == 0 {
		return nil, nil
	}

	patchBytes = append(patchBytes, '}')
	patchBytes = append([]byte(`{"`+fieldName+`":`), patchBytes...)
	return patchBytes, nil
}
