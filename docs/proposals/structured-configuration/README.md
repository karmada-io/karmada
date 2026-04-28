---
title: Structured configuration overrider
authors:
- "@Patrick0308"
- "@sophiefeifeifeiya"
reviewers:
- "@chaunceyjiang"
approvers:
- "@chaunceyjiang"
creation-date: 2024-08-12
---

# Structured configuration overrider
## Summary
The proposal introduces a new feature that allows users to partially override values inside  JSON and YAML fields. This is achieved using JSON patch operation. This design enables users to override the values within JSON/YAML fields partially, rather than replacing a whole JSON/YAML fields with `PlaintextOverrider`. Currently, `PlaintextOverrider`  applies JSON patch operations to whole fields, rather than specific values within fields, making it unsuitable for cases where users need to override individual values within those fields.

## Motivation
### Goals
+ Allow users to override specific values inside JSON and YAML in resources (e.g, configmap).
+ Support JSON patch (“add”, “remove”, “replace”) for both JSON and YAML.
### Non-Goals
+ Support all data formats, like XML.
+ Support every operation of YAML and JSON.
## Proposal
### User Stories (Optional)
#### Story 1
As an administrator and developer, I want to update specific values within JSON/YAML in resources to without replacing the entire configuration, ensuring that my changes are minimal and targeted.
### Notes/Constraints/Caveats (Optional)
Illustrated in YAML Implementation.
### Risks and Mitigations
## Design Details
### API Change
```go
type Overriders struct {
    ...
    // FieldOverrider represents the rules dedicated to modifying a specific field in any Kubernetes resource.
    // This allows changing a single field within the resource with multiple operations.
    // It is designed to handle structured field values such as those found in ConfigMaps or Secrets.
    // The current implementation supports JSON and YAML formats, but can easily be extended to support XML in the future.
    // +optional
    FieldOverrider []FieldOverrider `json:"fieldOverrider,omitempty"`
}

type FieldOverrider struct {
    // FieldPath specifies the initial location in the instance document where the operation should take place.
    // The path uses RFC 6901 for navigating into nested structures. For example, the path "/data/db-config.yaml"
    // specifies the configuration data key named "db-config.yaml" in a ConfigMap: "/data/db-config.yaml".
    // +required
    FieldPath string `json:"fieldPath"`

    // JSON represents the operations performed on the JSON document specified by the FieldPath.
    // +optional
    JSON []JSONPatchOperation `json:"json,omitempty"`

    // YAML represents the operations performed on the YAML document specified by the FieldPath.
    // +optional
    YAML []YAMLPatchOperation `json:"yaml,omitempty"`
}

// JSONPatchOperation represents a single field modification operation for JSON format.
type JSONPatchOperation struct {
    // SubPath specifies the relative location within the initial FieldPath where the operation should take place.
    // The path uses RFC 6901 for navigating into nested structures.
    // +required
    SubPath string `json:"subPath"`

    // Operator indicates the operation on target field.
    // Available operators are: "add", "remove", and "replace".
    // +kubebuilder:validation:Enum=add;remove;replace
    // +required
    Operator OverriderOperator `json:"operator"`

    // Value is the new value to set for the specified field if the operation is "add" or "replace".
    // For "remove" operation, this field is ignored.
    // +optional
    Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// YAMLPatchOperation represents a single field modification operation for YAML format.
type YAMLPatchOperation struct {
    // SubPath specifies the relative location within the initial FieldPath where the operation should take place.
    // The path uses RFC 6901 for navigating into nested structures.
    // +required
    SubPath string `json:"subPath"`

    // Operator indicates the operation on target field.
    // Available operators are: "add", "remove", and "replace".
    // +kubebuilder:validation:Enum=add;remove;replace
    // +required
    Operator OverriderOperator `json:"operator"`

    // Value is the new value to set for the specified field if the operation is "add" or "replace".
    // For "remove" operation, this field is ignored.
    // +optional
    Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// OverriderOperator is the set of operators that can be used in an overrider.
type OverriderOperator string

// These are valid overrider operators.
const (
    OverriderOpAdd     OverriderOperator = "add"
    OverriderOpRemove  OverriderOperator = "remove"
    OverriderOpReplace OverriderOperator = "replace"
)
```

### User usage example
For example, consider a ConfigMap with the following data in member cluster:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-config
data:
  db-config.yaml: |
    database:
      host: localhost
      port: 3306
