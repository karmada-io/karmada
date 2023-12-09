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
	"reflect"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// RFC6902 JSONPatch operations
const (
	JSONPatchOPAdd     = "add"
	JSONPatchOPReplace = "replace"
	JSONPatchOPRemove  = "remove"
	JSONPatchOPMove    = "move"
	JSONPatchOPCopy    = "copy"
	JSONPatchOPTest    = "test"
)

type jsonPatch struct {
	OP    string      `json:"op"`
	From  string      `json:"from,omitempty"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

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

// GenReplaceFieldJSONPatch returns the RFC6902 JSONPatch array as []byte, which is used to simply
// add/replace/delete certain JSON **Object** field.
func GenReplaceFieldJSONPatch(path string, originalFieldValue, newFieldValue interface{}) ([]byte, error) {
	if reflect.DeepEqual(originalFieldValue, newFieldValue) {
		return nil, nil
	}
	if newFieldValue == nil {
		return GenJSONPatch(JSONPatchOPRemove, "", path, nil)
	}
	// The implementation of "add" and "replace" for JSON objects is actually the same
	// in "github.com/evanphx/json-patch/v5", which is used by Karmada and K8s.
	// We implemented it here just to follow the RFC6902.
	if originalFieldValue == nil {
		return GenJSONPatch(JSONPatchOPAdd, "", path, newFieldValue)
	}
	return GenJSONPatch(JSONPatchOPReplace, "", path, newFieldValue)
}

// GenJSONPatch return JSONPatch array as []byte according to RFC6902
func GenJSONPatch(op, from, path string, value interface{}) ([]byte, error) {
	jp := jsonPatch{
		OP:   op,
		Path: path,
	}
	switch op {
	case JSONPatchOPAdd, JSONPatchOPReplace, JSONPatchOPTest:
		jp.Value = value
	case JSONPatchOPMove, JSONPatchOPCopy:
		jp.From = from
	case JSONPatchOPRemove:
	default:
		return nil, fmt.Errorf("unrecognized JSONPatch OP: %s", op)
	}

	return json.Marshal([]jsonPatch{jp})
}
