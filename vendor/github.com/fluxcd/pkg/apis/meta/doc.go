/*
Copyright 2020 The Flux authors

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

// Package meta contains the generic metadata APIs for use by GitOps Toolkit components.
//
// It is intended only to help adhere to Kubernetes API conventions, utility integrations, and Flux project considered
// best practices. It may therefore be suitable for usage by Kubernetes resources with no relationship to the GitOps
// Toolkit.
// +kubebuilder:object:generate=true
package meta
