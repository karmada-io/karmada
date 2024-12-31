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

package v1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

// addConversionFuncs ensures that the cache plugin can handle the field selectors for corev1 resources.
// It is copied from "k8s.io/kubernetes/pkg/apis/core/v1/conversion.go".
func addConversionFuncs(scheme *runtime.Scheme) error {
	// Add field conversion funcs.
	err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Pod"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				"spec.nodeName",
				"spec.restartPolicy",
				"spec.schedulerName",
				"spec.serviceAccountName",
				"spec.hostNetwork",
				"status.phase",
				"status.podIP",
				"status.podIPs",
				"status.nominatedNodeName":
				return label, value, nil
			// This is for backwards compatibility with old v1 clients which send spec.host
			case "spec.host":
				return "spec.nodeName", value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	)
	if err != nil {
		return err
	}
	err = scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Node"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name":
				return label, value, nil
			case "spec.unschedulable":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	)
	if err != nil {
		return err
	}
	err = scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("ReplicationController"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				"status.replicas":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		return err
	}
	if err := AddFieldLabelConversionsForEvent(scheme); err != nil {
		return err
	}
	if err := AddFieldLabelConversionsForNamespace(scheme); err != nil {
		return err
	}
	if err := AddFieldLabelConversionsForSecret(scheme); err != nil {
		return err
	}
	if err := AddFieldLabelConversionsForService(scheme); err != nil {
		return err
	}
	return nil
}

// AddFieldLabelConversionsForEvent adds field label conversions for Event.
func AddFieldLabelConversionsForEvent(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Event"),
		func(label, value string) (string, string, error) {
			switch label {
			case "involvedObject.kind",
				"involvedObject.namespace",
				"involvedObject.name",
				"involvedObject.uid",
				"involvedObject.apiVersion",
				"involvedObject.resourceVersion",
				"involvedObject.fieldPath",
				"reason",
				"reportingComponent",
				"source",
				"type",
				"metadata.namespace",
				"metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
}

// AddFieldLabelConversionsForNamespace adds field label conversions for Namespace.
func AddFieldLabelConversionsForNamespace(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Namespace"),
		func(label, value string) (string, string, error) {
			switch label {
			case "status.phase",
				"metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
}

// AddFieldLabelConversionsForSecret adds field label conversions for Secret.
func AddFieldLabelConversionsForSecret(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Secret"),
		func(label, value string) (string, string, error) {
			switch label {
			case "type",
				"metadata.namespace",
				"metadata.name":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
}

// AddFieldLabelConversionsForService adds field label conversions for Service.
func AddFieldLabelConversionsForService(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Service"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.namespace",
				"metadata.name",
				"spec.clusterIP",
				"spec.type":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
}
