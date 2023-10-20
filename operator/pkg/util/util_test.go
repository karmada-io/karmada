package util

import (
	"strings"
	"testing"
)

func TestListFileWithSuffix(t *testing.T) {
	suffix := ".yaml"
	files := ListFileWithSuffix(".", suffix)
	want := 2
	got := len(files)
	if want != got {
		t.Errorf("Expected %d ,but got :%d", want, got)
	}
	for _, f := range files {
		if !strings.HasSuffix(f.AbsPath, suffix) {
			t.Errorf("Want suffix with %s , but not exist, path is:%s", suffix, f.AbsPath)
		}
	}
}
