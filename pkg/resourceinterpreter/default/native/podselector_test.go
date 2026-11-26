/*
Copyright 2026 The Karmada Authors.

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

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_deployPodSelector(t *testing.T) {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "nginx"},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"frontend"}},
		},
	}
	object := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		Spec:     appsv1.DeploymentSpec{Selector: selector},
	}

	unstructuredObject, err := helper.ToUnstructured(object)
	if err != nil {
		t.Fatalf("Failed to transform object, error: %v", err)
	}

	got, err := deployPodSelector(unstructuredObject)
	assert.NoError(t, err)
	assert.Equal(t, selector, got)
}

func TestDefaultInterpreter_GetPodSelector(t *testing.T) {
	interpreter := NewDefaultInterpreter()

	deploy, err := helper.ToUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		Spec:     appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}}},
	})
	assert.NoError(t, err)

	assert.True(t, interpreter.HookEnabled(deploy.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretPodSelector))
	got, err := interpreter.GetPodSelector(deploy)
	assert.NoError(t, err)
	assert.Equal(t, &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}}, got)

	sts, err := helper.ToUnstructured(&appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
	})
	assert.NoError(t, err)

	assert.False(t, interpreter.HookEnabled(sts.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretPodSelector))
	_, err = interpreter.GetPodSelector(sts)
	assert.Error(t, err)
}
