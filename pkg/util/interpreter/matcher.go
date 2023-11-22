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

package interpreter

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

const (
	// Wildcard indicates that any value can be matched.
	Wildcard = "*"
)

// Matcher determines if the Object matches the Rule.
type Matcher struct {
	ObjGVK    schema.GroupVersionKind
	Operation configv1alpha1.InterpreterOperation
	Rule      configv1alpha1.RuleWithOperations
}

// Matches tells if the Operation, Object matches the Rule.
func (m *Matcher) Matches() bool {
	return m.operation() && m.group() && m.version() && m.kind()
}

func (m *Matcher) operation() bool {
	for _, op := range m.Rule.Operations {
		if op == configv1alpha1.InterpreterOperationAll || op == m.Operation {
			return true
		}
	}
	return false
}

func (m *Matcher) group() bool {
	return exactOrWildcard(m.ObjGVK.Group, m.Rule.APIGroups)
}

func (m *Matcher) version() bool {
	return exactOrWildcard(m.ObjGVK.Version, m.Rule.APIVersions)
}

func (m *Matcher) kind() bool {
	return exactOrWildcard(m.ObjGVK.Kind, m.Rule.Kinds)
}

func exactOrWildcard(requested string, items []string) bool {
	for _, item := range items {
		if item == Wildcard || item == requested {
			return true
		}
	}
	return false
}
