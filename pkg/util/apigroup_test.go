package util

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSkippedResourceConfigGVKParse(t *testing.T) {
	tests := []struct {
		input string
		out   []schema.GroupVersionKind
	}{
		{
			input: "v1/Node,Pod;networking.k8s.io/v1beta1/Ingress,IngressClass",
			out: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "Node",
				},
				{
					Group:   "",
					Version: "v1",
					Kind:    "Pod",
				},
				{
					Group:   "networking.k8s.io",
					Version: "v1beta1",
					Kind:    "Ingress",
				},
				{
					Group:   "networking.k8s.io",
					Version: "v1beta1",
					Kind:    "IngressClass",
				},
			}},
	}
	for _, test := range tests {
		r := NewSkippedResourceConfig()
		if err := r.Parse(test.input); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		for i, o := range test.out {
			ok := r.GroupVersionKindDisabled(o)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, o)
			}
		}
	}
}
func TestResourceConfigGVParse(t *testing.T) {
	tests := []struct {
		input string
		out   []schema.GroupVersion
	}{
		{
			input: "networking.k8s.io/v1;test/v1beta1",
			out: []schema.GroupVersion{
				{
					Group:   "networking.k8s.io",
					Version: "v1",
				},
				{
					Group:   "networking.k8s.io",
					Version: "v1",
				},
				{
					Group:   "test",
					Version: "v1beta1",
				},
				{
					Group:   "test",
					Version: "v1beta1",
				},
			}},
	}
	for _, test := range tests {
		r := NewSkippedResourceConfig()
		if err := r.Parse(test.input); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		for i, o := range test.out {
			ok := r.GroupVersionDisabled(o)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, o)
			}
		}
	}
}

func TestSkippedResourceConfigGroupParse(t *testing.T) {
	tests := []struct {
		input string
		out   []string
	}{
		{
			input: "networking.k8s.io;test",
			out: []string{
				"networking.k8s.io", "test",
			}},
	}
	for _, test := range tests {
		r := NewSkippedResourceConfig()
		if err := r.Parse(test.input); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		for i, o := range test.out {
			ok := r.GroupDisabled(o)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, o)
			}
		}
	}
}

func TestSkippedResourceConfigMixedParse(t *testing.T) {
	tests := []struct {
		input string
		out   SkippedResourceConfig
	}{
		{
			input: "v1/Node,Pod;networking.k8s.io/v1beta1/Ingress,IngressClass;networking.k8s.io;test.com/v1",
			out: SkippedResourceConfig{
				Groups: map[string]struct{}{
					"networking.k8s.io": {},
				},
				GroupVersions: map[schema.GroupVersion]struct{}{
					{
						Group:   "test.com",
						Version: "v1",
					}: {},
				},
				GroupVersionKinds: map[schema.GroupVersionKind]struct{}{
					{
						Group:   "",
						Version: "v1",
						Kind:    "Node",
					}: {},
					{
						Group:   "",
						Version: "v1",
						Kind:    "Pod",
					}: {},
					{
						Group:   "networking.k8s.io",
						Version: "v1beta1",
						Kind:    "Ingress",
					}: {},
					{
						Group:   "networking.k8s.io",
						Version: "v1beta1",
						Kind:    "IngressClass",
					}: {},
				},
			}},
	}
	for i, test := range tests {
		r := NewSkippedResourceConfig()
		if err := r.Parse(test.input); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		for g := range r.Groups {
			ok := r.GroupDisabled(g)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, g)
			}
		}

		for gv := range r.GroupVersions {
			ok := r.GroupVersionDisabled(gv)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, gv)
			}
		}

		for gvk := range r.GroupVersionKinds {
			ok := r.GroupVersionKindDisabled(gvk)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, gvk)
			}
		}
	}
}

func TestDefaultSkippedResourceConfigGroupParse(t *testing.T) {
	tests := []struct {
		input string
		out   []string
	}{
		{
			input: "",
			out: []string{
				"cluster.karmada.io", "policy.karmada.io", "work.karmada.io",
			}},
	}
	for _, test := range tests {
		r := NewSkippedResourceConfig()
		if err := r.Parse(test.input); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		for i, o := range test.out {
			ok := r.GroupDisabled(o)
			if !ok {
				t.Errorf("%d: unexpected error: %v", i, o)
			}
		}
		ok := r.GroupDisabled("")
		if ok {
			t.Error("unexpected error: v1")
		}
	}
}
