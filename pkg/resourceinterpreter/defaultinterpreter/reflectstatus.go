package defaultinterpreter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type reflectStatusInterpreter func(object *unstructured.Unstructured) (*runtime.RawExtension, error)
