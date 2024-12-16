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

package util

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util"
)

var testFinalizer = "test.io/test"

func TestRemoveWorkFinalizer(t *testing.T) {
	testItems := []struct {
		name           string
		executionSpace string
		workList       []runtime.Object
		wantErr        bool
		expect         map[string][]string
	}{
		{
			name:           "empty work list",
			executionSpace: "test",
			workList:       []runtime.Object{},
			wantErr:        false,
			expect:         nil,
		},
		{
			name:           "work without finalizer",
			executionSpace: "test",
			workList: []runtime.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "test",
					},
				},
			},
			wantErr: false,
			expect:  map[string][]string{"work1": {}},
		},
		{
			name:           "work with ExecutionControllerFinalizer",
			executionSpace: "test",
			workList: []runtime.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "work1",
						Namespace:  "test",
						Finalizers: []string{util.ExecutionControllerFinalizer, testFinalizer},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "work2",
						Namespace:  "test",
						Finalizers: []string{util.ExecutionControllerFinalizer},
					},
				},
			},
			wantErr: false,
			expect:  map[string][]string{"work1": {testFinalizer}, "work2": {}},
		},
	}
	for _, tt := range testItems {
		t.Run(tt.name, func(t *testing.T) {
			controlPlaneKarmadaClient := fakekarmadaclient.NewSimpleClientset(tt.workList...)
			err := RemoveWorkFinalizer(tt.executionSpace, controlPlaneKarmadaClient)
			assert.Equal(t, tt.wantErr, err != nil)

			for name, finalizers := range tt.expect {
				work, err := controlPlaneKarmadaClient.WorkV1alpha1().Works(tt.executionSpace).Get(context.TODO(), name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				assert.ElementsMatch(t, finalizers, work.GetFinalizers())
			}
		})
	}
}

func TestRemoveExecutionSpaceFinalizer(t *testing.T) {
	testItems := []struct {
		name             string
		executionSpace   corev1.Namespace
		wantErr          bool
		expectFinalizers []string
	}{
		{
			name: "executionSpace without finalizer",
			executionSpace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace1",
				},
			},
			wantErr:          false,
			expectFinalizers: []string{},
		},
		{
			name: "executionSpace with FinalizerKubernetes",
			executionSpace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "namespace1",
					Finalizers: []string{string(corev1.FinalizerKubernetes), testFinalizer},
				},
			},
			wantErr:          false,
			expectFinalizers: []string{testFinalizer},
		},
	}
	for _, tt := range testItems {
		t.Run(tt.name, func(t *testing.T) {
			controlPlaneKubeClient := fake.NewClientset(&tt.executionSpace)
			err := RemoveExecutionSpaceFinalizer(tt.executionSpace.Name, controlPlaneKubeClient)
			assert.Equal(t, tt.wantErr, err != nil)

			namespace, err := controlPlaneKubeClient.CoreV1().Namespaces().Get(context.TODO(), tt.executionSpace.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			assert.ElementsMatch(t, tt.expectFinalizers, namespace.GetFinalizers())
		})
	}
}

func TestRemoveClusterFinalizer(t *testing.T) {
	testItems := []struct {
		name             string
		cluster          clusterv1alpha1.Cluster
		wantErr          bool
		expectFinalizers []string
	}{
		{
			name: "cluster without finalizer",
			cluster: clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			wantErr:          false,
			expectFinalizers: []string{},
		},
		{
			name: "cluster with ClusterControllerFinalizer",
			cluster: clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "cluster1",
					Finalizers: []string{util.ClusterControllerFinalizer, testFinalizer},
				},
			},
			wantErr:          false,
			expectFinalizers: []string{testFinalizer},
		},
	}
	for _, tt := range testItems {
		t.Run(tt.name, func(t *testing.T) {
			controlPlaneKarmadaClient := fakekarmadaclient.NewSimpleClientset(&tt.cluster)
			err := RemoveClusterFinalizer(tt.cluster.Name, controlPlaneKarmadaClient)
			assert.Equal(t, tt.wantErr, err != nil)

			cluster, err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), tt.cluster.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			assert.ElementsMatch(t, tt.expectFinalizers, cluster.GetFinalizers())
		})
	}
}
