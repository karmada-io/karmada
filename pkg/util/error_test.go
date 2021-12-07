package util

import (
	"fmt"
	"reflect"
	"testing"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestAggregateErrors(t *testing.T) {
	err1 := fmt.Errorf("error1")
	err2 := fmt.Errorf("error2")
	channel := make(chan error, 2)
	channel <- err1
	channel <- err2

	tests := []struct {
		name     string
		input    <-chan error
		expected error
	}{
		{
			name:     "nil channel",
			input:    nil,
			expected: nil,
		},
		{
			name:     "channel has no error",
			input:    make(chan error, 1),
			expected: nil,
		},
		{
			name:     "channel has 2 errors",
			input:    channel,
			expected: utilerrors.NewAggregate([]error{err1, err2}),
		},
	}

	for _, test := range tests {
		if got := AggregateErrors(test.input); !reflect.DeepEqual(got, test.expected) {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, got)
		}
	}
}
