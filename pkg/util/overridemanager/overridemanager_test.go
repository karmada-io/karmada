package overridemanager

import (
	"reflect"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/helper"
)

func TestGetMatchingOverridePolicies(t *testing.T) {
	cluster1 := helper.NewCluster("cluster1")
	cluster2 := helper.NewCluster("cluster2")

	deployment := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deploymentObj, _ := utilhelper.ToUnstructured(deployment)

	overriders1 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("foo: bar")},
			},
		},
	}
	overriders2 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("aaa: bbb")},
			},
		},
	}
	overriders3 := policyv1alpha1.Overriders{
		Plaintext: []policyv1alpha1.PlaintextOverrider{
			{
				Path:     "/metadata/annotations",
				Operator: "add",
				Value:    apiextensionsv1.JSON{Raw: []byte("hello: world")},
			},
		},
	}
	// high implicit priority
	overridePolicy1 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy1",
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			},
			OverrideRules: []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{cluster1.Name, cluster2.Name},
					},
					Overriders: overriders1,
				},
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{cluster2.Name},
					},
					Overriders: overriders2,
				},
			},
		},
	}
	// low implicit priority
	overridePolicy2 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy2",
		},
		Spec: policyv1alpha1.OverrideSpec{
			OverrideRules: []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{cluster1.Name, cluster2.Name},
					},
					Overriders: overriders3,
				},
			},
		},
	}
	overridePolicy3 := &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "overridePolicy3",
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{},
			OverrideRules: []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{cluster1.Name, cluster2.Name},
					},
					Overriders: overriders3,
				},
			},
		},
	}
	oldOverridePolicy := &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "oldOverridePolicy",
		},
		Spec: policyv1alpha1.OverrideSpec{
			TargetCluster: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster1.Name},
			},
			Overriders: overriders3,
		},
	}

	m := &overrideManagerImpl{}
	tests := []struct {
		name             string
		policies         []GeneralOverridePolicy
		resource         *unstructured.Unstructured
		cluster          *clusterv1alpha1.Cluster
		wantedOverriders []policyOverriders
	}{
		{
			name:     "OverrideRules test 1",
			policies: []GeneralOverridePolicy{overridePolicy1, overridePolicy2},
			resource: deploymentObj,
			cluster:  cluster1,
			wantedOverriders: []policyOverriders{
				{
					name:       overridePolicy2.Name,
					namespace:  overridePolicy2.Namespace,
					overriders: overriders3,
				},
				{
					name:       overridePolicy1.Name,
					namespace:  overridePolicy1.Namespace,
					overriders: overriders1,
				},
			},
		},
		{
			name:     "OverrideRules test 2",
			policies: []GeneralOverridePolicy{overridePolicy1, overridePolicy2},
			resource: deploymentObj,
			cluster:  cluster2,
			wantedOverriders: []policyOverriders{
				{
					name:       overridePolicy2.Name,
					namespace:  overridePolicy2.Namespace,
					overriders: overriders3,
				},
				{
					name:       overridePolicy1.Name,
					namespace:  overridePolicy1.Namespace,
					overriders: overriders1,
				},
				{
					name:       overridePolicy1.Name,
					namespace:  overridePolicy1.Namespace,
					overriders: overriders2,
				},
			},
		},
		{
			name:     "OverrideRules test 3",
			policies: []GeneralOverridePolicy{overridePolicy3},
			resource: deploymentObj,
			cluster:  cluster2,
			wantedOverriders: []policyOverriders{
				{
					name:       overridePolicy3.Name,
					namespace:  overridePolicy3.Namespace,
					overriders: overriders3,
				},
			},
		},
		{
			name:     "TargetCluster and Overriders test",
			policies: []GeneralOverridePolicy{oldOverridePolicy},
			resource: deploymentObj,
			cluster:  cluster1,
			wantedOverriders: []policyOverriders{
				{
					name:       oldOverridePolicy.Name,
					namespace:  oldOverridePolicy.Namespace,
					overriders: overriders3,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.getOverridersFromOverridePolicies(tt.policies, tt.resource, tt.cluster); !reflect.DeepEqual(got, tt.wantedOverriders) {
				t.Errorf("getOverridersFromOverridePolicies() = %v, want %v", got, tt.wantedOverriders)
			}
		})
	}
}
