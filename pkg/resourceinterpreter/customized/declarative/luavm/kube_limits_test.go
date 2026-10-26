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

package luavm

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestAccuratePodLimitsExactRoundTrip(t *testing.T) {
	template := &corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name: "app", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("9007199254740993"),
		}},
	}}}}
	vm := New(false, 1)
	const script = `local kube = require("kube")
function Limits(template)
    local original = template.spec.containers[1].resources.limits.memory
    local result = kube.accuratePodLimits(template)
    return result.memory, original, template.spec.containers[1].resources.limits.memory
end`
	result, err := vm.RunScript(script, "Limits", 3, template)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range result {
		text, ok := value.(lua.LString)
		if !ok {
			t.Fatalf("limit must remain a Quantity string, got %T", value)
		}
		got, err := resource.ParseQuantity(string(text))
		if err != nil || got.Cmp(resource.MustParse("9007199254740993")) != 0 {
			t.Fatalf("lost Quantity precision: %q, %v", text, err)
		}
	}
	got := helper.PodTemplateLimits(template)
	quantity := got[corev1.ResourceMemory]
	if quantity.Cmp(resource.MustParse("9007199254740993")) != 0 {
		t.Fatal("Go and Lua helpers disagree")
	}
}

func TestAccuratePodLimitsInvalidQuantity(t *testing.T) {
	vm := New(false, 1)
	const script = `local kube = require("kube")
function Invalid()
    return kube.accuratePodLimits({spec={containers={{name="app",resources={limits={cpu="not-a-quantity"}}}}}})
end`
	if _, err := vm.RunScript(script, "Invalid", 1); err == nil {
		t.Fatal("invalid Quantity must return an error")
	}
}
