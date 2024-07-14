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

package helper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/karmada-io/karmada/pkg/util"
)

// ConvertToTypedObject converts an unstructured object to typed.
func ConvertToTypedObject(in, out interface{}) error {
	if in == nil || out == nil {
		return fmt.Errorf("convert objects should not be nil")
	}

	switch v := in.(type) {
	case *unstructured.Unstructured:
		return runtime.DefaultUnstructuredConverter.FromUnstructured(v.UnstructuredContent(), out)
	case map[string]interface{}:
		return runtime.DefaultUnstructuredConverter.FromUnstructured(v, out)
	default:
		return fmt.Errorf("convert object must be pointer of unstructured or map[string]interface{}")
	}
}

// ApplyReplica applies the Replica value for the specific field.
func ApplyReplica(workload *unstructured.Unstructured, desireReplica int64, field string) error {
	_, ok, err := unstructured.NestedInt64(workload.Object, util.SpecField, field)
	if err != nil {
		return err
	}
	if ok {
		err := unstructured.SetNestedField(workload.Object, desireReplica, util.SpecField, field)
		if err != nil {
			return err
		}
	}
	return nil
}

// ToUnstructured converts a typed object to an unstructured object.
func ToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uncastObj}, nil
}
