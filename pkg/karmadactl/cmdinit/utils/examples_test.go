package utils

import "testing"

func TestGenExamples(t *testing.T) {
	GenExamples("/tmp", "kubectl karmada", " register")
}
