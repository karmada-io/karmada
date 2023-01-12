package overridemanager

import (
	"reflect"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_overrideManagerImpl_ApplyOverridePolicies(t *testing.T) {
	deployment := helper.NewDeployment(metav1.NamespaceDefault, "test1")
	deployment.Labels = map[string]string{
		"testLabel": "testLabel",
	}
	deployment.Annotations = map[string]string{
		"testAnnotation": "testAnnotation",
	}
	deploymentObj, _ := utilhelper.ToUnstructured(deployment)

	clusterRole := helper.NewClusterRole("test2", nil)
	clusterRole.Labels = map[string]string{
		"testLabel": "testLabel",
	}
	clusterRole.Annotations = map[string]string{
		"testAnnotation": "testAnnotation",
	}
	clusterRoleObj, _ := utilhelper.ToUnstructured(clusterRole)
	type fields struct {
		Client        client.Client
		EventRecorder record.EventRecorder
	}
	type args struct {
		rawObj      *unstructured.Unstructured
		clusterName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantCOP *AppliedOverrides
		wantOP  *AppliedOverrides
		wantErr bool
	}{
		{
			name: "test overridePolicies",
			fields: fields{
				Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(helper.NewCluster("test1"),
					&policyv1alpha1.OverridePolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test1", Namespace: metav1.NamespaceDefault},
						Spec: policyv1alpha1.OverrideSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "default",
									Name:       "test1",
								},
							},
							OverrideRules: []policyv1alpha1.RuleWithCluster{
								{
									TargetCluster: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"test1"}},
									Overriders: policyv1alpha1.Overriders{
										LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
											{
												Operator: policyv1alpha1.OverriderOpAdd,
												Value: map[string]string{
													"testAddLabel": "testAddLabel",
												},
											},
										},
										AnnotationsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
											{
												Operator: policyv1alpha1.OverriderOpAdd,
												Value: map[string]string{
													"testAddAnnotation": "testAddAnnotation",
												},
											},
										},
									},
								},
							},
						},
					},
				).Build(),
				EventRecorder: &record.FakeRecorder{},
			},
			args: args{
				rawObj:      deploymentObj,
				clusterName: "test1",
			},
			wantCOP: nil,
			wantOP: &AppliedOverrides{
				AppliedItems: []OverridePolicyShadow{
					{
						PolicyName: "test1",
						Overriders: policyv1alpha1.Overriders{
							LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
								{
									Operator: policyv1alpha1.OverriderOpAdd,
									Value: map[string]string{
										"testAddLabel": "testAddLabel",
									},
								},
							},
							AnnotationsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
								{
									Operator: policyv1alpha1.OverriderOpAdd,
									Value: map[string]string{
										"testAddAnnotation": "testAddAnnotation",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test clusterOverridePolicy",
			fields: fields{
				Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(helper.NewCluster("test2"),
					&policyv1alpha1.ClusterOverridePolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: metav1.NamespaceDefault},
						Spec: policyv1alpha1.OverrideSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "rbac.authorization.k8s.io/v1",
									Kind:       "ClusterRole",
									Name:       "test2",
								},
							},
							OverrideRules: []policyv1alpha1.RuleWithCluster{
								{
									TargetCluster: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"test2"}},
									Overriders: policyv1alpha1.Overriders{
										LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
											{
												Operator: policyv1alpha1.OverriderOpAdd,
												Value: map[string]string{
													"testAddLabel": "testAddLabel",
												},
											},
										},
										AnnotationsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
											{
												Operator: policyv1alpha1.OverriderOpAdd,
												Value: map[string]string{
													"testAddAnnotation": "testAddAnnotation",
												},
											},
										},
									},
								},
							},
						},
					},
				).Build(),
				EventRecorder: &record.FakeRecorder{},
			},
			args: args{
				rawObj:      clusterRoleObj,
				clusterName: "test2",
			},
			wantCOP: &AppliedOverrides{
				AppliedItems: []OverridePolicyShadow{
					{
						PolicyName: "test2",
						Overriders: policyv1alpha1.Overriders{
							LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
								{
									Operator: policyv1alpha1.OverriderOpAdd,
									Value: map[string]string{
										"testAddLabel": "testAddLabel",
									},
								},
							},
							AnnotationsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
								{
									Operator: policyv1alpha1.OverriderOpAdd,
									Value: map[string]string{
										"testAddAnnotation": "testAddAnnotation",
									},
								},
							},
						},
					},
				},
			},
			wantOP:  nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &overrideManagerImpl{
				Client:        tt.fields.Client,
				EventRecorder: tt.fields.EventRecorder,
			}
			gotCOP, gotOP, err := o.ApplyOverridePolicies(tt.args.rawObj, tt.args.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyOverridePolicies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCOP, tt.wantCOP) {
				t.Errorf("ApplyOverridePolicies() gotCOP = %v, wantCOP %v", gotCOP, tt.wantCOP)
			}
			if !reflect.DeepEqual(gotOP, tt.wantOP) {
				t.Errorf("ApplyOverridePolicies() gotOP = %v, wantOP %v", gotOP, tt.wantOP)
			}
			wantLabels := map[string]string{"testLabel": "testLabel", "testAddLabel": "testAddLabel"}
			if !reflect.DeepEqual(tt.args.rawObj.GetLabels(), wantLabels) {
				t.Errorf("ApplyOverridePolicies() gotLabels = %v, wantLabels %v", tt.args.rawObj.GetLabels(), wantLabels)
			}
			wantAnnotations := map[string]string{"testAnnotation": "testAnnotation", "testAddAnnotation": "testAddAnnotation"}
			if !reflect.DeepEqual(tt.args.rawObj.GetAnnotations(), wantAnnotations) {
				t.Errorf("ApplyOverridePolicies() gotAnnotations = %v, wantAnnotations %v", tt.args.rawObj.GetAnnotations(), wantAnnotations)
			}
		})
	}
}

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
