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
