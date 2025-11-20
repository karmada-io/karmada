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
│           ├── customizations.yaml          # Resource interpreter customization rules
│           ├── customizations_tests.yaml    # Test cases for the customizations
│           └── testdata/                    # Test input and expected output files
│               ├── desired_xxx.yaml      # Input resource for desired state
│               ├── observed_xxx.yaml     # Input resource for observed state  
│               ├── status_xxx.yaml       # Input aggregated status items
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

Test data files are generally divided into three categories:

- `desired_xxx.yaml`: Resource definitions deployed on the control plane
- `observed_xxx.yaml`: Resource definitions observed in a member cluster
- `status_xxx.yaml`: Status information of the resource on each member cluster, with structure `[]workv1alpha2.AggregatedStatusItem`

Multiple test data files can be created for each category as needed. Pay attention to naming distinctions, as they will ultimately be referenced in `customizations_tests.yaml`.

#### 3. Create Test Configuration

Create `customizations_tests.yaml` to define test cases:

```yaml
tests:
  - observedInputPath: testdata/observed-flinkdeployment.yaml
    operation: InterpretReplica
    desiredResults:
      replica: 2
```

Where:
- `operation` specifies the operation of resource interpreter
- `desiredResults` defines the expected output, which is a key-value mapping where the key is the field name of the expected result and the value is the expected result

The keys in `desiredResults` for different operations correspond to the Name field of the results returned by the corresponding resource interpreter operation `RuleResult.Results`.

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

The `desiredResults` for operation `InterpretHealth` should contain the `healthy` key.

### Supported Operations

The test framework supports the following operations:

- `InterpretReplica` - Extract replica count from resource
- `InterpretComponent` -  Extract component information from resource
- `ReviseReplica` - Modify replica count in resource
- `InterpretStatus` - Extract status information
- `InterpretHealth` - Determine resource health status
- `InterpretDependency` - Extract resource dependencies
- `AggregateStatus` - Aggregate status from multiple clusters
- `Retain` - Retain the desired resource template.

### Test Validation

The test framework validates:

1. **Lua Script Syntax** - Ensures all Lua scripts are syntactically correct
2. **Execution Results** - Compares actual results with expected results

### Debugging Tests

To debug failing tests:

1. **Check Lua Script Syntax** - Ensure your Lua scripts are valid
2. **Verify Test Data** - Confirm test input files are properly formatted
3. **Review Expected Results** - Make sure expected results match the actual operation output
4. **Use Verbose Output** - Run tests with `-v` flag for detailed output

### Best Practices

1. **Comprehensive Coverage** - Test all supported operations for your resource type
2. **Edge Cases** - Include tests for edge cases and error conditions  
3. **Realistic Data** - Use realistic resource definitions in test data
4. **Clear Naming** - Use descriptive names for test files and cases

For more information about resource interpreter customizations, see the [Karmada documentation](https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter/).