# Thirdparty Resource Interpreter

This directory contains third-party resource interpreters for Karmada. These interpreters define how Karmada should handle custom resources from various third-party applications and operators.

## Files

- `thirdparty.go` - Main implementation of the third-party resource interpreter
- `thirdparty_test.go` - Test suite for validating resource interpreter customizations
- `resourcecustomizations/` - Directory containing resource customization definitions organized by API version and kind

## Directory Structure

The resource customizations are organized in the following structure:

```
resourcecustomizations/
├── <group>/
│   └── <version>/
│       └── <kind>/
│           ├── customizations.yaml                # Resource interpreter customization rules
│           └── testdata/                          # Test input and expected output files
│               ├── aggregatestatus-test.yaml      # test case for AggregateStatus operation
│               ├── interpretcomponent-test.yaml   # test case for InterpretComponent operation
│               ├── interprethealth-test.yaml      # test case for InterpretHealth operation
│               ├── ...
```

## How to test

### Running Tests

To run all third-party resource interpreter tests:

```bash
cd pkg/resourceinterpreter/default/thirdparty
go test -v
```

### Creating Test Cases

#### 1. Create Test Structure

For a new resource type, create the directory structure:

```bash
mkdir -p resourcecustomizations/<group>/<version>/<kind>/testdata
```

#### 2. Create Test Data Files

Test data files are divided by operation type. Each file contains different test cases for the corresponding operation. The naming convention for test data files is `<operation>-test.yaml`.

For example:
```markdown
aggregatestatus-test.yaml        # test case for AggregateStatus operation
interpretcomponent-test.yaml     # test case for InterpretComponent operation
interprethealth-test.yaml        # test case for InterpretHealth operation
```

#### 3. Add Test Cases

The test case structure is as follows:
```go
type IndividualTest struct {
	Name          string                              `yaml:"name"`                    // the name of individual test
	Description   string                              `yaml:"description,omitempty"`   // the description of individual test
	DesiredObj    *unstructured.Unstructured          `yaml:"desiredObj,omitempty"`    // the desired object
	ObservedObj   *unstructured.Unstructured          `yaml:"observedObj,omitempty"`   // the observed object
	StatusItems   []workv1alpha2.AggregatedStatusItem `yaml:"statusItems,omitempty"`   // the status items of aggregated status
	InputReplicas int64                               `yaml:"inputReplicas,omitempty"` // the input replicas for revise operation
	Operation     string                              `yaml:"operation"`               // the operation of resource interpreter
	Output map[string]interface{}                     `yaml:"output,omitempty"` // the expected output results
}
```

Where:
- `Output` The output are key-value mapping where the key is the field name of the expected result and the value is the expected result. The keys in output for different operations correspond to the Name field of the results returned by the corresponding resource interpreter operation `RuleResult.Results`.

For example:
```go
func (h *healthInterpretationRule) Run(interpreter *declarative.ConfigurableInterpreter, args RuleArgs) *RuleResult {
	obj, err := args.getObjectOrError()
	if err != nil {
		return newRuleResultWithError(err)
	}
	healthy, enabled, err := interpreter.InterpretHealth(obj)
	if err != nil {
		return newRuleResultWithError(err)
	}
	if !enabled {
		return newRuleResultWithError(fmt.Errorf("rule is not enabled"))
	}
	return newRuleResult().add("healthy", healthy)
}
```

The output for operation `InterpretHealth` should contain the `healthy` key.

For more examples of test cases, refer to the existing test data files in this directory.

### Supported Operations

The test framework supports the following operations:

- `InterpretReplica` - Extract replica count from resource
- `InterpretComponent` - Extract component information from resource
- `ReviseReplica` - Modify replica count in resource
- `InterpretStatus` - Extract status information
- `InterpretHealth` - Determine resource health status
- `InterpretDependency` - Extract resource dependencies
- `AggregateStatus` - Aggregate status from multiple clusters
- `Retain` - Retain the desired resource template.

> NOTE: Not all operations need to be implemented for every resource type. Implement only the operations relevant to your resource.

For more information about resource interpreter customizations, see the [Karmada documentation](https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter/).
