package helper

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
)

var Schema = runtime.NewScheme()

func init() {
	utilruntime.Must(karmadafake.AddToScheme(Schema))
}

func TestIsOverridePolicyExist(t *testing.T) {
	tests := []struct {
		name            string
		policyName      string
		policyNamespace string
		policyCreated   bool
		expected        bool
	}{
		{
			name:            "override policy exists",
			policyName:      "foo",
			policyNamespace: "bar",
			policyCreated:   true,
			expected:        true,
		},
		{
			name:            "override policy does not exist",
			policyName:      "foo",
			policyNamespace: "bar",
			policyCreated:   false,
			expected:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(Schema).Build()
			if tt.policyCreated {
				testOverridePolicy := &policyv1alpha1.OverridePolicy{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: tt.policyNamespace,
						Name:      tt.policyName,
					},
				}
				err := fakeClient.Create(context.TODO(), testOverridePolicy)
				if err != nil {
					t.Fatalf("failed to create overridePolicy, err is: %v", err)
				}
			}
			res, err := IsOverridePolicyExist(fakeClient, tt.policyNamespace, tt.policyName)
			if err != nil {
				t.Fatalf("failed to check overridePolicy exist, err is: %v", err)
			}
			if res != tt.expected {
				t.Errorf("IsOverridePolicyExist() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestIsClusterOverridePolicyExist(t *testing.T) {
	tests := []struct {
		name          string
		policyName    string
		policyCreated bool
		expected      bool
	}{
		{
			name:          "cluster override policy exists",
			policyName:    "foo",
			policyCreated: true,
			expected:      true,
		},
		{
			name:          "cluster override policy does not exist",
			policyName:    "foo",
			policyCreated: false,
			expected:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(Schema).Build()
			if tt.policyCreated {
				testClusterOverridePolicy := &policyv1alpha1.ClusterOverridePolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: tt.policyName,
					},
				}
				err := fakeClient.Create(context.TODO(), testClusterOverridePolicy)
				if err != nil {
					t.Fatalf("failed to create clusterOverridePolicy, err is: %v", err)
				}
			}
			res, err := IsClusterOverridePolicyExist(fakeClient, tt.policyName)
			if err != nil {
				t.Fatalf("failed to check clusterOverridePolicy exist, err is: %v", err)
			}
			if res != tt.expected {
				t.Errorf("IsClusterOverridePolicyExist() = %v, want %v", res, tt.expected)
			}
		})
	}
}
