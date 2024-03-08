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
package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestExecutionController_Reconcile(t *testing.T) {
	tests := []struct {
		name      string
		c         Controller
		work      *workv1alpha1.Work
		ns        string
		expectRes controllerruntime.Result
		existErr  bool
	}{
		{
			name: "work dispatching is suspended, no error, no apply",
			c:    newController(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)),
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Spec: workv1alpha1.WorkSpec{
					SuspendDispatching: ptr.To(true),
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name:      "work",
					Namespace: tt.ns,
				},
			}

			if err := tt.c.Client.Create(context.Background(), tt.work); err != nil {
				t.Fatalf("Failed to create cluster: %v", err)
			}

			res, err := tt.c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectRes, res)
			if tt.existErr {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func newController(objects ...client.Object) Controller {
	return Controller{
		Client:          fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(objects...).Build(),
		InformerManager: genericmanager.GetInstance(),
		PredicateFunc:   helper.NewClusterPredicateOnAgent("test"),
	}
}

func newCluster(name string, clusterType string, clusterStatus metav1.ConditionStatus) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterv1alpha1.ClusterSpec{},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterType,
					Status: clusterStatus,
				},
			},
		},
	}
}
