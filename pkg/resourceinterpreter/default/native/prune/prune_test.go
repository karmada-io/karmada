/*
Copyright 2023 The Karmada Authors.

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

package prune

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	storagevolume "k8s.io/component-helpers/storage/volume"
	utildeployment "k8s.io/kubectl/pkg/util/deployment"

	"github.com/karmada-io/karmada/pkg/util"
)

type field []string

func (f field) String() string {
	return "." + strings.Join(f, ".")
}

type test struct {
	name                    string
	workload                *unstructured.Unstructured
	extraHooks              []func(*unstructured.Unstructured)
	unexpectedFields        []field
	unexpectedResource      string
	shouldNotRemoveFields   []field
	shouldNotRemoveResource string
	containsFunc            func(interface{}, string) bool
}

func TestRemoveIrrelevantField(t *testing.T) {
	var tests = []*test{
		{
			name: "remove common object irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"creationTimestamp":          "2023-03-13T05:00:41Z",
						"deletionTimestamp":          "2023-03-13T06:00:41Z",
						"deletionGracePeriodSeconds": 10,
						"generation":                 2,
						"managedFields": []map[string]interface{}{
							{
								"apiVersion": "v1",
								"fieldsType": "FieldsV1",
								"manager":    "name",
								"operation":  "Apply",
							},
						},
						"resourceVersion": "22222",
						"selfLink":        "http://example.com",
						"uid":             "db56a4a6-0dff-465a-b046-2c1dea42a42b",
						"ownerReferences": []map[string]interface{}{
							{
								"apiVersion": "v1",
								"kind":       "Pod",
								"name":       "foo",
								"uid":        "fb11a9a6-1daa-265b-c046-1c1dea42a42c",
							},
						},
						"finalizers": []string{"foregroundDeletion"},
					},
					"status": map[string]interface{}{},
				},
			},
			extraHooks: nil,
			unexpectedFields: []field{
				{"metadata", "creationTimestamp"},
				{"metadata", "deletionTimestamp"},
				{"metadata", "deletionGracePeriodSeconds"},
				{"metadata", "generation"},
				{"metadata", "managedFields"},
				{"metadata", "resourceVersion"},
				{"metadata", "selfLink"},
				{"metadata", "uid"},
				{"metadata", "ownerReferences"},
				{"metadata", "finalizers"},
				{"status"},
			},
		},
		{
			name: "remove service irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.ServiceKind,
					"spec": map[string]interface{}{
						"clusterIP":  "10.10.10.10",
						"clusterIPs": []string{"10.10.10.10"},
					},
				},
			},
			extraHooks: nil,
			unexpectedFields: []field{
				{"spec", "clusterIP"},
				{"spec", "clusterIPs"},
			},
		},
		{
			name: "remove job irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.JobKind,
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"foo":            "bar",
								"controller-uid": "ab11a9a6-1daa-265b-c046-1c1dea42a42c",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"controller-uid": "ab11a9a6-1daa-265b-c046-1c1dea42a42c",
									"job-name":       "test-job",
									"foo":            "bar",
								},
							},
						},
						"ttlSecondsAfterFinished": 10,
					},
				},
			},
			extraHooks: []func(*unstructured.Unstructured){RemoveJobTTLSeconds},
			unexpectedFields: []field{
				{"spec", "selector", "matchLabels", "controller-uid"},
				{"spec", "template", "metadata", "labels", "controller-uid"},
				{"spec", "template", "metadata", "labels", "job-name"},
				{"spec", "ttlSecondsAfterFinished"},
			},
			shouldNotRemoveFields: []field{
				{"spec", "selector", "matchLabels", "foo"},
				{"spec", "template", "metadata", "labels", "foo"},
			},
		},
		{
			name: "remove serviceaccount irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.ServiceAccountKind,
					"metadata": map[string]interface{}{
						"name": "foo",
					},
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "foo-token-6pgxf",
						},
						map[string]interface{}{
							"name": "foo-dockercfg-zdr2j",
						},
					},
				},
			},
			extraHooks:              nil,
			unexpectedFields:        []field{{"secrets"}},
			unexpectedResource:      "foo-token-6pgxf",
			shouldNotRemoveFields:   []field{{"secrets"}},
			shouldNotRemoveResource: "foo-dockercfg-zdr2j",
			containsFunc: func(obj interface{}, resource string) bool {
				maps, _ := obj.([]interface{})

				for _, m := range maps {
					if m.(map[string]interface{})["name"] == resource {
						return true
					}
				}
				return false
			},
		},
		{
			name: "remove service-account token secret irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.SecretKind,
					"metadata": map[string]interface{}{
						corev1.ServiceAccountUIDKey: "123",
					},
					"type": string(corev1.SecretTypeServiceAccountToken),
					"data": map[string]interface{}{
						corev1.ServiceAccountTokenKey: "abc",
					},
				},
			},
			unexpectedFields: []field{
				{"metadata", "annotations", corev1.ServiceAccountUIDKey},
				{"data", corev1.ServiceAccountTokenKey},
			},
		},
		{
			name: "retains secret basic-auth fields",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.SecretKind,
					"metadata": map[string]interface{}{
						"foo": "bar",
					},
					"type": string(corev1.SecretTypeBasicAuth),
					"data": map[string]interface{}{
						corev1.BasicAuthUsernameKey: "foo",
						corev1.BasicAuthPasswordKey: "bar",
					},
				},
			},
			shouldNotRemoveFields: []field{
				{"metadata", "foo"},
				{"data", corev1.BasicAuthUsernameKey},
				{"data", corev1.BasicAuthPasswordKey},
			},
		},
		{
			name: "remove selected-node pvc annotation",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.PersistentVolumeClaimKind,
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							storagevolume.AnnSelectedNode: "node1",
						},
					},
				},
			},
			unexpectedFields: []field{
				{"metadata", "annotations", storagevolume.AnnSelectedNode},
			},
		},
		{
			name: "removes deployment revision annotation",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.DeploymentKind,
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							utildeployment.RevisionAnnotation: 1,
						},
					},
				},
			},
			unexpectedFields: []field{
				{"metadata", "annotations", utildeployment.RevisionAnnotation},
			},
		},
		{
			name: "removes deployment revision history annotation",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": util.DeploymentKind,
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							utildeployment.RevisionHistoryAnnotation: "1,2",
						},
					},
				},
			},
			unexpectedFields: []field{
				{"metadata", "annotations", utildeployment.RevisionHistoryAnnotation},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemoveIrrelevantFields(tt.workload, tt.extraHooks...); err != nil {
				t.Fatalf("RemoveIrrelevantField() expects no error but got: %v", err)
				return
			}

			unexpectedFields, err := getUnexpectedFields(tt)
			if err != nil {
				t.Fatal(err)
				return
			}
			if len(unexpectedFields) > 0 {
				t.Errorf("RemoveIrrelevantField() failed to remove irrelevant fields: %v", unexpectedFields)
			}

			shouldNotRemoveFields, err := getShouldNotRemoveFields(tt)
			if err != nil {
				t.Fatal(err)
				return
			}
			if len(shouldNotRemoveFields) > 0 {
				t.Errorf("RemoveIrrelevantField() should not remove those fields: %v", shouldNotRemoveFields)
			}
		})
	}
}

func getUnexpectedFields(t *test) ([]field, error) {
	var unexpectedFields []field
	for _, field := range t.unexpectedFields {
		val, found, err := unstructured.NestedFieldNoCopy(t.workload.Object, field...)
		if err != nil {
			return nil, fmt.Errorf("NestedFieldNoCopy() expect no error but got: %v", err)
		}

		if found {
			if t.containsFunc == nil || t.containsFunc(val, t.unexpectedResource) {
				unexpectedFields = append(unexpectedFields, field)
			}
		}
	}
	return unexpectedFields, nil
}

func getShouldNotRemoveFields(t *test) ([]field, error) {
	var shouldNotRemoveFields []field
	for _, field := range t.shouldNotRemoveFields {
		val, found, err := unstructured.NestedFieldNoCopy(t.workload.Object, field...)
		if err != nil {
			return nil, fmt.Errorf("NestedFieldNoCopy() expect no error but got: %v", err)
		}

		if !found || (t.containsFunc != nil && !t.containsFunc(val, t.shouldNotRemoveResource)) {
			shouldNotRemoveFields = append(shouldNotRemoveFields, field)
		}
	}
	return shouldNotRemoveFields, nil
}
