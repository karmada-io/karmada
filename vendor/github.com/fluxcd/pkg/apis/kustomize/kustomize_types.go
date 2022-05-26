/*
Copyright 2021 The Flux authors

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

package kustomize

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Image contains an image name, a new name, a new tag or digest, which will replace the original name and tag.
type Image struct {
	// Name is a tag-less image name.
	// +required
	Name string `json:"name"`

	// NewName is the value used to replace the original name.
	// +optional
	NewName string `json:"newName,omitempty"`

	// NewTag is the value used to replace the original tag.
	// +optional
	NewTag string `json:"newTag,omitempty"`

	// Digest is the value used to replace the original image tag.
	// If digest is present NewTag value is ignored.
	// +optional
	Digest string `json:"digest,omitempty"`
}

// Selector specifies a set of resources. Any resource that matches intersection of all conditions is included in this
// set.
type Selector struct {
	// Group is the API group to select resources from.
	// Together with Version and Kind it is capable of unambiguously identifying and/or selecting resources.
	// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
	// +optional
	Group string `json:"group,omitempty"`

	// Version of the API Group to select resources from.
	// Together with Group and Kind it is capable of unambiguously identifying and/or selecting resources.
	// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
	// +optional
	Version string `json:"version,omitempty"`

	// Kind of the API Group to select resources from.
	// Together with Group and Version it is capable of unambiguously
	// identifying and/or selecting resources.
	// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/api-group.md
	// +optional
	Kind string `json:"kind,omitempty"`

	// Namespace to select resources from.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name to match resources with.
	// +optional
	Name string `json:"name,omitempty"`

	// AnnotationSelector is a string that follows the label selection expression
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	// It matches with the resource annotations.
	// +optional
	AnnotationSelector string `json:"annotationSelector,omitempty"`

	// LabelSelector is a string that follows the label selection expression
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	// It matches with the resource labels.
	// +optional
	LabelSelector string `json:"labelSelector,omitempty"`
}

// Patch contains an inline StrategicMerge or JSON6902 patch, and the target the patch should
// be applied to.
type Patch struct {
	// Patch contains an inline StrategicMerge patch or an inline JSON6902 patch with
	// an array of operation objects.
	// +required
	Patch string `json:"patch,omitempty"`

	// Target points to the resources that the patch document should be applied to.
	// +optional
	Target Selector `json:"target,omitempty"`
}

// JSON6902 is a JSON6902 operation object.
// https://datatracker.ietf.org/doc/html/rfc6902#section-4
type JSON6902 struct {
	// Op indicates the operation to perform. Its value MUST be one of "add", "remove", "replace", "move", "copy", or
	// "test".
	// https://datatracker.ietf.org/doc/html/rfc6902#section-4
	// +kubebuilder:validation:Enum=test;remove;add;replace;move;copy
	// +required
	Op string `json:"op"`

	// Path contains the JSON-pointer value that references a location within the target document where the operation
	// is performed. The meaning of the value depends on the value of Op.
	// +required
	Path string `json:"path"`

	// From contains a JSON-pointer value that references a location within the target document where the operation is
	// performed. The meaning of the value depends on the value of Op, and is NOT taken into account by all operations.
	// +optional
	From string `json:"from,omitempty"`

	// Value contains a valid JSON structure. The meaning of the value depends on the value of Op, and is NOT taken into
	// account by all operations.
	// +optional
	Value *apiextensionsv1.JSON `json:"value,omitempty"`
}

// JSON6902Patch contains a JSON6902 patch and the target the patch should be applied to.
type JSON6902Patch struct {
	// Patch contains the JSON6902 patch document with an array of operation objects.
	// +required
	Patch []JSON6902 `json:"patch"`

	// Target points to the resources that the patch document should be applied to.
	// +required
	Target Selector `json:"target"`
}
