/*
Copyright 2021 The Karmada Authors.

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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func generateTestCommandDeploymentYaml() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image":   "nginx",
								"name":    "nginx",
								"command": []interface{}{"nginx", "-v", "-t"},
							}}}}}}}
}

func generateTestArgsDeploymentYaml() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image": "nginx",
								"name":  "nginx",
								"args":  []interface{}{"nginx", "-v", "-t"},
							}}}}}}}
}

func generateTestCommandPodYaml() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"image":   "fictional.registry.example/imagename:v1.0.0",
						"name":    "nginx",
						"command": []interface{}{"nginx", "-v", "-t"},
					}}}}}
}

func generateTestCommandStatefulSetYaml() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata": map[string]interface{}{
				"name": "web",
			},
			"spec": map[string]interface{}{
				"replicas": 2,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image":   "fictional.registry.example/imagename:v1.0.0",
								"name":    "nginx",
								"command": []interface{}{"nginx", "-v", "-t"},
							}}}}}}}
}

func generateTestCommandReplicaSetYaml() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image":   "fictional.registry.example/imagename:v1.0.0",
								"name":    "nginx",
								"command": []interface{}{"nginx", "-v", "-t"},
							}}}}}}}
}

func generateTestCommandDaemonSetYaml() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image":   "fictional.registry.example/imagename:v1.0.0",
								"name":    "nginx",
								"command": []interface{}{"nginx", "-v", "-t"},
							}}}}}}}
}

func generateTestCommandDeploymentYamlWithTwoContainer() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "nginx",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 1,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":    "nginx",
								"command": []interface{}{"nginx", "-v", "-t"},
							},
							map[string]interface{}{
								"name":    "nginx1",
								"command": []interface{}{"nginx", "-v", "-t"},
							}}}}}}}
}

func TestParseJSONPatchesByCommandOverrider(t *testing.T) {
	type args struct {
		rawObj               *unstructured.Unstructured
		CommandArgsOverrider *policyv1alpha1.CommandArgsOverrider
	}
	tests := []struct {
		name    string
		args    args
		want    []overrideOption
		wantErr bool
	}{
		{
			name: "CommandArgsOverrider, resource kind: Deployment, operator: add",
			args: args{
				rawObj: generateTestCommandDeploymentYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpAdd,
					Value:         []string{"&& echo 'hello karmada'"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v", "-t", "&& echo 'hello karmada'"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource kind: Deployment, operator: remove",
			args: args{
				rawObj: generateTestCommandDeploymentYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpRemove,
					Value:         []string{"-t"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, remove value is empty, resource kind: Deployment, operator: remove",
			args: args{
				rawObj: generateTestCommandDeploymentYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpRemove,
					Value:         []string{},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v", "-t"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource has more than one container",
			args: args{
				rawObj: generateTestCommandDeploymentYamlWithTwoContainer(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpAdd,
					Value:         []string{"echo 'hello karmada'"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v", "-t", "echo 'hello karmada'"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource has more than one container",
			args: args{
				rawObj: generateTestCommandDeploymentYamlWithTwoContainer(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpRemove,
					Value:         []string{"-t"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider resource kind: Pod, operator: add",
			args: args{
				rawObj: generateTestCommandPodYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpAdd,
					Value:         []string{"echo 'hello karmada'"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/containers/0/command",
					Value: []string{"nginx", "-v", "-t", "echo 'hello karmada'"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource kind: StatefulSet, operator: add",
			args: args{
				rawObj: generateTestCommandStatefulSetYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpAdd,
					Value:         []string{"echo 'hello karmada'"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v", "-t", "echo 'hello karmada'"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource kind: ReplicaSet, operator: remove",
			args: args{
				rawObj: generateTestCommandReplicaSetYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpRemove,
					Value:         []string{"-t"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource kind: DaemonSet, operator: remove",
			args: args{
				rawObj: generateTestCommandDaemonSetYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpRemove,
					Value:         []string{"-t"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/command",
					Value: []string{"nginx", "-v"},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildCommandArgsPatches(CommandString, tt.args.rawObj, tt.args.CommandArgsOverrider)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildCommandArgsPatches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildCommandArgsPatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseJSONPatchesByArgsOverrider(t *testing.T) {
	type args struct {
		rawObj               *unstructured.Unstructured
		CommandArgsOverrider *policyv1alpha1.CommandArgsOverrider
	}
	tests := []struct {
		name    string
		args    args
		want    []overrideOption
		wantErr bool
	}{
		{
			name: "CommandArgsOverrider, resource kind: Deployment, operator: replace",
			args: args{
				rawObj: generateTestArgsDeploymentYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpAdd,
					Value:         []string{"&& echo 'hello karmada'"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/args",
					Value: []string{"nginx", "-v", "-t", "&& echo 'hello karmada'"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource kind: Deployment, operator: replace",
			args: args{
				rawObj: generateTestArgsDeploymentYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpRemove,
					Value:         []string{"-t"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpReplace),
					Path:  "/spec/template/spec/containers/0/args",
					Value: []string{"nginx", "-v"},
				},
			},
			wantErr: false,
		}, {
			name: "CommandArgsOverrider, resource kind: Deployment, operator: add",
			args: args{
				rawObj: generateTestCommandDeploymentYaml(),
				CommandArgsOverrider: &policyv1alpha1.CommandArgsOverrider{
					ContainerName: "nginx",
					Operator:      policyv1alpha1.OverriderOpAdd,
					Value:         []string{"-t"},
				},
			},
			want: []overrideOption{
				{
					Op:    string(policyv1alpha1.OverriderOpAdd),
					Path:  "/spec/template/spec/containers/0/args",
					Value: []string{"-t"},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildCommandArgsPatches(ArgsString, tt.args.rawObj, tt.args.CommandArgsOverrider)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildCommandPatches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildCommandPatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
