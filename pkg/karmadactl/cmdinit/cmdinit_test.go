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
