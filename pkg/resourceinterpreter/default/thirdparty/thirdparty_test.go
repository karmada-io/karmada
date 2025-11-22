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

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/luavm"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

var rules interpreter.Rules = interpreter.AllResourceInterpreterCustomizationRules

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
	jsonData, err := k8syaml.ToJSON(data)
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
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
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

func getExpectedOutput(t *testing.T, path string) *ExpectedOutput {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var expected ExpectedOutput
	err = k8syaml.Unmarshal(data, &expected)
	if err != nil {
		t.Fatal(err)
	}
	return &expected
}

type TestStructure struct {
	Tests []IndividualTest `yaml:"tests"`
}

type IndividualTest struct {
	DesiredInputPath   string          `yaml:"desiredInputPath,omitempty"`
	ObservedInputPath  string          `yaml:"observedInputPath,omitempty"`
	StatusInputPath    string          `yaml:"statusInputPath,omitempty"`
	DesiredReplica     int64           `yaml:"desiredReplicas,omitempty"`
	Operation          string          `yaml:"operation"`
	ExpectedOutputPath string          `yaml:"expectedOutputPath,omitempty"`
	ExpectedOutput     *ExpectedOutput `yaml:"expectedOutput,omitempty"`
}

type ExpectedOutput struct {
	// For AggregateStatus operation
	AggregatedStatus map[string]interface{} `yaml:"aggregatedStatus,omitempty"`
	// For InterpretDependency operation
	Dependencies []map[string]interface{} `yaml:"dependencies,omitempty"`
	// For Retain operation
	Retained map[string]interface{} `yaml:"retained,omitempty"`
	// For InterpretReplica operation (returns replica and requires)
	Replica  *int32                 `yaml:"replica,omitempty"`
	Requires map[string]interface{} `yaml:"requires,omitempty"`
	// For ReviseReplica operation
	Revised map[string]interface{} `yaml:"revised,omitempty"`
	// For InterpretStatus operation
	Status map[string]interface{} `yaml:"status,omitempty"`
	// For InterpretHealth operation
	Healthy *bool `yaml:"healthy,omitempty"`
	// For InterpretComponent operation
	Components []map[string]interface{} `yaml:"components,omitempty"`
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
	err = k8syaml.Unmarshal(yamlBytes, &resourceTest)
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
			args := interpreter.RuleArgs{Replica: input.DesiredReplica}
			if input.DesiredInputPath != "" {
				args.Desired = getObj(t, dir+"/"+strings.TrimPrefix(input.DesiredInputPath, "/"))
			}
			if input.ObservedInputPath != "" {
				args.Observed = getObj(t, dir+"/"+strings.TrimPrefix(input.ObservedInputPath, "/"))
			}
			if input.StatusInputPath != "" {
				args.Status = getAggregatedStatusItems(t, dir+"/"+strings.TrimPrefix(input.StatusInputPath, "/"))
			}

			// Load expected output from file if specified
			var expectedOutput *ExpectedOutput
			if input.ExpectedOutputPath != "" {
				expectedOutput = getExpectedOutput(t, dir+"/"+strings.TrimPrefix(input.ExpectedOutputPath, "/"))
			}

			result := rule.Run(ipt, args)
			// Verify the result if expected output is provided
			verifyRuleResult(t, customization.Name, rule.Name(), result, expectedOutput)
		}
	}
}

// verifyRuleResult verifies that the rule execution result matches the expected output.
//
// Verification Strategy:
//   - If expected is nil: Skip verification entirely (allows tests without expected outputs)
//   - If expected is not nil but a specific field is nil: Skip that field's verification (flexible testing)
//   - If expected field is defined: Strictly verify and fail if mismatch
//
// Future Enhancement Plan:
//
//	Once all third-party resource tests have complete expected outputs defined, we should
//	switch to strict mode where having an actual output without a corresponding expected
//	field will cause the test to fail. This ensures complete test coverage.
//
//	To enable strict mode in the future:
//	  1. Ensure all test cases have expectedOutputPath or expectedOutput defined
//	  2. Uncomment the t.Fatalf lines in each case below
//	  3. This will catch any missing expected output definitions
func verifyRuleResult(t *testing.T, customizationName string, operation string, result *interpreter.RuleResult, expected *ExpectedOutput) {
	if expected == nil {
		// Skip verification if no expected output is provided
		// This allows gradual test enhancement
		return
	}

	if result.Err != nil {
		t.Fatalf("execute %s %s error: %v", customizationName, operation, result.Err)
		return
	}

	if len(result.Results) == 0 {
		t.Fatalf("execute %s %s returned no results", customizationName, operation)
		return
	}

	// Build a map of actual results by name for easy lookup
	actualResults := make(map[string]interface{})
	for _, res := range result.Results {
		actualResults[res.Name] = res.Value
	}

	// Verify each result based on its name
	// NOTE: Currently we skip verification if the expected field is not defined (nil check + continue).
	// This allows gradual test enhancement - tests can be added without expected outputs first.
	// TODO: Once all tests have complete expected outputs, change these to strict checks:
	//       if expected.Field == nil { t.Fatalf("missing expected output for %s", resultName) }
	//       This will ensure no output is left unverified.
	for resultName, resultValue := range actualResults {
		verifyResultByName(t, customizationName, resultName, resultValue, expected)
	}
}

