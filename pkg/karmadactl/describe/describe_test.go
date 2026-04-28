/*
Copyright 2024 The Karmada Authors.

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

package describe

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	describecmd "k8s.io/kubectl/pkg/describe"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

var deployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-deployment",
		Namespace: "default",
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "nginx"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image:           "nginx",
						ImagePullPolicy: "Always",
						Name:            "nginx",
					},
				},
			},
		},
	},
}

type ResourceDescribe struct{}

func (rd ResourceDescribe) Describe(string, string, describecmd.DescriberSettings) (output string, err error) {
	// Serialize the deployment response to JSON.
	bodyBytes, err := json.Marshal(deployment)
	if err != nil {
		return "", fmt.Errorf("failed to serialize deployment response: %v", err)
	}

	// Return the serialized deployment as a string (pretty-printed JSON).
	return string(bodyBytes), nil
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name         string
		describeOpts *CommandDescribeOptions
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "Validate_WithMemberOperationScopeAndWithoutCluster_MemberClusterOptMustBeSpecified",
			describeOpts: &CommandDescribeOptions{OperationScope: options.Members},
			wantErr:      true,
			errMsg:       "must specify a member cluster",
		},
		{
			name: "Validate__WithMemberOperationScopeAndWithCluster_Validated",
			describeOpts: &CommandDescribeOptions{
				OperationScope: options.Members,
				Cluster:        "test-cluster",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.describeOpts.Validate()
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}
