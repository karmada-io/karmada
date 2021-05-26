package overridemanager

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func generateDeploymentYaml() *unstructured.Unstructured {
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
								"image": "fictional.registry.example/imagename:v1.0.0",
								"name":  "nginx",
							}}}}}}}
}

func generatePodYaml() *unstructured.Unstructured {
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
						"image": "fictional.registry.example/imagename:v1.0.0",
						"name":  "nginx",
					}}}}}
}

func generateStatefulSetYaml() *unstructured.Unstructured {
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
								"image": "fictional.registry.example/imagename:v1.0.0",
								"name":  "nginx",
							}}}}}}}
}

func generateReplicaSetYaml() *unstructured.Unstructured {
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
								"image": "fictional.registry.example/imagename:v1.0.0",
								"name":  "nginx",
							}}}}}}}
}

func generateDeploymentYamlWithTwoContainer() *unstructured.Unstructured {
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
								"image": "fictional.registry.example/imagename:v1.0.0",
								"name":  "nginx",
							},
							map[string]interface{}{
								"image": "k8s.gcr.io/nginx-slim:0.8",
								"name":  "nginx",
							}}}}}}}
}

func TestParseJSONPatchesByImageOverrider(t *testing.T) {
	type args struct {
		rawObj         *unstructured.Unstructured
		imageOverrider *policyv1alpha1.ImageOverrider
	}
	tests := []struct {
		name    string
		args    args
		want    []overrideOption
		wantErr bool
	}{
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Registry, operator: add",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Registry",
					Operator:  "add",
					Value:     ".test",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example.test/imagename:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Registry, operator: replace",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Registry",
					Operator:  "replace",
					Value:     "fictional.registry.us",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.us/imagename:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Registry, operator: remove",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Registry",
					Operator:  "remove",
					Value:     "fictional.registry.us",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "imagename:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Repository, operator: add",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "add",
					Value:     "/nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/imagename/nginx:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Repository, operator: replace",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/nginx:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Repository, operator: remove",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "remove",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Tag, operator: add",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Tag",
					Operator:  "add",
					Value:     "sha256:dbcc1c35ac38df41fd2f5e4130b32ffdb93ebae8b3dbe638c23575912276fc9c",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/imagename:v1.0.0", // only one of tag and digest is valid.
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Tag, operator: replace",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Tag",
					Operator:  "replace",
					Value:     "sha256:dbcc1c35ac38df41fd2f5e4130b32ffdb93ebae8b3dbe638c23575912276fc9c",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/imagename@sha256:dbcc1c35ac38df41fd2f5e4130b32ffdb93ebae8b3dbe638c23575912276fc9c",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Deployment, component: Tag, operator: remove",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Tag",
					Operator:  "remove",
					Value:     "sha256:dbcc1c35ac38df41fd2f5e4130b32ffdb93ebae8b3dbe638c23575912276fc9c",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/imagename",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: Pod",
			args: args{
				rawObj: generatePodYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/containers/0/image",
					Value: "fictional.registry.example/nginx:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: StatefulSet",
			args: args{
				rawObj: generateStatefulSetYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/nginx:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource kind: ReplicaSet",
			args: args{
				rawObj: generateReplicaSetYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/nginx:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with empty predicate, resource has more than one container",
			args: args{
				rawObj: generateDeploymentYamlWithTwoContainer(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/nginx:v1.0.0",
				},
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/1/image",
					Value: "k8s.gcr.io/nginx:0.8",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with predicate, resource has one container",
			args: args{
				rawObj: generateDeploymentYaml(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Predicate: &policyv1alpha1.ImagePredicate{
						Path: "/spec/template/spec/containers/0/image",
					},
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "fictional.registry.example/nginx:v1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "imageOverrider with predicate, resource has more than one container",
			args: args{
				rawObj: generateDeploymentYamlWithTwoContainer(),
				imageOverrider: &policyv1alpha1.ImageOverrider{
					Predicate: &policyv1alpha1.ImagePredicate{
						Path: "/spec/template/spec/containers/1/image",
					},
					Component: "Repository",
					Operator:  "replace",
					Value:     "nginx",
				},
			},
			want: []overrideOption{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/1/image",
					Value: "k8s.gcr.io/nginx:0.8",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPatches(tt.args.rawObj, tt.args.imageOverrider)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildPatches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
