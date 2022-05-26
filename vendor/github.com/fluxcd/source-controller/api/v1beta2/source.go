/*
Copyright 2022 The Flux authors

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

package v1beta2

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// SourceIndexKey is the key used for indexing objects based on their
	// referenced Source.
	SourceIndexKey string = ".metadata.source"
)

// Source interface must be supported by all API types.
// Source is the interface that provides generic access to the Artifact and
// interval. It must be supported by all kinds of the source.toolkit.fluxcd.io
// API group.
//
// +k8s:deepcopy-gen=false
type Source interface {
	runtime.Object
	// GetRequeueAfter returns the duration after which the source must be
	// reconciled again.
	GetRequeueAfter() time.Duration
	// GetArtifact returns the latest artifact from the source if present in
	// the status sub-resource.
	GetArtifact() *Artifact
}
