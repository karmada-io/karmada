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

package v1alpha2

import "sigs.k8s.io/controller-runtime/pkg/conversion"

// Check if our ResourceBinding implements necessary interface
var _ conversion.Hub = &ResourceBinding{}

// Check if our ClusterResourceBinding implements necessary interface
var _ conversion.Hub = &ClusterResourceBinding{}

// Hub marks this type as a conversion hub.
func (*ResourceBinding) Hub() {}

// Hub marks this type as a conversion hub.
func (*ClusterResourceBinding) Hub() {}
