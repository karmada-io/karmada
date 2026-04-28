/*
Copyright 2025 The Karmada Authors.

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

package helper

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func Test_BuildPreservedLabelState(t *testing.T) {
	type args struct {
		statePreservation *policyv1alpha1.StatePreservation
		rawStatus         []byte
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "successful case",
			args: args{
				statePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
						{AliasLabelName: "key-b", JSONPath: "{ .health }"},
					},
				},
				rawStatus: []byte(`{"replicas": 2, "health": true}`),
			},
			wantErr: assert.NoError,
			want:    map[string]string{"key-a": "2", "key-b": "true"},
		},
		{
			name: "one statePreservation rule exist not found field",
			args: args{
				statePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
						{AliasLabelName: "key-b", JSONPath: "{ .notfound }"},
					},
				},
				rawStatus: []byte(`{"replicas": 2, "health": true}`),
			},
			wantErr: assert.Error,
			want:    nil,
		},
		{
			name: "one statePreservation rule has invalid jsonPath",
			args: args{
				statePreservation: &policyv1alpha1.StatePreservation{
					Rules: []policyv1alpha1.StatePreservationRule{
						{AliasLabelName: "key-a", JSONPath: "{ .replicas }"},
						{AliasLabelName: "key-b", JSONPath: "{ %health }"},
					},
				},
				rawStatus: []byte(`{"replicas": 2, "health": true}`),
			},
			wantErr: assert.Error,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildPreservedLabelState(tt.args.statePreservation, tt.args.rawStatus)
			if !tt.wantErr(t, err, fmt.Sprintf("buildPreservedLabelState(%v, %s)", tt.args.statePreservation, tt.args.rawStatus)) {
				return
			}
			assert.Equalf(t, tt.want, got, "buildPreservedLabelState(%v, %s)", tt.args.statePreservation, tt.args.rawStatus)
		})
	}
}

func Test_parseJSONValue(t *testing.T) {
	// This json value describes a DeploymentList object, it contains two deployment elements.
	var deploymentListStrBytes = []byte(`{"apiVersion":"v1","items":[{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"creationTimestamp":"2024-11-27T07:59:13Z","generation":2,"labels":{"app":"nginx","propagationpolicy.karmada.io/permanent-id":"89a95e21-57ec-4f5d-b8c6-15bc196c1449"},"name":"nginx-01","namespace":"default","resourceVersion":"1148","uid":"12cf1e9f-61fd-4e47-a14e-e844165c7f93"},"spec":{"progressDeadlineSeconds":600,"replicas":2,"revisionHistoryLimit":10,"selector":{"matchLabels":{"app":"nginx"}},"strategy":{"rollingUpdate":{"maxSurge":"25%","maxUnavailable":"25%"},"type":"RollingUpdate"},"template":{"metadata":{"creationTimestamp":null,"labels":{"app":"nginx"}},"spec":{"containers":[{"image":"nginx","imagePullPolicy":"Always","name":"nginx","resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File"}],"dnsPolicy":"ClusterFirst","restartPolicy":"Always","schedulerName":"default-scheduler","securityContext":{},"terminationGracePeriodSeconds":30}}},"status":{"availableReplicas":2,"observedGeneration":2,"readyReplicas":2,"replicas":2,"updatedReplicas":2}},{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"creationTimestamp":"2024-11-27T07:59:13Z","generation":2,"labels":{"app":"nginx","propagationpolicy.karmada.io/permanent-id":"89a95e21-57ec-4f5d-b8c6-15bc196c1449"},"name":"nginx-02","namespace":"default","resourceVersion":"1149","uid":"12cf1e9f-61fd-4e47-a14e-e844165c7f93"},"spec":{"progressDeadlineSeconds":600,"replicas":2,"revisionHistoryLimit":10,"selector":{"matchLabels":{"app":"nginx"}},"strategy":{"rollingUpdate":{"maxSurge":"25%","maxUnavailable":"25%"},"type":"RollingUpdate"},"template":{"metadata":{"creationTimestamp":null,"labels":{"app":"nginx"}},"spec":{"containers":[{"image":"nginx","imagePullPolicy":"Always","name":"nginx","resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File"}],"dnsPolicy":"ClusterFirst","restartPolicy":"Always","schedulerName":"default-scheduler","securityContext":{},"terminationGracePeriodSeconds":30}}},"status":{"availableReplicas":2,"observedGeneration":2,"readyReplicas":2,"replicas":2,"updatedReplicas":2}}],"kind":"List","metadata":{"resourceVersion":""}}`)
	type args struct {
		rawStatus []byte
		jsonPath  string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		// Build the following test cases from the perspective of parsing application state
		{
			name: "target field not found",
			args: args{
				rawStatus: []byte(`{"readyReplicas": 2}`),
				jsonPath:  "{ .replicas }",
			},
			wantErr: assert.Error,
		},
		{
			name: "invalid jsonPath",
			args: args{
				rawStatus: []byte(`{"readyReplicas": 2}`),
				jsonPath:  "{ %replicas }",
			},
			wantErr: assert.Error,
		},
		{
			name: "success to parse",
			args: args{
				rawStatus: []byte(`{"replicas": 2}`),
				jsonPath:  "{ .replicas }",
			},
			wantErr: assert.NoError,
			want:    "2",
		},
		// Build the following test cases in terms of what the function supports (which we don't use now).
		// Please refer to Function Support: https://kubernetes.io/docs/reference/kubectl/jsonpath/
		{
			name: "the current object parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{ @ }",
			},
			wantErr: assert.NoError,
			want:    string(deploymentListStrBytes),
		},
		{
			name: "child operator parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{ ['kind'] }",
			},
			wantErr: assert.NoError,
			want:    "List",
		},
		{
			name: "recursive descent parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{ ..resourceVersion }",
			},
			wantErr: assert.NoError,
			want:    "1148 1149",
		},
		{
			name: "wildcard get all objects parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{ .items[*].metadata.name }",
			},
			wantErr: assert.NoError,
			want:    "nginx-01 nginx-02",
		},
		{
			name: "subscript operator parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{ .items[0].metadata.name }",
			},
			wantErr: assert.NoError,
			want:    "nginx-01",
		},
		{
			name: "filter parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{ .items[?(@.metadata.name==\"nginx-01\")].metadata.name }",
			},
			wantErr: assert.NoError,
			want:    "nginx-01",
		},
		{
			name: "iterate list parse",
			args: args{
				rawStatus: deploymentListStrBytes,
				jsonPath:  "{range .items[*]}[{.metadata.name}, {.metadata.namespace}] {end}",
			},
			wantErr: assert.NoError,
			want:    "[nginx-01, default] [nginx-02, default]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJSONValue(tt.args.rawStatus, tt.args.jsonPath)
			if !tt.wantErr(t, err, fmt.Sprintf("parseJSONValue(%s, %v)", tt.args.rawStatus, tt.args.jsonPath)) {
				return
			}
			got = strings.Trim(got, " ")
			assert.Equalf(t, tt.want, got, "parseJSONValue(%s, %v)", tt.args.rawStatus, tt.args.jsonPath)
		})
	}
}
