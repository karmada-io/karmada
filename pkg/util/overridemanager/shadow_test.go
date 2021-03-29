package overridemanager

import (
	"testing"
)

func TestAppliedOverrides_AscendOrder(t *testing.T) {
	applied := AppliedOverrides{}
	item2 := OverridePolicyShadow{PolicyName: "bbb"}
	item1 := OverridePolicyShadow{PolicyName: "aaa"}
	item3 := OverridePolicyShadow{PolicyName: "ccc"}

	applied.Add(item1.PolicyName, item1.Overriders)
	applied.Add(item2.PolicyName, item2.Overriders)
	applied.Add(item3.PolicyName, item3.Overriders)

	appliedBytes, err := applied.MarshalJSON()
	if err != nil {
		t.Fatalf("not expect error, but got: %v", err)
	}

	expectJSON := `[{"policyName":"aaa","overriders":{}},{"policyName":"bbb","overriders":{}},{"policyName":"ccc","overriders":{}}]`
	if string(appliedBytes) != expectJSON {
		t.Fatalf("expect %s, but got: %s", expectJSON, string(appliedBytes))
	}
}
