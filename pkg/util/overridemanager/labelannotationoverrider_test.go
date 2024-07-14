/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package overridemanager

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_applyLabelsOverriders(t *testing.T) {
	type args struct {
		rawObj            *unstructured.Unstructured
		commandOverriders []policyv1alpha1.LabelAnnotationOverrider
	}
	deployment := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deployment.Labels = map[string]string{
		"foo": "foo",
		"bar": "bar",
	}
	deployment2 := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deployment2.Labels = nil
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "test empty labels",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment2)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"test2": "test2",
						},
					},
				},
			},
			want: map[string]string{
				"test2": "test2",
			},
			wantErr: false,
		},
		{
			name: "test labels replace",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpReplace,
						Value: map[string]string{
							"foo":       "exist",
							"non-exist": "non-exist",
						},
					},
				},
			},
			want: map[string]string{
				"foo": "exist",
				"bar": "bar",
			},
			wantErr: false,
		},
		{
			name: "test labels add",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"test": "test",
						},
					},
				},
			},
			want: map[string]string{
				"test": "test",
				"foo":  "foo",
				"bar":  "bar",
			},
			wantErr: false,
		},
		{
			name: "test labels remove",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value: map[string]string{
							"foo":       "foo",
							"non-exist": "non-exist",
						},
					},
				},
			},
			want: map[string]string{
				"bar": "bar",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := applyLabelsOverriders(tt.args.rawObj, tt.args.commandOverriders); (err != nil) != tt.wantErr {
				t.Fatalf("applyLabelsOverriders() error = %v, wantErr %v", err, tt.wantErr)
			}
			newDeployment := &appsv1.Deployment{}
			_ = utilhelper.ConvertToTypedObject(tt.args.rawObj, newDeployment)
			if !reflect.DeepEqual(tt.want, newDeployment.Labels) {
				t.Errorf("expect: %v, but got: %v", tt.want, newDeployment.Labels)
			}
		})
	}
}

func Test_applyAnnotationsOverriders(t *testing.T) {
	type args struct {
		rawObj            *unstructured.Unstructured
		commandOverriders []policyv1alpha1.LabelAnnotationOverrider
	}
	deployment := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deployment.Annotations = map[string]string{
		"foo": "foo",
		"bar": "bar",
	}
	deployment2 := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deployment2.Annotations = nil

	deployment3 := helper.NewDeployment(metav1.NamespaceDefault, "test")
	deployment3.Annotations = map[string]string{
		"testannotation/projectId": "c-m-lfx9lk92:p-v86cf",
		"foo":                      "foo",
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "test empty annotations",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment2)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"test2": "test2",
						},
					},
				},
			},
			want: map[string]string{
				"test2": "test2",
			},
			wantErr: false,
		},
		{
			name: "test annotations replace",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpReplace,
						Value: map[string]string{
							"foo":       "exist",
							"non-exist": "non-exist",
						},
					},
				},
			},
			want: map[string]string{
				"foo": "exist",
				"bar": "bar",
			},
			wantErr: false,
		},
		{
			name: "test annotations add",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"test": "test",
						},
					},
				},
			},
			want: map[string]string{
				"test": "test",
				"foo":  "foo",
				"bar":  "bar",
			},
			wantErr: false,
		},
		{
			name: "test annotations remove",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value: map[string]string{
							"foo":       "foo",
							"non-exist": "non-exist",
						},
					},
				},
			},
			want: map[string]string{
				"bar": "bar",
			},
			wantErr: false,
		},
		{
			name: "test annotations remain the same",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value:    map[string]string{},
					},
				},
			},
			want: map[string]string{
				"bar": "bar",
				"foo": "foo",
			},
			wantErr: false,
		},
		{
			name: "test annotations remain the same",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpReplace,
						Value: map[string]string{
							"test": "test",
						},
					},
				},
			},
			want: map[string]string{
				"bar": "bar",
				"foo": "foo",
			},
			wantErr: false,
		},
		{
			name: "test annotations remain the same",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value: map[string]string{
							"test": "test",
						},
					},
				},
			},
			want: map[string]string{
				"bar": "bar",
				"foo": "foo",
			},
			wantErr: false,
		},
		{
			name: "test add composed annotation",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"testannotation/projectId": "c-m-lfx9lk92:p-v86cf",
						},
					},
				},
			},
			want: map[string]string{
				"foo":                      "foo",
				"bar":                      "bar",
				"testannotation/projectId": "c-m-lfx9lk92:p-v86cf",
			},
			wantErr: false,
		},
		{
			name: "test remove composed annotation",
			args: args{
				rawObj: func() *unstructured.Unstructured {
					deploymentObj, _ := utilhelper.ToUnstructured(deployment3)
					return deploymentObj
				}(),
				commandOverriders: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value: map[string]string{
							"testannotation/projectId": "c-m-lfx9lk92:p-v86cf",
						},
					},
				},
			},
			want:    map[string]string{"foo": "foo"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := applyAnnotationsOverriders(tt.args.rawObj, tt.args.commandOverriders); (err != nil) != tt.wantErr {
				t.Fatalf("applyAnnotationsOverriders() error = %v, wantErr %v", err, tt.wantErr)
			}
			newDeployment := &appsv1.Deployment{}
			_ = utilhelper.ConvertToTypedObject(tt.args.rawObj, newDeployment)
			if !reflect.DeepEqual(tt.want, newDeployment.Annotations) {
				t.Errorf("expect: %v, but got: %v", tt.want, newDeployment.Annotations)
			}
		})
	}
}
