package utils

import "testing"

func TestGenExamples(_ *testing.T) {
	GenExamples("/tmp", "kubectl karmada", " register")
}
