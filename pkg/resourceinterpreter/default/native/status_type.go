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

package native

import appsv1 "k8s.io/api/apps/v1"

// WrappedDeploymentStatus is a wrapper for appsv1.DeploymentStatus.
type WrappedDeploymentStatus struct {
	appsv1.DeploymentStatus `json:",inline"`

	// Generation holds the generation(.metadata.generation) of resource running on member cluster.
	Generation int64 `json:"generation,omitempty"`
	// ResourceTemplateGeneration holds the value of annotation resourcetemplate.karmada.io/generation grabbed
	// from resource running on member cluster.
	ResourceTemplateGeneration int64 `json:"resourceTemplateGeneration,omitempty"`
}
