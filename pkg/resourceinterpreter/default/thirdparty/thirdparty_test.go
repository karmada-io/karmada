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

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	Name          string                              `yaml:"name"`                    // the name of individual test
	Description   string                              `yaml:"description,omitempty"`   // the description of individual test
	DesiredObj    *unstructured.Unstructured          `yaml:"desiredObj,omitempty"`    // the desired object
	ObservedObj   *unstructured.Unstructured          `yaml:"observedObj,omitempty"`   // the observed object
	StatusItems   []workv1alpha2.AggregatedStatusItem `yaml:"statusItems,omitempty"`   // the status items of aggregated status
	InputReplicas int64                               `yaml:"inputReplicas,omitempty"` // the input replicas for revise operation
	Operation     string                              `yaml:"operation"`               // the operation of resource interpreter
	Filepath      string                              `yaml:"filepath,omitempty"`      // the file path of current test case, used for logging
	// TODO(@zhzhuang-zju): When we have a complete set of test cases, change Output to required field.
	Output map[string]interface{} `yaml:"output,omitempty"` // the expected output results
}

func checkInterpretationRule(t *testing.T, path string, configs []*configv1alpha1.ResourceInterpreterCustomization) {
	ipt := declarative.NewConfigurableInterpreter(nil, 10)
	ipt.LoadConfig(configs)

	dir := filepath.Dir(path)
	testDataDir := filepath.Join(dir, "testdata")

	var err error
	for _, customization := range configs {
		for _, input := range getAllTestCases(t, testDataDir).Tests {
			t.Run(fmt.Sprintf("[%s/%s]:%s", customization.Name, input.Operation, input.Name), func(t *testing.T) {
				rule := rules.GetByOperation(input.Operation)
				if rule == nil {
					t.Fatalf("FilePath: %s. Test case: %s. Operation %s is not supported. Use one of: %s", input.Filepath, input.Name, input.Operation, strings.Join(rules.Names(), ", "))
				}
				err = checkScript(rule.GetScript(customization))
				if err != nil {
					t.Fatalf("FilePath: %s. Test case: %s. Checking %s of %s, expected nil, but got: %v", input.Filepath, input.Name, rule.Name(), customization.Name, err)
				}
				args := buildRuleArgs(input)
				result := rule.Run(ipt, args)
				if result.Err != nil {
					t.Fatalf("FilePath: %s. Test case: %s. Execute %s %s error: %v\n", input.Filepath, input.Name, customization.Name, rule.Name(), result.Err)
				}
				for _, res := range result.Results {
					expected, ok := input.Output[res.Name]
					if !ok {
						// TODO(@zhzhuang-zju): Once we have a complete set of test cases, change this to t.Fatal.
						t.Logf("FilePath: %s. Test case: %s. No expected result for %s of %s\n", input.Filepath, input.Name, res.Name, customization.Name)
						continue
					}

					if equal, err := deepEqual(expected, res.Value); err != nil || !equal {
						t.Fatalf("FilePath: %s. Test case: %s. Unexpected result for %s, expected: %+v, got: %+v, error: %v", input.Filepath, input.Name, res.Name, expected, res.Value, err)
					}
				}
			})
		}
	}
}

func getAllTestCases(t *testing.T, testDataDir string) TestStructure {
	var resourceTest TestStructure
	err := filepath.Walk(testDataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(fmt.Errorf("failed to access path %s: %v", path, err))
		}
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(fmt.Errorf("failed to read file %s: %v", path, err))
		}

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			var test IndividualTest
			err = decoder.Decode(&test)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatal(fmt.Errorf("failed to decode file %s: %v", path, err))
			}
			test.Filepath = path
			resourceTest.Tests = append(resourceTest.Tests, test)
		}
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
	return resourceTest
}

func buildRuleArgs(input IndividualTest) interpreter.RuleArgs {
	return interpreter.RuleArgs{
		Replica:  input.InputReplicas,
		Desired:  input.DesiredObj,
		Observed: input.ObservedObj,
		Status:   input.StatusItems,
	}
}

func deepEqual(expected, actualValue interface{}) (bool, error) {
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

	case []configv1alpha1.DependentObjectReference:
		var unmarshaledExpected []configv1alpha1.DependentObjectReference
		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
			return false, fmt.Errorf("failed to unmarshal expected JSON into []DependentObjectReference: %w", err)
		}
		return checker.DeepEqual(unmarshaledExpected, typedActual), nil

	case []workv1alpha2.Component:
		var unmarshaledExpected []workv1alpha2.Component
		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
			return false, fmt.Errorf("failed to unmarshal expected JSON into []Component: %w", err)
		}

		return checker.DeepEqual(unmarshaledExpected, typedActual), nil

	case *unstructured.Unstructured:
		var unmarshaledExpected unstructured.Unstructured

		if err := json.Unmarshal(expectedJSONBytes, &unmarshaledExpected); err != nil {
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
