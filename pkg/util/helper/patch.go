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
