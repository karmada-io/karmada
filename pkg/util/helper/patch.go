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
		return []byte(`[]`), nil
	}
	if newFieldValue == nil {
		return GenJSONPatch(JSONPatchOPRemove, "", path, nil)
	}
	// The implementation of “add” and “replace” for JSON objects is actually the same
	// in “github.com/evanphx/json-patch/v5”, which is used by us and k8s.
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