```
The following is OverridePolicy which uses FieldOverrider:
```yaml
apiVersion: karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example-configmap-override
spec:
  resourceSelectors:
    - apiVersion: v1
      kind: ConfigMap
      name: example-config
  overrideRules:
    - overriders:
        fieldOverrider:
          - fieldPath: /data/db-config.yaml
            yaml:
              - subPath: /database/host
                operator: replace
                value: "remote-db.example.com"
              - subPath: /database/port
                operator: replace
                value: "3307"
```
After we apply this policy, we can modify the db-config.yaml
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-config
data:
  db-config.yaml: |
    database:
      host: remote-db.example.com
      port: 3307
```

### YAML Implementation

We choose Plan1.
(1) Plan1 is easy to implement by converting YAML directly to JSON, while Plan2 has to encapsulate ytt's grammar as if it is JSON patch.
(2) Plan1 and Plan2 both have the same issues illustrated in the following.

#### Plan 1: directly convert to and back from JSON (chosen)
If YAML is directly converted to JSON and then converted back using (‘sigs.k8s.io/yaml’), some data type information might be lost.
1. Dates and Times
   ```yaml
   dob: 1979-05-27T07:32:00Z
   date_of_birth: 1979-05-27
   
   dob: time.Time
   date_of_birth: time.Time
   
   # after transformation
   
   dob: string
   date_of_birth: string
   ```
2. Supports anchors (&) and aliases (*) to reference and reuse values
   ```yaml
   default: &default
     name: Alice
     age: 30
   employee1:
     <<: *default
     role: Developer
   employee2:
     <<: *default
     role: Designer
   
   # after transformation
   
   default:
     age: 30
     name: Alice
   employee1:
     age: 30
     name: Alice
     role: Developer
   employee2:
     age: 30
     name: Alice
     role: Designer
   ```
   **Can be applied if:**
   (1) Do not consider the situation described above
   (2) Write cases to deal with different types **(large maintenance costs)**

#### Plan 2: ytt (Aborted)
Use the third party libraries **ytt**, supporting the overlays https://carvel.dev/ytt/docs/v0.50.x/ytt-overlays/ to implement and json operation similar operations. Implementation has its own specific syntax for ytt.
Documents: https://carvel.dev/ytt/docs/v0.50.x/ytt-overlays/
Joining methods: https://github.com/carvel-dev/ytt/blob/develop/examples/integrating-with-ytt/apis.md#as-a-go-module
After testing, it has the same problem as the **Plan 1**.
1. Dates and Times
```yaml
dob: 1979-05-27T07:32:00Z
date_of_birth: 1979-05-27

# after transformation

dob: "1979-05-27T07:32:00Z"
date_of_birth: "1979-05-27"
```

2. Supports anchors (&) and aliases (*) to reference and reuse values
```yaml
default: &default
  name: Alice
  age: 30
employee1:
  <<: *default
  role: Developer
employee2:
  <<: *default
  role: Designer

# after transformation

default:
  name: Alice
  age: 30
employee1:
  name: Alice
  age: 30
  role: Developer
employee2:
  name: Alice
  age: 30
  role: Designer
number: 12345
string: "6789"
```
**Can be applied if:**
(1) Do not consider the situation described above
(2) Ability to maintain functionality despite incompatibilities arising from encapsulating ytt as JSON operations later on.

### Test Plan
#### UT
- Add unit tests to cover the new functions.
#### E2E
+ Write proposal in `coverage_docs/overridepolicy_test.md` : deployment `FieldOverrider` testing;
+ Use ginkgo to complete the code `overridepolicy_test.go`.

