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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindApplication is kind name of Application.
	ResourceKindApplication = "Application"
	// ResourceSingularApplication is singular name of Application.
	ResourceSingularApplication = "application"
	// ResourcePluralApplication is plural name of Application.
	ResourcePluralApplication = "applications"
	// ResourceNamespaceScopedApplication indicates if Application is NamespaceScoped.
	ResourceNamespaceScopedApplication = true
)

// Application represents a Karmada application
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ApplicationSpec `json:"spec,omitempty"`
}

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// RelatedApplications specifies other applications that should be migrated together during failover.
	// These applications must exist in the same namespace as this application.
	// +optional
	// +listType=set
	RelatedApplications []string `json:"relatedApplications,omitempty"`
}

// ApplicationList contains a list of Application
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}
