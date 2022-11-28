package interpreter

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestOperation(t *testing.T) {
	tests := []struct {
		name     string
		matcher  *Matcher
		expected bool
	}{
		{
			name: "operations with wildcard",
			matcher: &Matcher{
				Rule: configv1alpha1.RuleWithOperations{
					Operations: []configv1alpha1.InterpreterOperation{
						configv1alpha1.InterpreterOperationAll,
						configv1alpha1.InterpreterOperationInterpretReplica,
					},
				},
			},
			expected: true,
		},
		{
			name: "equal operations",
			matcher: &Matcher{
				Operation: configv1alpha1.InterpreterOperationRetain,
				Rule: configv1alpha1.RuleWithOperations{
					Operations: []configv1alpha1.InterpreterOperation{
						configv1alpha1.InterpreterOperationRetain,
						configv1alpha1.InterpreterOperationInterpretReplica,
					},
				},
			},
			expected: true,
		},
		{
			name: "not equal operations",
			matcher: &Matcher{
				Operation: configv1alpha1.InterpreterOperationPrune,
				Rule: configv1alpha1.RuleWithOperations{
					Operations: []configv1alpha1.InterpreterOperation{
						configv1alpha1.InterpreterOperationRetain,
						configv1alpha1.InterpreterOperationInterpretReplica,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.matcher.operation()
			if res != tt.expected {
				t.Errorf("operation() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		name     string
		matcher  *Matcher
		expected bool
	}{
		{
			name: "group with wildcard",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						APIGroups: []string{Wildcard},
					},
				},
			},
			expected: true,
		},
		{
			name: "equal group",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						APIGroups: []string{"batch", "apps"},
					},
				},
			},
			expected: true,
		},
		{
			name: "not equal group",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						APIGroups: []string{"batch"},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.matcher.group()
			if res != tt.expected {
				t.Errorf("group() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	tests := []struct {
		name     string
		matcher  *Matcher
		expected bool
	}{
		{
			name: "version with wildcard",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						APIVersions: []string{Wildcard},
					},
				},
			},
			expected: true,
		},
		{
			name: "equal version",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						APIVersions: []string{"v1", "v1beta1"},
					},
				},
			},
			expected: true,
		},
		{
			name: "not equal version",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						APIVersions: []string{"v1beta1"},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.matcher.version()
			if res != tt.expected {
				t.Errorf("version() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestKind(t *testing.T) {
	tests := []struct {
		name     string
		matcher  *Matcher
		expected bool
	}{
		{
			name: "kind with wildcard",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						Kinds: []string{Wildcard},
					},
				},
			},
			expected: true,
		},
		{
			name: "equal kind",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						Kinds: []string{"Pod", "Deployment"},
					},
				},
			},
			expected: true,
		},
		{
			name: "not equal kind",
			matcher: &Matcher{
				ObjGVK: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				Rule: configv1alpha1.RuleWithOperations{
					Rule: configv1alpha1.Rule{
						Kinds: []string{"Pod"},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.matcher.kind()
			if res != tt.expected {
				t.Errorf("kind() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestExactOrWildcard(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		items     []string
		expected  bool
	}{
		{
			name:     "wildcard",
			items:    []string{Wildcard},
			expected: true,
		},
		{
			name:      "equal",
			requested: "foo",
			items:     []string{"foo", "bar"},
			expected:  true,
		},
		{
			name:      "not equal",
			requested: "foo",
			items:     []string{"bar"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := exactOrWildcard(tt.requested, tt.items)
			if res != tt.expected {
				t.Errorf("exactOrWildcard() = %v, want %v", res, tt.expected)
			}
		})
	}
}
