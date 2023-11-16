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

package cmdinit

import (
	"testing"
)

func TestNewCmdInit(t *testing.T) {
	cmdinit := NewCmdInit("karmadactl")
	if cmdinit == nil {
		t.Errorf("NewCmdInit() want return not nil, but return nil")
	}
}

func Test_initExample(t *testing.T) {
	cmdinitexample := NewCmdInit("karmadactl")
	if cmdinitexample == nil {
		t.Errorf("initExample() want return not nil, but return nil")
	}
}
