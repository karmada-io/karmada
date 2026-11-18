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
	containsFunc            func(any, string) bool
}

func TestRemoveIrrelevantField(t *testing.T) {
	var tests = []*test{
		{
			name: "remove common object irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"creationTimestamp":          "2023-03-13T05:00:41Z",
						"deletionTimestamp":          "2023-03-13T06:00:41Z",
						"deletionGracePeriodSeconds": 10,
						"generation":                 2,
						"managedFields": []map[string]any{
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
						"ownerReferences": []map[string]any{
							{
								"apiVersion": "v1",
								"kind":       "Pod",
								"name":       "foo",
								"uid":        "fb11a9a6-1daa-265b-c046-1c1dea42a42c",
							},
						},
						"finalizers": []string{"foregroundDeletion"},
					},
					"status": map[string]any{},
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
				Object: map[string]any{
					"kind": util.ServiceKind,
					"spec": map[string]any{
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
			name: "remove service nodePort from spec.ports[*] with multiple ports",
			workload: &unstructured.Unstructured{
				Object: map[string]any{
					"kind": util.ServiceKind,
					"spec": map[string]any{
						"type":       "NodePort",
						"clusterIP":  "10.10.10.10",
						"clusterIPs": []string{"10.10.10.10"},
						"ports": []any{
							map[string]any{
								"name":       "http",
								"protocol":   "TCP",
								"port":       int64(80),
								"targetPort": int64(80),
								"nodePort":   int64(32410),
							},
							map[string]any{
								"name":       "https",
								"protocol":   "TCP",
								"port":       int64(443),
								"targetPort": int64(443),
								"nodePort":   int64(32412),
							},
						},
					},
				},
			},
			unexpectedFields: []field{
				{"spec", "ports"},
			},
			unexpectedResource: "nodePort",
			shouldNotRemoveFields: []field{
				{"spec", "ports"},
			},
			shouldNotRemoveResource: "name",
			containsFunc:            portContainsField,
		},
		{
			name: "remove service nodePort from spec.ports[*] with single port",
			workload: &unstructured.Unstructured{
				Object: map[string]any{
					"kind": util.ServiceKind,
					"spec": map[string]any{
						"type":       "NodePort",
						"clusterIP":  "10.10.10.10",
						"clusterIPs": []string{"10.10.10.10"},
						"ports": []any{
							map[string]any{
								"name":       "http",
								"protocol":   "TCP",
								"port":       int64(80),
								"targetPort": int64(80),
								"nodePort":   int64(30080),
							},
						},
					},
				},
			},
			unexpectedFields: []field{
				{"spec", "ports"},
			},
			unexpectedResource: "nodePort",
			shouldNotRemoveFields: []field{
				{"spec", "ports"},
			},
			shouldNotRemoveResource: "name",
			containsFunc:            portContainsField,
		},
		{
			name: "headless service preserved when stripping nodePort",
			workload: &unstructured.Unstructured{
				Object: map[string]any{
					"kind": util.ServiceKind,
					"spec": map[string]any{
						"type":      "ClusterIP",
						"clusterIP": "None",
						"ports": []any{
							map[string]any{
								"name":       "http",
								"protocol":   "TCP",
								"port":       int64(80),
								"targetPort": int64(80),
							},
						},
					},
				},
			},
			unexpectedFields: []field{
				{"spec", "ports"},
			},
			unexpectedResource: "nodePort",
			shouldNotRemoveFields: []field{
				{"spec", "ports"},
			},
			shouldNotRemoveResource: "name",
			containsFunc:            portContainsField,
		},
		{
			name: "remove job irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]any{
					"kind": util.JobKind,
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"foo":            "bar",
								"controller-uid": "ab11a9a6-1daa-265b-c046-1c1dea42a42c",
							},
						},
						"template": map[string]any{
							"metadata": map[string]any{
								"labels": map[string]any{
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
				Object: map[string]any{
					"kind": util.ServiceAccountKind,
					"metadata": map[string]any{
						"name": "foo",
					},
					"secrets": []any{
						map[string]any{
							"name": "foo-token-6pgxf",
						},
						map[string]any{
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
			containsFunc:            namedEntryContains,
		},
		{
			name: "remove service-account token secret irrelevant fields",
			workload: &unstructured.Unstructured{
				Object: map[string]any{
					"kind": util.SecretKind,
					"metadata": map[string]any{
						corev1.ServiceAccountUIDKey: "123",
					},
					"type": string(corev1.SecretTypeServiceAccountToken),
					"data": map[string]any{
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
				Object: map[string]any{
					"kind": util.SecretKind,
					"metadata": map[string]any{
						"foo": "bar",
					},
					"type": string(corev1.SecretTypeBasicAuth),
					"data": map[string]any{
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
				Object: map[string]any{
					"kind": util.PersistentVolumeClaimKind,
					"metadata": map[string]any{
						"annotations": map[string]any{
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
				Object: map[string]any{
					"kind": util.DeploymentKind,
					"metadata": map[string]any{
						"annotations": map[string]any{
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
				Object: map[string]any{
					"kind": util.DeploymentKind,
					"metadata": map[string]any{
						"annotations": map[string]any{
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

// portContainsField reports whether obj is a slice of port-maps and at least
// one of them has the given resource as a key. Used by the Service nodePort
// cases to check that a field was (or was not) stripped from spec.ports[*].
func portContainsField(obj any, resource string) bool {
	ports, ok := obj.([]any)
	if !ok {
		return false
	}
	for _, p := range ports {
		m, _ := p.(map[string]any)
		if _, ok := m[resource]; ok {
			return true
		}
	}
	return false
}

// namedEntryContains reports whether obj is a slice of maps and at least one
// has its "name" field equal to resource. Used by the ServiceAccount case to
// check that a specific named entry was (or was not) removed from .secrets.
func namedEntryContains(obj any, resource string) bool {
	entries, _ := obj.([]any)
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok && m["name"] == resource {
			return true
		}
	}
	return false
}
