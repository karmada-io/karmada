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

package framework

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestNewClusterInfo(t *testing.T) {
	testCases := []struct {
		name    string
		cluster *clusterv1alpha1.Cluster
		want    *ClusterInfo
	}{
		{
			name: "Create ClusterInfo with valid cluster",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
			want: &ClusterInfo{
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
		},
		{
			name:    "Create ClusterInfo with nil cluster",
			cluster: nil,
			want:    &ClusterInfo{cluster: nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewClusterInfo(tc.cluster)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClusterInfo_Cluster(t *testing.T) {
	testCases := []struct {
		name        string
		clusterInfo *ClusterInfo
		want        *clusterv1alpha1.Cluster
	}{
		{
			name: "Get cluster from valid ClusterInfo",
			clusterInfo: &ClusterInfo{
				cluster: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				},
			},
			want: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
			},
		},
		{
			name:        "Get cluster from nil ClusterInfo",
			clusterInfo: nil,
			want:        nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.clusterInfo.Cluster()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFitError_Error(t *testing.T) {
	testCases := []struct {
		name            string
		fitError        FitError
		expectedOutput  string
		expectedReasons []string
	}{
		{
			name: "No clusters available",
			fitError: FitError{
				NumAllClusters: 0,
				Diagnosis:      Diagnosis{ClusterToResultMap: ClusterToResultMap{}},
			},
			expectedOutput:  "0/0 clusters are available: no cluster exists.",
			expectedReasons: []string{},
		},
		{
			name: "Multiple reasons for unavailability",
			fitError: FitError{
				NumAllClusters: 3,
				Diagnosis: Diagnosis{
					ClusterToResultMap: ClusterToResultMap{
						"cluster1": &Result{reasons: []string{"insufficient CPU", "insufficient memory"}},
						"cluster2": &Result{reasons: []string{"insufficient CPU"}},
						"cluster3": &Result{reasons: []string{"taint mismatch"}},
					},
				},
			},
			expectedOutput: "0/3 clusters are available:",
			expectedReasons: []string{
				"2 insufficient CPU",
				"1 insufficient memory",
				"1 taint mismatch",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fitError.Error()

			// Check if the error message starts with the expected output
			assert.True(t, strings.HasPrefix(got, tc.expectedOutput), "Error message should start with expected output")

			if len(tc.expectedReasons) > 0 {
				// Check each reason
				for _, reason := range tc.expectedReasons {
					assert.Contains(t, got, reason, "Error message should contain the reason: %s", reason)
				}

				// Check the total number of reasons
				gotReasons := strings.Split(strings.TrimPrefix(got, tc.expectedOutput), ",")
				assert.Equal(t, len(tc.expectedReasons), len(gotReasons), "Number of reasons should match")
			} else {
				// If no reasons are expected, the got message should exactly match the expected output
				assert.Equal(t, tc.expectedOutput, got, "Error message should exactly match expected output when no reasons are provided")
			}
		})
	}
}

func TestUnschedulableError_Error(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{
			name:    "Unschedulable due to insufficient resources",
			message: "Insufficient CPU in all clusters",
		},
		{
			name:    "Unschedulable due to taint mismatch",
			message: "No cluster matches required tolerations",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			unschedulableErr := UnschedulableError{Message: tc.message}
			assert.Equal(t, tc.message, unschedulableErr.Error())
		})
	}
}
