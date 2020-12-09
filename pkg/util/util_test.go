package util

import (
	"reflect"
	"testing"
)

func TestGetDifferenceSetWhenIncludeItemAndExcludeItemIsNull(t *testing.T) {
	testName := "IncludeItemAndExcludeItemIsNull"
	want := []string{}
	t.Run(testName, func(t *testing.T) {
		if got := GetDifferenceSet(nil, nil); !reflect.DeepEqual(got, want) {
			t.Errorf("GetDifferenceSet() = %v, want %v", got, want)
		}
	})
}

func TestGetDifferenceSetWhenIncludeItemOrExcludeItemIsNull(t *testing.T) {
	type args struct {
		includeItems []string
		excludeItems []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "TestWhenIncludeItemsIsNullButExcludeItemIsNotNull", args: args{
			includeItems: nil,
			excludeItems: []string{"a", "b"},
		}, want: []string{}},
		{name: "TestWhenExcludeItemsIsNullButIncludeItemIsNotNull", args: args{
			includeItems: []string{"a", "b"},
			excludeItems: nil,
		}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDifferenceSet(tt.args.includeItems, tt.args.excludeItems); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDifferenceSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDifferenceSetWhenIncludeItemOrExcludeItemIsNotNull(t *testing.T) {
	type args struct {
		includeItems []string
		excludeItems []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "TestWithIntersections", args: args{
			includeItems: []string{"a", "b"},
			excludeItems: []string{"b"},
		}, want: []string{"a"}},
		{name: "TestWithOutIntersections", args: args{
			includeItems: []string{"a", "b"},
			excludeItems: []string{"c"},
		}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDifferenceSet(tt.args.includeItems, tt.args.excludeItems); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDifferenceSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUniqueElementsWhenListIsNull(t *testing.T) {
	type args struct {
		list []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "TestWhenListIsNull", args: args{list: nil}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUniqueElements(tt.args.list); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUniqueElements() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUniqueElementsWhenListIsNotNull(t *testing.T) {
	type args struct {
		list []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "TestWhenListWithDuplication", args: args{list: []string{"a", "a", "c"}}, want: []string{"a", "c"}},
		{name: "TestWhenListWithOutDuplication", args: args{list: []string{"a", "c"}}, want: []string{"a", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUniqueElements(tt.args.list); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUniqueElements() = %v, want %v", got, tt.want)
			}
		})
	}
}
