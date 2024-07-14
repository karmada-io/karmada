/*
Copyright 2023 The Karmada Authors.

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
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestRetentionRule_Name(t *testing.T) {
	r := &retentionRule{}
	expected := string(configv1alpha1.InterpreterOperationRetain)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestRetentionRule_Document(t *testing.T) {
	rule := &retentionRule{}
	expected := `This rule is used to retain runtime values to the desired specification.
The script should implement a function as follows:
function Retain(desiredObj, observedObj)
  desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo
  return desiredObj
end`
	assert.Equal(t, expected, rule.Document())
}

func TestRetentionRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		customization := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					Retention: &configv1alpha1.LocalValueRetention{
						LuaScript: "hello world",
					},
				},
			},
		}

		rule := &retentionRule{}
		script := rule.GetScript(customization)

		expectedScript := customization.Spec.Customizations.Retention.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		customization := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					Retention: nil,
				},
			},
		}

		rule := &retentionRule{}
		script := rule.GetScript(customization)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestRetentionRule_SetScript(t *testing.T) {
	t.Run("Set non-empty script", func(t *testing.T) {
		customization := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					Retention: &configv1alpha1.LocalValueRetention{},
				},
			},
		}

		rule := &retentionRule{}
		script := "function Retain(desiredObj, observedObj)\n  desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo\n  return desiredObj\nend"

		rule.SetScript(customization, script)
		if customization.Spec.Customizations.Retention.LuaScript != script {
			t.Errorf("Expected script to be set to %q, but got %q", script, customization.Spec.Customizations.Retention.LuaScript)
		}
	})

	t.Run("Set empty script", func(t *testing.T) {
		customization := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					Retention: nil,
				},
			},
		}

		rule := &retentionRule{}
		script := ""

		rule.SetScript(customization, script)
		if customization.Spec.Customizations.Retention != nil {
			t.Errorf("Expected retention to be nil, but got %v", customization.Spec.Customizations.Retention)
		}
	})

	t.Run("Set empty Retention", func(t *testing.T) {
		customization := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					Retention: nil,
				},
			},
		}

		rule := &retentionRule{}
		script := "function Retain(desiredObj, observedObj)\n  desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo\n  return desiredObj\nend"

		rule.SetScript(customization, script)
		if customization.Spec.Customizations.Retention.LuaScript != script {
			t.Errorf("Expected script to be set to %q, but got %q", script, customization.Spec.Customizations.Retention.LuaScript)
		}
	})
}

func TestReplicaResourceRule_Name(t *testing.T) {
	r := &replicaResourceRule{}
	expected := string(configv1alpha1.InterpreterOperationInterpretReplica)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestReplicaResourceRule_Document(t *testing.T) {
	rule := &replicaResourceRule{}
	expected := `This rule is used to discover the resource's replica as well as resource requirements.
The script should implement a function as follows:
function GetReplicas(desiredObj)
  replica = desiredObj.spec.replicas
  nodeClaim = {}
  nodeClaim.hardNodeAffinity = {}
  nodeClaim.nodeSelector = {}
  nodeClaim.tolerations = {}
  return replica, nodeClaim
end`
	assert.Equal(t, expected, rule.Document())
}

func TestReplicaResourceRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{
						LuaScript: "return 'test script'",
					},
				},
			},
		}
		r := &replicaResourceRule{}
		script := r.GetScript(c)
		expectedScript := c.Spec.Customizations.ReplicaResource.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					ReplicaResource: nil,
				},
			},
		}

		rule := &replicaResourceRule{}
		script := rule.GetScript(c)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestReplicaResourceRule_SetScript(t *testing.T) {
	c := &configv1alpha1.ResourceInterpreterCustomization{
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Customizations: configv1alpha1.CustomizationRules{
				ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{},
			},
		},
	}
	r := &replicaResourceRule{}

	t.Run("Empty Script", func(t *testing.T) {
		r.SetScript(c, "")
		if c.Spec.Customizations.ReplicaResource != nil {
			t.Errorf("Expected ReplicaResource to be nil, but got %v", c.Spec.Customizations.ReplicaResource)
		}
	})

	t.Run("Non-Empty Script", func(t *testing.T) {
		c.Spec.Customizations.ReplicaResource = nil
		script := "test script"
		r.SetScript(c, script)
		if c.Spec.Customizations.ReplicaResource == nil {
			t.Errorf("Expected ReplicaResource to be non-nil, but got nil")
		}
		if c.Spec.Customizations.ReplicaResource.LuaScript != "test script" {
			t.Errorf("Expected ReplicaResource.LuaScript to be %s, but got %s", script, c.Spec.Customizations.ReplicaResource.LuaScript)
		}
	})
}

func TestReplicaRevisionRule_Name(t *testing.T) {
	r := &replicaRevisionRule{}
	expected := string(configv1alpha1.InterpreterOperationReviseReplica)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestReplicaRevisionRule_Document(t *testing.T) {
	rule := &replicaRevisionRule{}
	expected := `This rule is used to revise replicas in the desired specification.
The script should implement a function as follows:
function ReviseReplica(desiredObj, desiredReplica)
  desiredObj.spec.replicas = desiredReplica
  return desiredObj
end`
	assert.Equal(t, expected, rule.Document())
}

func TestReplicaRevisionRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					ReplicaRevision: &configv1alpha1.ReplicaRevision{
						LuaScript: "return 'test script'",
					},
				},
			},
		}
		r := &replicaRevisionRule{}
		script := r.GetScript(c)
		expectedScript := c.Spec.Customizations.ReplicaRevision.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					ReplicaRevision: nil,
				},
			},
		}

		rule := &replicaRevisionRule{}
		script := rule.GetScript(c)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestReplicaRevisionRule_SetScript(t *testing.T) {
	c := &configv1alpha1.ResourceInterpreterCustomization{
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Customizations: configv1alpha1.CustomizationRules{
				ReplicaRevision: &configv1alpha1.ReplicaRevision{},
			},
		},
	}
	r := &replicaRevisionRule{}

	t.Run("Empty Script", func(t *testing.T) {
		r.SetScript(c, "")
		if c.Spec.Customizations.ReplicaRevision != nil {
			t.Errorf("Expected ReplicaRevision to be nil, but got %v", c.Spec.Customizations.ReplicaRevision)
		}
	})

	t.Run("Non-Empty Script", func(t *testing.T) {
		c.Spec.Customizations.ReplicaRevision = nil
		script := "test script"
		r.SetScript(c, script)
		if c.Spec.Customizations.ReplicaRevision == nil {
			t.Errorf("Expected ReplicaRevision to be non-nil, but got nil")
		}
		if c.Spec.Customizations.ReplicaRevision.LuaScript != "test script" {
			t.Errorf("Expected ReplicaRevision.LuaScript to be %s, but got %s", script, c.Spec.Customizations.ReplicaRevision.LuaScript)
		}
	})
}

func TestStatusReflectionRule_Name(t *testing.T) {
	r := &statusReflectionRule{}
	expected := string(configv1alpha1.InterpreterOperationInterpretStatus)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestStatusReflectionRule_Document(t *testing.T) {
	rule := &statusReflectionRule{}
	expected := `This rule is used to get the status from the observed specification.
The script should implement a function as follows:
function ReflectStatus(observedObj)
  status = {}
  status.readyReplicas = observedObj.status.observedObj
  return status
end`
	assert.Equal(t, expected, rule.Document())
}

func TestStatusReflectionRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					StatusReflection: &configv1alpha1.StatusReflection{
						LuaScript: "return 'test script'",
					},
				},
			},
		}
		r := &statusReflectionRule{}
		script := r.GetScript(c)
		expectedScript := c.Spec.Customizations.StatusReflection.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					StatusReflection: nil,
				},
			},
		}

		rule := &statusReflectionRule{}
		script := rule.GetScript(c)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestStatusReflectionRule_SetScript(t *testing.T) {
	c := &configv1alpha1.ResourceInterpreterCustomization{
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Customizations: configv1alpha1.CustomizationRules{
				StatusReflection: &configv1alpha1.StatusReflection{},
			},
		},
	}
	r := &statusReflectionRule{}

	t.Run("Empty Script", func(t *testing.T) {
		r.SetScript(c, "")
		if c.Spec.Customizations.StatusReflection != nil {
			t.Errorf("Expected StatusReflection to be nil, but got %v", c.Spec.Customizations.StatusReflection)
		}
	})

	t.Run("Non-Empty Script", func(t *testing.T) {
		c.Spec.Customizations.StatusReflection = nil
		script := "test script"
		r.SetScript(c, script)
		if c.Spec.Customizations.StatusReflection == nil {
			t.Errorf("Expected StatusReflection to be non-nil, but got nil")
		}
		if c.Spec.Customizations.StatusReflection.LuaScript != "test script" {
			t.Errorf("Expected StatusReflection.LuaScript to be %s, but got %s", script, c.Spec.Customizations.StatusReflection.LuaScript)
		}
	})
}

func TestStatusAggregationRule_Name(t *testing.T) {
	r := &statusAggregationRule{}
	expected := string(configv1alpha1.InterpreterOperationAggregateStatus)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestStatusAggregationRule_Document(t *testing.T) {
	rule := &statusAggregationRule{}
	expected := `This rule is used to aggregate decentralized statuses to the desired specification.
The script should implement a function as follows:
function AggregateStatus(desiredObj, statusItems)
  for i = 1, #items do
    desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + items[i].readyReplicas
  end
  return desiredObj
end`
	assert.Equal(t, expected, rule.Document())
}

func TestStatusAggregationRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					StatusAggregation: &configv1alpha1.StatusAggregation{
						LuaScript: "return 'test script'",
					},
				},
			},
		}
		r := &statusAggregationRule{}
		script := r.GetScript(c)
		expectedScript := c.Spec.Customizations.StatusAggregation.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					StatusAggregation: nil,
				},
			},
		}

		rule := &statusAggregationRule{}
		script := rule.GetScript(c)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestStatusAggregationRule_SetScript(t *testing.T) {
	c := &configv1alpha1.ResourceInterpreterCustomization{
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Customizations: configv1alpha1.CustomizationRules{
				StatusAggregation: &configv1alpha1.StatusAggregation{},
			},
		},
	}
	r := &statusAggregationRule{}

	t.Run("Empty Script", func(t *testing.T) {
		r.SetScript(c, "")
		if c.Spec.Customizations.StatusAggregation != nil {
			t.Errorf("Expected StatusAggregation to be nil, but got %v", c.Spec.Customizations.StatusAggregation)
		}
	})

	t.Run("Non-Empty Script", func(t *testing.T) {
		c.Spec.Customizations.StatusAggregation = nil
		script := "test script"
		r.SetScript(c, script)
		if c.Spec.Customizations.StatusAggregation == nil {
			t.Errorf("Expected Customizations to be non-nil, but got nil")
		}
		if c.Spec.Customizations.StatusAggregation.LuaScript != "test script" {
			t.Errorf("Expected StatusAggregation.LuaScript to be %s, but got %s", script, c.Spec.Customizations.StatusAggregation.LuaScript)
		}
	})
}

func TestHealthInterpretationRule_Name(t *testing.T) {
	r := &healthInterpretationRule{}
	expected := string(configv1alpha1.InterpreterOperationInterpretHealth)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestHealthInterpretationRule_Document(t *testing.T) {
	rule := &healthInterpretationRule{}
	expected := `This rule is used to assess the health state of a specific resource.
The script should implement a function as follows:
luaScript: >
function InterpretHealth(observedObj)
  if observedObj.status.readyReplicas == observedObj.spec.replicas then
    return true
  end
end`
	assert.Equal(t, expected, rule.Document())
}

func TestHealthInterpretationRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					HealthInterpretation: &configv1alpha1.HealthInterpretation{
						LuaScript: "return 'test script'",
					},
				},
			},
		}
		r := &healthInterpretationRule{}
		script := r.GetScript(c)
		expectedScript := c.Spec.Customizations.HealthInterpretation.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					HealthInterpretation: nil,
				},
			},
		}

		rule := &healthInterpretationRule{}
		script := rule.GetScript(c)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestHealthInterpretationRule_SetScript(t *testing.T) {
	c := &configv1alpha1.ResourceInterpreterCustomization{
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Customizations: configv1alpha1.CustomizationRules{
				HealthInterpretation: &configv1alpha1.HealthInterpretation{},
			},
		},
	}
	r := &healthInterpretationRule{}

	t.Run("Empty Script", func(t *testing.T) {
		r.SetScript(c, "")
		if c.Spec.Customizations.HealthInterpretation != nil {
			t.Errorf("Expected HealthInterpretation to be nil, but got %v", c.Spec.Customizations.HealthInterpretation)
		}
	})

	t.Run("Non-Empty Script", func(t *testing.T) {
		c.Spec.Customizations.HealthInterpretation = nil
		script := "test script"
		r.SetScript(c, script)
		if c.Spec.Customizations.HealthInterpretation == nil {
			t.Errorf("Expected HealthInterpretation to be non-nil, but got nil")
		}
		if c.Spec.Customizations.HealthInterpretation.LuaScript != "test script" {
			t.Errorf("Expected HealthInterpretation.LuaScript to be %s, but got %s", script, c.Spec.Customizations.HealthInterpretation.LuaScript)
		}
	})
}

func TestDependencyInterpretationRule_Name(t *testing.T) {
	r := &dependencyInterpretationRule{}
	expected := string(configv1alpha1.InterpreterOperationInterpretDependency)
	actual := r.Name()
	assert.Equal(t, expected, actual, "Name should return %v", expected)
}

func TestDependencyInterpretationRule_Document(t *testing.T) {
	rule := &dependencyInterpretationRule{}
	expected := ` This rule is used to interpret the dependencies of a specific resource.
The script should implement a function as follows:
function GetDependencies(desiredObj)
  dependencies = {}
  serviceAccountName = desiredObj.spec.template.spec.serviceAccountName
  if serviceAccountName ~= nil and serviceAccountName ~= "default" then
    dependency = {}
    dependency.apiVersion = "v1"
    dependency.kind = "ServiceAccount"
    dependency.name = serviceAccountName
    dependency.namespace = desiredObj.metadata.namespace
    dependencies[1] = dependency
  end
  return dependencies
end`
	assert.Equal(t, expected, rule.Document())
}

func TestDependencyInterpretationRule_GetScript(t *testing.T) {
	t.Run("WithScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					DependencyInterpretation: &configv1alpha1.DependencyInterpretation{
						LuaScript: "return 'test script'",
					},
				},
			},
		}
		r := &dependencyInterpretationRule{}
		script := r.GetScript(c)
		expectedScript := c.Spec.Customizations.DependencyInterpretation.LuaScript
		if script != expectedScript {
			t.Errorf("GetScript() returned %q, expected %q", script, expectedScript)
		}
	})

	t.Run("WithoutScript", func(t *testing.T) {
		c := &configv1alpha1.ResourceInterpreterCustomization{
			Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
				Customizations: configv1alpha1.CustomizationRules{
					DependencyInterpretation: nil,
				},
			},
		}

		rule := &dependencyInterpretationRule{}
		script := rule.GetScript(c)

		if script != "" {
			t.Errorf("GetScript() returned %q, expected an empty string", script)
		}
	})
}

func TestDependencyInterpretationRule_SetScript(t *testing.T) {
	c := &configv1alpha1.ResourceInterpreterCustomization{
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Customizations: configv1alpha1.CustomizationRules{
				DependencyInterpretation: &configv1alpha1.DependencyInterpretation{},
			},
		},
	}
	r := &dependencyInterpretationRule{}

	t.Run("Empty Script", func(t *testing.T) {
		r.SetScript(c, "")
		if c.Spec.Customizations.DependencyInterpretation != nil {
			t.Errorf("Expected DependencyInterpretation to be nil, but got %v", c.Spec.Customizations.DependencyInterpretation)
		}
	})

	t.Run("Non-Empty Script", func(t *testing.T) {
		c.Spec.Customizations.DependencyInterpretation = nil
		script := "test script"
		r.SetScript(c, script)
		if c.Spec.Customizations.DependencyInterpretation == nil {
			t.Errorf("Expected DependencyInterpretation to be non-nil, but got nil")
		}
		if c.Spec.Customizations.DependencyInterpretation.LuaScript != "test script" {
			t.Errorf("Expected DependencyInterpretation.LuaScript to be %s, but got %s", script, c.Spec.Customizations.DependencyInterpretation.LuaScript)
		}
	})
}

func TestRulesNames(t *testing.T) {
	rule1 := &healthInterpretationRule{}
	rule2 := &dependencyInterpretationRule{}
	rules := Rules{rule1, rule2}

	expectedNames := []string{rule1.Name(), rule2.Name()}
	actualNames := rules.Names()

	if !reflect.DeepEqual(actualNames, expectedNames) {
		t.Errorf("Expected names %v, but got %v", expectedNames, actualNames)
	}
}

func TestGetByOperation(t *testing.T) {
	rules := Rules{&healthInterpretationRule{}, &dependencyInterpretationRule{}}

	tests := []struct {
		name         string
		operation    string
		expectedRule Rule
	}{
		{
			name:         "valid operation name",
			operation:    "InterpretHealth",
			expectedRule: &healthInterpretationRule{},
		},
		{
			name:         "invalid operation name",
			operation:    "invalid",
			expectedRule: nil,
		},
		{
			name:         "empty operation name",
			operation:    "",
			expectedRule: nil,
		},
		{
			name:         "case-insensitive operation name",
			operation:    "InterpretDEPendency",
			expectedRule: &dependencyInterpretationRule{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualRule := rules.GetByOperation(tt.operation)
			if actualRule != tt.expectedRule {
				t.Errorf("Expected rule %v, but got %v", tt.expectedRule, actualRule)
			}
		})
	}
}

func TestRules_Get(t *testing.T) {
	rule1 := &healthInterpretationRule{}
	rule2 := &dependencyInterpretationRule{}
	rules := Rules{rule1, rule2}

	// Test getting a rule that exists
	result := rules.Get("InterpretHealth")
	if result != rule1 {
		t.Errorf("Expected rule1, but got %v", result)
	}

	// Test getting a rule that does not exist
	result = rules.Get("NonexistentRule")
	if result != nil {
		t.Errorf("Expected nil, but got %v", result)
	}
}

func TestGetDesiredObjectOrError(t *testing.T) {
	args := RuleArgs{
		Desired: nil,
	}
	_, err := args.getDesiredObjectOrError()
	expectedErr := "desired, desired-file options are not set"
	if err == nil || err.Error() != expectedErr {
		t.Errorf("getDesiredObjectOrError() returned unexpected error: %v", err)
	}
}

func TestGetObservedObjectOrError(t *testing.T) {
	args := RuleArgs{}
	_, err := args.getObservedObjectOrError()
	if err == nil {
		t.Errorf("Expected error, but got nil")
	}

	obj := &unstructured.Unstructured{}
	args.Observed = obj
	result, err := args.getObservedObjectOrError()
	if err != nil {
		t.Errorf("Expected nil error, but got %v", err)
	}
	if result != obj {
		t.Errorf("Expected %v, but got %v", obj, result)
	}
}

func TestGetObjectOrError(t *testing.T) {
	desired := &unstructured.Unstructured{}
	observed := &unstructured.Unstructured{}

	t.Run("Both desired and observed objects are nil", func(t *testing.T) {
		args := RuleArgs{}
		_, err := args.getObjectOrError()
		if err == nil {
			t.Errorf("Expected error, but got nil")
		}
	})

	t.Run("Both desired and observed objects are set", func(t *testing.T) {
		args := RuleArgs{Desired: desired, Observed: observed}
		_, err := args.getObjectOrError()
		if err == nil {
			t.Errorf("Expected error, but got nil")
		}
	})

	t.Run("Only desired object is set", func(t *testing.T) {
		args := RuleArgs{Desired: desired}
		obj, err := args.getObjectOrError()
		if err != nil {
			t.Errorf("Expected nil error, but got %v", err)
		}
		if obj != desired {
			t.Errorf("Expected %v, but got %v", desired, obj)
		}
	})

	t.Run("Only observed object is set", func(t *testing.T) {
		args := RuleArgs{Observed: observed}
		obj, err := args.getObjectOrError()
		if err != nil {
			t.Errorf("Expected nil error, but got %v", err)
		}
		if obj != observed {
			t.Errorf("Expected %v, but got %v", observed, obj)
		}
	})
}

func TestNewRuleResult(t *testing.T) {
	result := newRuleResult()
	if result.Err != nil {
		t.Errorf("Expected error to be nil, but got %v", result.Err)
	}
	if len(result.Results) != 0 {
		t.Errorf("Expected results to be empty, but got %v", result.Results)
	}
}

func TestNewRuleResultWithError(t *testing.T) {
	err := fmt.Errorf("test error")
	result := newRuleResultWithError(err)
	if result.Err != err {
		t.Errorf("expected error %v, but got %v", err, result.Err)
	}
}

func TestRuleResultAdd(t *testing.T) {
	r := newRuleResult()
	r.add("test1", "value1")
	r.add("test2", 2)
	if len(r.Results) != 2 {
		t.Errorf("Expected 2 results, but got %d", len(r.Results))
	}
	if r.Results[0].Name != "test1" || r.Results[0].Value != "value1" {
		t.Errorf("Unexpected result at index 0: %v", r.Results[0])
	}
	if r.Results[1].Name != "test2" || r.Results[1].Value != 2 {
		t.Errorf("Unexpected result at index 1: %v", r.Results[1])
	}
}
