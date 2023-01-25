/*
Copyright 2020 The Kubernetes Authors.

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

// Package annotations implements annotation helper functions.
package annotations

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// IsPaused returns true if the Cluster is paused or the object has the `paused` annotation.
func IsPaused(cluster *clusterv1.Cluster, o metav1.Object) bool {
	if cluster.Spec.Paused {
		return true
	}
	return HasPaused(o)
}

// IsExternallyManaged returns true if the object has the `managed-by` annotation.
func IsExternallyManaged(o metav1.Object) bool {
	return hasAnnotation(o, clusterv1.ManagedByAnnotation)
}

// HasPaused returns true if the object has the `paused` annotation.
func HasPaused(o metav1.Object) bool {
	return hasAnnotation(o, clusterv1.PausedAnnotation)
}

// HasSkipRemediation returns true if the object has the `skip-remediation` annotation.
func HasSkipRemediation(o metav1.Object) bool {
	return hasAnnotation(o, clusterv1.MachineSkipRemediationAnnotation)
}

// HasWithPrefix returns true if at least one of the annotations has the prefix specified.
func HasWithPrefix(prefix string, annotations map[string]string) bool {
	for key := range annotations {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// ReplicasManagedByExternalAutoscaler returns true if the standard annotation for external autoscaler is present.
func ReplicasManagedByExternalAutoscaler(o metav1.Object) bool {
	return hasTruthyAnnotationValue(o, clusterv1.ReplicasManagedByAnnotation)
}

// AddAnnotations sets the desired annotations on the object and returns true if the annotations have changed.
func AddAnnotations(o metav1.Object, desired map[string]string) bool {
	if len(desired) == 0 {
		return false
	}
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		o.SetAnnotations(annotations)
	}
	hasChanged := false
	for k, v := range desired {
		if cur, ok := annotations[k]; !ok || cur != v {
			annotations[k] = v
			hasChanged = true
		}
	}
	return hasChanged
}

// hasAnnotation returns true if the object has the specified annotation.
func hasAnnotation(o metav1.Object, annotation string) bool {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[annotation]
	return ok
}

// hasTruthyAnnotationValue returns true if the object has an annotation with a value that is not "false".
func hasTruthyAnnotationValue(o metav1.Object, annotation string) bool {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return false
	}
	if val, ok := annotations[annotation]; ok {
		return val != "false"
	}
	return false
}