// verifyResultByName verifies a single result based on its name
func verifyResultByName(t *testing.T, customizationName string, resultName string, resultValue interface{}, expected *ExpectedOutput) {
	switch resultName {
	case "aggregatedStatus":
		verifyOptionalJSONField(t, customizationName, "AggregateStatus", resultValue, expected.AggregatedStatus)
	case "dependencies":
		verifyOptionalJSONField(t, customizationName, "InterpretDependency", resultValue, expected.Dependencies)
	case "retained":
		verifyOptionalJSONField(t, customizationName, "Retain", resultValue, expected.Retained)
	case "replica":
		verifyReplicaField(t, customizationName, resultValue, expected.Replica)
	case "requires":
		verifyOptionalJSONField(t, customizationName, "InterpretReplica 'requires'", resultValue, expected.Requires)
	case "revised":
		verifyOptionalJSONField(t, customizationName, "ReviseReplica", resultValue, expected.Revised)
	case "status":
		verifyOptionalJSONField(t, customizationName, "InterpretStatus", resultValue, expected.Status)
	case "healthy":
		verifyHealthyField(t, customizationName, resultValue, expected.Healthy)
	case "components":
		verifyOptionalJSONField(t, customizationName, "InterpretComponent", resultValue, expected.Components)
	default:
		t.Logf("Unknown result name: %s, skipping verification", resultName)
	}
}

// verifyOptionalJSONField verifies a field if the expected value is provided, otherwise skips
func verifyOptionalJSONField(t *testing.T, customizationName string, operation string, actual interface{}, expected interface{}) {
	if expected == nil {
		// TODO: Change to strict check when all tests have expected outputs
		// t.Fatalf("missing expected '%s' output for %s", operation, customizationName)
		return
	}
	verifyJSONMatch(t, customizationName, operation, actual, expected)
}

// verifyReplicaField verifies the replica field (int32)
func verifyReplicaField(t *testing.T, customizationName string, resultValue interface{}, expectedReplica *int32) {
	if expectedReplica == nil {
		// TODO: Change to strict check when all tests have expected outputs
		// t.Fatalf("missing expected 'replica' for %s", customizationName)
		return
	}

	actualReplica, ok := resultValue.(int32)
	if !ok {
		// Try to convert from float64 (JSON unmarshal default for numbers)
		if f, ok := resultValue.(float64); ok {
			actualReplica = int32(f)
		} else {
			t.Fatalf("InterpretReplica 'replica' result is not int32 for %s, got %T", customizationName, resultValue)
		}
	}

	if actualReplica != *expectedReplica {
		t.Fatalf("InterpretReplica 'replica' result mismatch for %s: expected %d, got %d",
			customizationName, *expectedReplica, actualReplica)
	}
	t.Logf("✓ InterpretReplica 'replica' verification passed for %s (value: %d)", customizationName, actualReplica)
}

// verifyHealthyField verifies the healthy field (bool)
func verifyHealthyField(t *testing.T, customizationName string, resultValue interface{}, expectedHealthy *bool) {
	if expectedHealthy == nil {
		// TODO: Change to strict check when all tests have expected outputs
		// t.Fatalf("missing expected 'healthy' for %s", customizationName)
		return
	}

	actualHealthy, ok := resultValue.(bool)
	if !ok {
		t.Fatalf("InterpretHealth result is not bool for %s, got %T", customizationName, resultValue)
	}

	if actualHealthy != *expectedHealthy {
		t.Fatalf("InterpretHealth result mismatch for %s: expected %v, got %v",
			customizationName, *expectedHealthy, actualHealthy)
	}
	t.Logf("✓ InterpretHealth verification passed for %s (value: %v)", customizationName, actualHealthy)
}

// verifyJSONMatch compares actual and expected values via JSON marshaling
func verifyJSONMatch(t *testing.T, customizationName string, operation string, actual interface{}, expected interface{}) {
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("failed to marshal actual result for %s: %v", operation, err)
	}

	var actualData interface{}
	if err := json.Unmarshal(actualJSON, &actualData); err != nil {
		t.Fatalf("failed to unmarshal actual result for %s: %v", operation, err)
	}

	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("failed to marshal expected result for %s: %v", operation, err)
	}
	var expectedData interface{}
	if err := json.Unmarshal(expectedJSON, &expectedData); err != nil {
		t.Fatalf("failed to unmarshal expected result for %s: %v", operation, err)
	}

	if !compareJSON(actualData, expectedData) {
		t.Fatalf("%s result mismatch for %s\nExpected:\n%s\nActual:\n%s",
			operation, customizationName, prettyJSON(expectedData), prettyJSON(actualData))
	}
	t.Logf("✓ %s verification passed for %s", operation, customizationName)
}

func compareJSON(a, b interface{}) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

func prettyJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	// Pretty format by adding line breaks
	var buf bytes.Buffer
	if err := yaml.Unmarshal(data, &buf); err == nil {
		return buf.String()
	}
	return string(data)
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
		decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
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
