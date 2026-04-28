/*
Copyright 2020 The Kubernetes Authors.

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

// This code is directly lifted from the kubefed codebase.
// For reference:
// https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain_test.go#L91-L292

package lifted

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain_test.go#L91-L174
// +lifted:changed

func TestRetainHealthCheckNodePortInServiceFields(t *testing.T) {
	tests := []struct {
		name          string
		desiredObj    *unstructured.Unstructured
		clusterObj    *unstructured.Unstructured
		retainSucceed bool
		expectedValue *int64
	}{
		{
			"cluster object has no healthCheckNodePort",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			true,
			nil,
		},
		{
			"cluster object has invalid healthCheckNodePort",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"healthCheckNodePort": "invalid string",
					},
				},
			},
			false,
			nil,
		},
		{
			"cluster object has healthCheckNodePort 0",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"healthCheckNodePort": int64(0),
					},
				},
			},
			true,
			nil,
		},
		{
			"cluster object has healthCheckNodePort 1000",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"healthCheckNodePort": int64(1000),
					},
				},
			},
			true,
			ptr.To[int64](1000),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := RetainServiceFields(test.desiredObj, test.clusterObj); (err == nil) != test.retainSucceed {
				t.Fatalf("test %s fails: unexpected returned error %v", test.name, err)
			}

			currentValue, ok, err := unstructured.NestedInt64(test.desiredObj.Object, "spec", "healthCheckNodePort")
			if err != nil {
				t.Fatalf("test %s fails: %v", test.name, err)
			}
			if !ok && test.expectedValue != nil {
				t.Fatalf("test %s fails: expect specified healthCheckNodePort but not found", test.name)
			}
			if ok && (test.expectedValue == nil || *test.expectedValue != currentValue) {
				t.Fatalf("test %s fails: unexpected current healthCheckNodePort %d", test.name, currentValue)
			}
		})
	}
}

// +lifted:source=https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/sync/dispatch/retain_test.go#L176-L292
// +lifted:changed

func TestRetainClusterIPInServiceFields(t *testing.T) {
	tests := []struct {
		name                   string
		desiredObj             *unstructured.Unstructured
		clusterObj             *unstructured.Unstructured
		retainSucceed          bool
		expectedClusterIPValue *string
	}{
		{
			"cluster object has no clusterIP",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			true,
			nil,
		},
		{
			"cluster object has invalid clusterIP",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"clusterIP": -1000,
					},
				},
			},
			false,
			nil,
		},
		{
			"cluster object has valid clusterIP",
			&unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"clusterIP": "1.2.3.4",
					},
				},
			},
			true,
			ptr.To("1.2.3.4"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := RetainServiceFields(test.desiredObj, test.clusterObj); (err == nil) != test.retainSucceed {
				t.Fatalf("test %s fails: unexpected returned error %v", test.name, err)
			}

			currentClusterIPValue, ok, err := unstructured.NestedString(test.desiredObj.Object, "spec", "clusterIP")
			if err != nil {
				t.Fatalf("test %s fails: %v", test.name, err)
			}
			if !ok && test.expectedClusterIPValue != nil {
				t.Fatalf("test %s fails: expect specified clusterIP but not found", test.name)
			}
			if ok && (test.expectedClusterIPValue == nil || *test.expectedClusterIPValue != currentClusterIPValue) {
				t.Fatalf("test %s fails: unexpected current clusterIP %s", test.name, currentClusterIPValue)
			}
		})
	}
}

func TestRetainServiceAccountFields(t *testing.T) {
	tests := []struct {
		name                          string
		desiredObj                    *unstructured.Unstructured
		observedObj                   *unstructured.Unstructured
		expectedErr                   bool
		expectedSecretsValue          []interface{}
		expectedImagePullSecretsValue []interface{}
	}{
		{
			name: "neither desired or observed service account has the secrets field",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			expectedErr:          false,
			expectedSecretsValue: nil,
		},
		{
			name: "both desired and observed service account have the same secrets field",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
			expectedErr: false,
			expectedSecretsValue: []interface{}{
				map[string]interface{}{
					"name": "test",
				},
			},
			expectedImagePullSecretsValue: nil,
		},
		{
			name: "desired and observed service account have the different secrets field",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
						map[string]interface{}{
							"name": "test-token",
						},
					},
				},
			},
			expectedErr: false,
			expectedSecretsValue: []interface{}{
				map[string]interface{}{
					"name": "test",
				},
				map[string]interface{}{
					"name": "test-token",
				},
			},
			expectedImagePullSecretsValue: nil,
		},
		{
			name: "observed service account has the imagePullSecrets field but the desired not",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"secrets": []interface{}{
						map[string]interface{}{
							"name": "test",
						},
					},
					"imagePullSecrets": []interface{}{
						map[string]interface{}{
							"name": "foo",
						},
					},
				},
			},
			expectedErr: false,
			expectedSecretsValue: []interface{}{
				map[string]interface{}{
					"name": "test",
				},
			},
			expectedImagePullSecretsValue: []interface{}{
				map[string]interface{}{
					"name": "foo",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			desiredObj, err := RetainServiceAccountFields(test.desiredObj, test.observedObj)
			if (err != nil) != test.expectedErr {
				t.Fatalf("unexpected returned error %v", err)
			}

			if err == nil {
				table := []struct {
					name          string
					field         string
					expectedValue []interface{}
				}{
					{name: "secrets", field: SecretsField, expectedValue: test.expectedSecretsValue},
					{name: "imagePullSecrets", field: ImagePullSecretsField, expectedValue: test.expectedImagePullSecretsValue},
				}

				for _, entry := range table {
					currentValue, ok, err := unstructured.NestedSlice(desiredObj.Object, entry.field)
					if err != nil {
						t.Fatalf("failed to get %q field from serviceaccount: %v", entry.name, err)
					}
					if !ok && entry.expectedValue != nil {
						t.Fatalf("expected %q %v but not found", entry.name, entry.expectedValue)
					}
					if ok && !reflect.DeepEqual(entry.expectedValue, currentValue) {
						t.Fatalf("unexpected %q: got %v, want %v", entry.name, currentValue, entry.expectedValue)
					}
				}
			}
		})
	}
}
