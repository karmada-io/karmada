package crdexplorer

import (
	"fmt"
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func strPtr(s string) *string { return &s }

func int32Ptr(i int32) *int32 { return &i }

func TestHasWildcardOperation(t *testing.T) {
	tests := []struct {
		name       string
		operations []configv1alpha1.OperationType
		expected   bool
	}{
		{
			name:       "no operations",
			operations: nil,
			expected:   false,
		},
		{
			name: "has wildcard operation",
			operations: []configv1alpha1.OperationType{
				configv1alpha1.OperationAll,
			},
			expected: true,
		},
		{
			name: "no have wildcard operation",
			operations: []configv1alpha1.OperationType{
				configv1alpha1.ExploreReplica,
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := hasWildcardOperation(test.operations)
			if result != test.expected {
				t.Errorf("Case: %s failed: expected get %v, but got %v", test.name, test.expected, result)
			}
		})
	}
}

func TestIsAcceptedExploreReviewVersions(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "is accepted explore review versions",
			version:  acceptedExploreReviewVersions[0],
			expected: true,
		},
		{
			name:     "is not accepted explore review versions",
			version:  "",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isAcceptedExploreReviewVersions(test.version)
			if result != test.expected {
				t.Errorf("Case: %s failed: expected get %v, but got %v", test.name, test.expected, result)
			}
		})
	}
}

func TestValidateRuleWithOperations(t *testing.T) {
	fldPath := field.NewPath("webhooks").Child("rules")
	notSupportedOperation := []configv1alpha1.OperationType{(configv1alpha1.OperationType)("notsupported")}

	tests := []struct {
		name               string
		ruleWithOperations *configv1alpha1.RuleWithOperations
		expectedError      string
	}{
		{
			name:               "no operations",
			ruleWithOperations: &configv1alpha1.RuleWithOperations{},
			expectedError:      "operations: Required value",
		},
		{
			name: "2 operations with wildcard",
			ruleWithOperations: &configv1alpha1.RuleWithOperations{
				Operations: []configv1alpha1.OperationType{
					configv1alpha1.OperationAll,
					configv1alpha1.ExploreReplica,
				},
			},
			expectedError: "if '*' is present, must not specify other operations",
		},
		{
			name:               "not supported operations",
			ruleWithOperations: &configv1alpha1.RuleWithOperations{Operations: notSupportedOperation},
			expectedError:      "operations[0]: Unsupported value",
		},
		{
			name: "not validated rule",
			ruleWithOperations: &configv1alpha1.RuleWithOperations{
				Operations: []configv1alpha1.OperationType{
					configv1alpha1.OperationAll,
				},
				Rule: configv1alpha1.Rule{
					APIGroups: []string{""},
				}},
			expectedError: "apiVersions: Required value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateRuleWithOperations(test.ruleWithOperations, fldPath)
			err := errs.ToAggregate()
			if err != nil {
				if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
					t.Errorf("Case: %s failed: expected to contain %s, got %s", test.name, e, a)
				}
			} else {
				if test.expectedError != "" {
					t.Errorf("Case: %s failed: unexpected no error, expected to contain %s", test.name, test.expectedError)
				}
			}
		})
	}
}

func TestValidateExploreReviewVersions(t *testing.T) {
	fldPath := field.NewPath("webhooks").Child("exploreReviewVersions")

	tests := []struct {
		name          string
		versions      []string
		expectedError string
	}{
		{
			name:          "no versions",
			versions:      nil,
			expectedError: fmt.Sprintf("must specify one of %v", strings.Join(acceptedExploreReviewVersions, ", ")),
		},
		{
			name:          "duplicate versions",
			versions:      []string{"v1", "v1"},
			expectedError: "duplicate version",
		},
		{
			name:          "invalid versions",
			versions:      []string{"test", "test"},
			expectedError: fmt.Sprintf("must include at least one of %v", strings.Join(acceptedExploreReviewVersions, ", ")),
		},
		{
			name:          "invalid DNS (RFC 1035) label",
			versions:      []string{"a b"},
			expectedError: "a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateExploreReviewVersions(test.versions, fldPath)
			err := errs.ToAggregate()
			if err != nil {
				if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
					t.Errorf("Case: %s failed: expected to contain %s, got %s", test.name, e, a)
				}
			} else {
				if test.expectedError != "" {
					t.Errorf("Case: %s failed: unexpected no error, expected to contain %s", test.name, test.expectedError)
				}
			}
		})
	}
}

func TestValidateWebhook(t *testing.T) {
	fldPath := field.NewPath("webhooks")
	policy := admissionregistrationv1.FailurePolicyType("fake policy")

	tests := []struct {
		name          string
		hook          *configv1alpha1.ResourceExploringWebhook
		expectedError string
	}{
		{
			name: "not qualified domain name",
			hook: &configv1alpha1.ResourceExploringWebhook{
				Name: "",
			},
			expectedError: "webhooks.name: Required value",
		},
		{
			name: "invalid rules",
			hook: &configv1alpha1.ResourceExploringWebhook{
				Rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.OperationType{},
					},
				},
			},
			expectedError: "operations: Required value",
		},
		{
			name: "invalid policy",
			hook: &configv1alpha1.ResourceExploringWebhook{
				FailurePolicy: &policy,
			},
			expectedError: "matchPolicy",
		},
		{
			name: "invalid timeout",
			hook: &configv1alpha1.ResourceExploringWebhook{
				TimeoutSeconds: int32Ptr(60),
			},
			expectedError: "the timeout value must be between 1 and 30 seconds",
		},
		{
			name: "ClientConfig: exactly one of url or service is required",
			hook: &configv1alpha1.ResourceExploringWebhook{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{},
			},
			expectedError: "exactly one of url or service is required",
		},
		{
			name: "ClientConfig: invalid URL",
			hook: &configv1alpha1.ResourceExploringWebhook{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: strPtr(""),
				},
			},
			expectedError: "host must be provided",
		},
		{
			name: "ClientConfig: invalid service",
			hook: &configv1alpha1.ResourceExploringWebhook{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name: "",
						Port: int32Ptr(8080),
						Path: strPtr(""),
					},
				},
			},
			expectedError: "service name is required",
		},
		{
			name: "invalid explore review versions",
			hook: &configv1alpha1.ResourceExploringWebhook{
				ExploreReviewVersions: []string{""},
			},
			expectedError: fmt.Sprintf("must include at least one of %v", strings.Join(acceptedExploreReviewVersions, ", ")),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateWebhook(test.hook, fldPath)
			err := errs.ToAggregate()
			if err != nil {
				if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
					t.Errorf("Case: %s failed: expected to contain %s, got %s", test.name, e, a)
				}
			} else {
				if test.expectedError != "" {
					t.Errorf("Case: %s failed: unexpected no error, expected to contain %s", test.name, test.expectedError)
				}
			}
		})
	}
}
