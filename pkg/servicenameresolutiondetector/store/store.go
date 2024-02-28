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

package store

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ConditionStore provides an interface to load or store conditions.
type ConditionStore interface {
	Load(key string) (*metav1.Condition, error)
	Store(key string, cond *metav1.Condition) error
	ListAll() ([]metav1.Condition, error)
}