## Alternatives
There are three API designs to achieve this:
### (1) Each data format has different name with same struct
```go
type Overriders struct {
    ...
    // JSONPlaintextOverrider represents the rules dedicated to handling json object overrides
    // +optional
    JSONPlaintextOverrider []JSONPlaintextOverrider `json:"jsonPlaintextOverrider,omitempty"`
    // YAMLPlaintextOverrider represents the rules dedicated to handling yaml object overrides
    // +optional
    YAMLPlaintextOverrider []YAMLPlaintextOverrider `json:"yamlPlaintextOverrider,omitempty"`
}

type JSONPlaintextOverrider struct {
    // Path indicates the path of target field
    Path string `json:"path"`
    // JSONPatch represents json patch rules defined with plaintext overriders.
    Patch []Patch `json:"patch"`
    // MergeValue t represents the object value to be merged into the object.
    MergeValue apiextensionsv1.JSON `json:"mergeValue"`
    // MergeRawValue represents the raw, original format data (e.g., YAML, JSON) to be merged into the object.
    MergeRawValue string `json:"mergeRawValue"`
}

type YAMLPlaintextOverrider struct {
    // Path indicates the path of target field
    Path string `json:"path"`
    // JSONPatch represents json patch rules defined with plaintext overriders.
    Patch []Patch `json:"patch"`
    // MergeValue t represents the object value to be merged into the object.
    MergeValue apiextensionsv1.JSON `json:"mergeValue"`
    // MergeRawValue represents the raw, original format data (e.g., YAML, JSON) to be merged into the object.
    MergeRawValue string `json:"mergeRawValue"`
}

type Patch struct {
    // Path indicates the path of target field
    Path string `json:"path"`
    // From indicates the path of original field when operator is move and copy
    From string `json:"from"`
    // Operator indicates the operation on target field.
    // Available operators are: add, replace and remove.
    // +kubebuilder:validation:Enum=add;remove;replace;move;test;copy
    Operator PatchOperator `json:"operator"`
    // Value to be applied to target field.
    // Must be empty when operator is Remove.
    // +optional
    Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// PatchOperator is the set of operators that can be used in an overrider.
type PatchOperator string

// These are valid patch operators.
const (
    PatchOpAdd     PatchOperator = "add"
    PatchOpRemove  PatchOperator = "remove"
    PatchOpReplace  PatchOperator = "replace"
    PatchOpMove  PatchOperator = "move"
    PatchOpTest  PatchOperator = "test"
    PatchOpCopy  PatchOperator = "copy"
)
```

### (2) Each data format has different name with same struct

```go
type Overriders struct {
    ...
    // JSONPlaintextOverrider represents the rules dedicated to handling json object overrides
    // +optional
    JSONPlaintextOverrider []PlaintextObjectOverrider `json:"jsonPlaintextOverrider,omitempty"`
    // YAMLPlaintextOverrider represents the rules dedicated to handling yaml object overrides
    // +optional
    YAMLPlaintextOverrider []PlaintextObjectOverrider `json:"yamlPlaintextOverrider,omitempty"`
    // TOMLPlaintextOverrider represents the rules dedicated to handling toml object overrides
    // +optional
    TOMLPlaintextOverrider []PlaintextObjectOverrider `json:"tomlPlaintextOverrider,omitempty"`
    // XMLPlaintextOverrider represents the rules dedicated to handling xml object overrides
    // +optional
    XMLPlaintextOverrider []PlaintextObjectOverrider `json:"xmlPlaintextOverrider,omitempty"`
}

type PlaintextObjectOverrider struct {
    // Path indicates the path of target field
    Path string `json:"path"`
    // JSONPatch represents json patch rules defined with plaintext overriders.
    JSONPatch []JSONPatch `json:"jsonPatch"`
    // MergeValue t represents the object value to be merged into the object.
    MergeValue apiextensionsv1.JSON `json:"mergeValue"`
    // MergeRawValue represents the raw, original format data (e.g., YAML, JSON) to be merged into the object.
    MergeRawValue string `json:"mergeRawValue"`
}

type JSONPatch  struct {
    // Path indicates the path of target field
    Path string `json:"path"`
    // From indicates the path of original field when operator is move and copy
    From string `json:"from"`
    // Operator indicates the operation on target field.
    // Available operators are: add, replace and remove.
    // +kubebuilder:validation:Enum=add;remove;replace;move;test;copy
    Operator JSONPatchOperator `json:"operator"`
    // Value to be applied to target field.
    // Must be empty when operator is Remove.
    // +optional
    Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// PatchOperator is the set of operators that can be used in an overrider.
type JSONPatchOperator string

// These are valid patch operators.
const (
    PatchOpAdd     JSONPatchOperator = "add"
    PatchOpRemove  JSONPatchOperator = "remove"
    PatchOpReplace  JSONPatchOperator = "replace"
    PatchOpMove  JSONPatchOperator = "move"
    PatchOpTest  JSONPatchOperator = "test"
    PatchOpCopy  JSONPatchOperator = "copy"
)
```

### (3) Enumeration

