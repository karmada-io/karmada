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

package helper

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// UpdateStatus updates the given object's status in the Kubernetes
// cluster. The object's desired state must be reconciled with the existing
// state inside the passed in callback MutateFn.
//
// The MutateFn is called when updating an object's status.
//
// It returns the executed operation and an error.
//
// Note: changes to any sub-resource other than status will be ignored.
// Changes to the status sub-resource will only be applied if the object
// already exist.
func UpdateStatus(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	key := client.ObjectKeyFromObject(obj)

	if err := mutate(f, key, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Extract status from the mutated object to construct a JSON Patch
	// that directly updates /status path.
	// FYI: https://github.com/karmada-io/karmada/issues/6858
	status, err := getStatusFromObject(obj)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	patch, err := buildStatusPatch(status)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	// Apply JSON Patch directly to /status
	if err := c.Status().Patch(ctx, obj, patch); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultUpdatedStatusOnly, nil
}

func getStatusFromObject(obj client.Object) (json.RawMessage, error) {
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(objBytes, &fields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}

	statusRaw, ok := fields["status"]
	if !ok {
		return nil, fmt.Errorf("object %T does not have status field", obj)
	}

	return statusRaw, nil
}

func buildStatusPatch(statusRaw json.RawMessage) (client.Patch, error) {
	type patchOp struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	}

	ops := []patchOp{
		{
			Op:    "add",
			Path:  "/status",
			Value: statusRaw,
		},
	}

	patchBytes, err := json.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %w", err)
	}

	return client.RawPatch(types.JSONPatchType, patchBytes), nil
}

// mutate wraps a MutateFn and applies validation to its result.
func mutate(f controllerutil.MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}
