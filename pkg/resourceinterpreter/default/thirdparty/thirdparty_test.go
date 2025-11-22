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

package thirdparty

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/luavm"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

var rules interpreter.Rules = interpreter.AllResourceInterpreterCustomizationRules
var checker = conversion.EqualitiesOrDie(
	func(a, b resource.Quantity) bool {
		return a.Equal(b)
	})

func checkScript(script string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	l, err := luavm.NewWithContext(ctx)
	if err != nil {
		return err
	}
	defer l.Close()
	_, err = l.LoadString(script)
	return err
}

func getObj(t *testing.T, path string) *unstructured.Unstructured {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := yaml.ToJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	obj := make(map[string]interface{})
	err = json.Unmarshal(jsonData, &obj)
	if err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func getAggregatedStatusItems(t *testing.T, path string) []workv1alpha2.AggregatedStatusItem {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var statusItems []workv1alpha2.AggregatedStatusItem
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		statusItem := &workv1alpha2.AggregatedStatusItem{}
		err = decoder.Decode(statusItem)
		if err != nil {
			break
		}
		statusItems = append(statusItems, *statusItem)
	}
	if err != io.EOF {
		t.Fatal(err)
	}

	return statusItems
}

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	DesiredInputPath  string                 `yaml:"desiredInputPath,omitempty"`
	ObservedInputPath string                 `yaml:"observedInputPath,omitempty"`
	StatusInputPath   string                 `yaml:"statusInputPath,omitempty"`
	DesiredResults    map[string]interface{} `yaml:"desiredResults,omitempty"`
	Operation         string                 `yaml:"operation"`
}

func checkInterpretationRule(t *testing.T, path string, configs []*configv1alpha1.ResourceInterpreterCustomization) {
	ipt := declarative.NewConfigurableInterpreter(nil)
	ipt.LoadConfig(configs)

	dir := filepath.Dir(path)
	yamlBytes, err := os.ReadFile(dir + string(os.PathSeparator) + "customizations_tests.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var resourceTest TestStructure
	err = yaml.Unmarshal(yamlBytes, &resourceTest)
	if err != nil {
		t.Fatal(err)
	}
	for _, customization := range configs {
		for _, input := range resourceTest.Tests {
			rule := rules.GetByOperation(input.Operation)
			if rule == nil {
				t.Fatalf("operation %s is not supported. Use one of: %s", input.Operation, strings.Join(rules.Names(), ", "))
			}
			err = checkScript(rule.GetScript(customization))
			if err != nil {
				t.Fatalf("checking %s of %s, expected nil, but got: %v", rule.Name(), customization.Name, err)
			}
			args := interpreter.RuleArgs{}
			if input.DesiredInputPath != "" {
				args.Desired = getObj(t, dir+"/"+strings.TrimPrefix(input.DesiredInputPath, "/"))
			}
			if input.ObservedInputPath != "" {
				args.Observed = getObj(t, dir+"/"+strings.TrimPrefix(input.ObservedInputPath, "/"))
			}
			if input.StatusInputPath != "" {
				args.Status = getAggregatedStatusItems(t, dir+"/"+strings.TrimPrefix(input.StatusInputPath, "/"))
			}
			result := rule.Run(ipt, args)
			if result.Err != nil {
				t.Fatalf("execute %s %s error: %v\n", customization.Name, rule.Name(), result.Err)
			}
			for _, res := range result.Results {
				expected, ok := input.DesiredResults[res.Name]
				if !ok {
					t.Logf("no expected result for %s", res.Name)
					continue
				}

				if equal, err := deepEqual(expected, res.Value); err != nil || !equal {
					t.Fatal("unexpected result for", res.Name, "expected:", expected, "got:", res.Value, "error:", err)
				}
			}
		}
	}
}

func deepEqual(expected, actualValue interface{}) (bool, error) {
	err := checker.AddFuncs(
		func(a, b resource.Quantity) bool {
			return a.Equal(b)
		})
	if err != nil {
		return false, fmt.Errorf("failed to add custom equality function: %w", err)
	}

	expectedJSONBytes, err := json.Marshal(expected)
	if err != nil {
		return false, fmt.Errorf("failed to marshal expected value: %w", err)
	}

	// Handle known types for semantic comparison
	switch typedActual := actualValue.(type) {
	case *workv1alpha2.ReplicaRequirements:
		var unmarshaledExpected workv1alpha2.ReplicaRequirements
		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
			return false, fmt.Errorf("failed to unmarshal expected JSON into ReplicaRequirements: %w", err)
		}
		return checker.DeepEqual(&unmarshaledExpected, typedActual), nil

	case *configv1alpha1.DependentObjectReference:
		var unmarshaledExpected configv1alpha1.DependentObjectReference
		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
			return false, fmt.Errorf("failed to unmarshal expected JSON into DependentObjectReference: %w", err)
		}
		return checker.DeepEqual(&unmarshaledExpected, typedActual), nil

	case []workv1alpha2.Component:
		var unmarshaledExpected []workv1alpha2.Component
		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
			return false, fmt.Errorf("failed to unmarshal expected JSON into []Component: %w", err)
		}

		return checker.DeepEqual(unmarshaledExpected, typedActual), nil

	case *unstructured.Unstructured:
		var unmarshaledExpected unstructured.Unstructured

		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
			fmt.Println(err)
			return false, fmt.Errorf("failed to unmarshal expected JSON into Unstructured: %w", err)
		}
		return checker.DeepEqual(&unmarshaledExpected, typedActual), nil

	default:
		// Fallback: marshal actualValue and do byte-wise comparison
		actualJSON, err := json.Marshal(actualValue)
		if err != nil {
			return false, fmt.Errorf("failed to marshal actual value: %w", err)
		}
		return bytes.Equal(expectedJSONBytes, actualJSON), nil
	}
}

func TestThirdPartyCustomizationsFile(t *testing.T) {
	err := filepath.Walk("resourcecustomizations", func(path string, f os.FileInfo, err error) error {
		if err != nil {
			// cannot happen
			return err
		}
		if f.IsDir() {
			return nil
		}
		if strings.Contains(path, "testdata") {
			return nil
		}
		if filepath.Base(path) != configurableInterpreterFile {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			// cannot happen
			return err
		}
		var configs []*configv1alpha1.ResourceInterpreterCustomization
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			config := &configv1alpha1.ResourceInterpreterCustomization{}
			err = decoder.Decode(config)
			if err != nil {
				break
			}
			dirSplit := strings.Split(path, string(os.PathSeparator))
			if len(dirSplit) != 5 {
				return fmt.Errorf("the directory format is incorrect. Dir: %s", path)
			}
			if config.Spec.Target.APIVersion != fmt.Sprintf("%s/%s", dirSplit[1], dirSplit[2]) {
				return fmt.Errorf("Target.APIVersion does not match directory format. Target.APIVersion: %s, Dir: %s", config.Spec.Target.APIVersion, path)
			}
			if config.Spec.Target.Kind != dirSplit[3] {
				return fmt.Errorf("Target.Kind does not match directory format. Target.Kind: %s, Dir: %s", config.Spec.Target.Kind, path)
			}
			configs = append(configs, config)
		}
		if err != io.EOF {
			return err
		}
		checkInterpretationRule(t, path, configs)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, but got: %v", err)
	}
}