```go
type Overriders struct {
    ...
    // PlaintextObjectOverrider represents the rules dedicated to handling yaml  object overrides
    // +optional
    PlaintextObjectOverrider []PlaintextObjectOverrider `json:"yamlPlaintextOverrider,omitempty"`
}

type PlaintextObjectOverrider struct {
    // Path indicates the path of target field
    Path string `json:"path"`
    // DataFormat indicates the type of data formats type to be modified
    DataFormat DataFormat `json:"dataFormat"`
    // JSONPatch represents json patch rules defined with plaintext overriders.
    JSONPatch []JSONPatch `json:"jsonPatch"`
    // MergeValue t represents the object value to be merged into the object.
    MergeValue apiextensionsv1.JSON `json:"mergeValue"`
    // MergeRawValue represents the raw, original format data (e.g., YAML, JSON) to be merged into the object.
    MergeRawValue string `json:"mergeRawValue"`
}

type DataFormat string

const (
    yaml DataFormat = "yaml"
    json DataFormat = "json"
    toml DataFormat = "toml"
)
```

### Analysis of 3 Implementations

(1) Each data format has different struct (**Easiest to extend**)
json -> json * op -> json -> JSONPlaintextOverrider []JSONPlaintextOverrider
yaml -> yaml * op -> yaml -> YAMLPlaintextOverrider []YAMLPlaintextOverrider
xml -> xml * op -> xml -> XMLPlaintextOverrider []XMLPlaintextOverrider
...
This one is designed for **native operations/JSON operations** for each data format.
For example, json has 5 json operations, yaml has 3 yaml operations, and xml has 4 xml operations, ...
It should be 5+3+4+.. operations in total, which is a huge number.

(2) Each data format has different name with same struct
json -> json * op -> json -> JSONPlaintextOverrider []PlaintextObjectOverrider
yaml -> yaml * op -> yaml -> YAMLPlaintextOverrider []PlaintextObjectOverrider
xml -> xml * op -> xml -> XMLPlaintextOverrider []PlaintextObjectOverrider
...
This one is designed for **JSON operations** for all data formats.
(3) enum
json -> json * op -> json -> enum
yaml -> json * op -> yaml -> enum
xml -> json * op -> xml -> enum
...
This one is designed for **JSON operations** for all data formats.

### User usage example for (1)
For example, consider a ConfigMap with the following data in member cluster:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-configmap
  namespace: default
data:
  config.json: |
    {
      "keyA": "valueA",
      "keyB": "valueB",
      "keyC": "valueC",
      "keyD": "valueD",
      "keyE": "valueE",
      "keyF": "valueF"
    }
```

The following is OverridePolicy which uses JSONPlaintextOverrider:

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: example-override
  namespace: default
spec:
  targetCluster:
    clusterNames:
      - member1
  resourceSelectors:
    - apiVersion: v1
      kind: ConfigMap
      name: example-configmap
      namespace: default
  overrideRules:
    - overriders:
        jsonPlaintextOverrider:
          - path: /data/config.json
            patch:
              - path: /keyA
                operator: test
                value: "valueA"
              - path: /keyD
                operator: add
                value: ""
              - path: /keyB
                operator: remove
              - path: /keyC
                operator: replace
                value: "newly added value"
              - from: /keyD
                path: /keyF
                operator: move
              - from: /keyE
                path: /keyG
                operator: copy
            mergeValue:
              {
                "keyH": "valueH",
                "keyI": "valueI"
              }
            mergeRawValue: '{"keyJ": "valueJ","keyK": "valueK"}'
```

After we apply this policy, we can modify the config.json

1. Test: The operation checks if keyA has the value `valueA`. Since it does, the operation proceeds.
2. Add: Adds an empty string as the value for keyD.
3. Remove: Removes keyB and its value.
4. Replace: Replaces the value of keyC with `newly added value`.
5. Move: Moves the value of keyD to keyF, which effectively deletes keyD and sets the value of keyF to `valueD`.
6. Copy: Copies the value of keyE to keyG.
7. Merge: Adds new keys keyH and keyI with values `valueH` and `valueI`.
8. Merge Raw Value: Adds keys keyJ and keyK with values `valueJ` and `valueK`.
   Finally, we get a new config.json with JSON operation.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example-configmap
  namespace: default
data:
  config.json: |
    {
      "keyA": "valueA",
      "keyC": "newly added value",
      "keyE": "valueE",
      "keyF": "valueD",
      "keyG": "valueE",
      "keyH": "valueH",
      "keyI": "valueI",
      "keyJ": "valueJ",
      "keyK": "valueK"
    }
```