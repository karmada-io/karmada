/*
Copyright 2021 The Karmada Authors.

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

package overridemanager

import (
	"testing"
)

func TestAppliedOverrides_AscendOrder(t *testing.T) {
	applied := AppliedOverrides{}
	appliedEmptyBytes, er := applied.MarshalJSON()
	if er != nil {
		t.Fatalf("not expect error, but got: %v", er)
	}
	if appliedEmptyBytes != nil {
		t.Fatalf("expect nil, but got: %s", string(appliedEmptyBytes))
	}

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
