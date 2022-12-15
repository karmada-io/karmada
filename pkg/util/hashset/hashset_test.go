package hashset

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func ExampleSet_List() {
	s := Make[int]() // make a set for int
	s.Insert(1, 3, 2)
	s.Insert(4)
	l := s.List()
	sort.Slice(l, func(i, j int) bool {
		return l[i] < l[j]
	})
	fmt.Println(l)
	// Output:
	// [1 2 3 4]
}

func TestSet_List_BuiltinType(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "input nothing should list nothing",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "should guarantee no repeated items",
			input:    []string{"Kevin", "Jim", "Kevin", "Kevin"}, // repeat "Kevin"
			expected: []string{"Jim", "Kevin"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set := Make[string]()
			set.Insert(test.input...)
			list := set.List()
			sort.Slice(list, func(i, j int) bool {
				return list[i] < list[j]
			})
			if !reflect.DeepEqual(list, test.expected) {
				t.Fatalf("expected: %v, but got: %v", test.expected, list)
			}
		})
	}
}

func TestSet_List_CustomType(t *testing.T) {
	type CustomType struct {
		Kind string
		Name string
	}

	tests := []struct {
		name     string
		input    []CustomType
		expected []CustomType
	}{
		{
			name:     "input nothing should list nothing",
			input:    []CustomType{},
			expected: []CustomType{},
		},
		{
			name: "should guarantee no repeated items",
			input: []CustomType{ // repeat {Kind: "k1", Name: "n1"}
				{Kind: "k1", Name: "n1"},
				{Kind: "k2", Name: "n2"},
				{Kind: "k1", Name: "n1"},
			},
			expected: []CustomType{{Kind: "k1", Name: "n1"}, {Kind: "k2", Name: "n2"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set := Make[CustomType]()
			set.Insert(test.input...)
			list := set.List()
			sort.Slice(list, func(i, j int) bool {
				if list[i].Kind != list[j].Kind {
					return list[i].Kind < list[j].Kind
				}
				return list[i].Name < list[j].Name
			})
			if !reflect.DeepEqual(list, test.expected) {
				t.Fatalf("expected: %v, but got: %v", test.expected, list)
			}
		})
	}
}
