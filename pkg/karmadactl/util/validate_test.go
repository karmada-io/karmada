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

package util

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestVerifyWhetherClustersExist(t *testing.T) {
	clusters := clusterv1alpha1.ClusterList{Items: []clusterv1alpha1.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "member1"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "member2"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "member3"},
		},
	}}
	tests := []struct {
		name     string
		input    []string
		clusters *clusterv1alpha1.ClusterList
		wantErr  error
	}{
		{
			name:     "input is nil",
			input:    nil,
			clusters: &clusters,
			wantErr:  nil,
		},
		{
			name:     "not exist",
			input:    []string{"member1", "member4"},
			clusters: &clusters,
			wantErr:  fmt.Errorf("clusters don't exist: member4"),
		},
		{
			name:     "exist",
			input:    []string{"member1"},
			clusters: &clusters,
			wantErr:  nil,
		},
		{
			name:     "clusterList is empty",
			input:    []string{"member1", "member2"},
			clusters: &clusterv1alpha1.ClusterList{Items: make([]clusterv1alpha1.Cluster, 0)},
			wantErr:  fmt.Errorf("clusters don't exist: member1,member2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isExist := VerifyClustersExist(tt.input, tt.clusters); !isErrorEqual(tt.wantErr, isExist) {
				t.Errorf("VerifyClustersExist want: %v, actually: %v", tt.wantErr, isExist)
			}
		})
	}
}

func isErrorEqual(want error, actual error) bool {
	if want == nil && actual == nil {
		return true
	}
	if want != nil && actual != nil && want.Error() == actual.Error() {
		return true
	}
	return false
}
