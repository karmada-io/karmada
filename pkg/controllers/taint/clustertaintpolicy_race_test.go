/*
Copyright 2026 The Karmada Authors.

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

package taint

import (
	"context"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

// TestReconcilePatchRejectsStaleData verifies that ClusterTaintPolicyController's
// Patch uses optimistic locking so that a concurrent taint modification (e.g. by
// cluster-controller adding a health taint) causes a conflict error instead of
// silently overwriting the taints array.
func TestReconcilePatchRejectsStaleData(t *testing.T) {
	ctx := context.Background()
	now := metav1.Now()

	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "member1",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: clusterv1alpha1.Push,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: now,
					Reason:             "ClusterNotReady",
				},
			},
		},
	}

	policy := &policyv1alpha1.ClusterTaintPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-policy",
			CreationTimestamp: now,
		},
		Spec: policyv1alpha1.ClusterTaintPolicySpec{
			Taints: []policyv1alpha1.Taint{
				{
					Key:    "custom/unhealthy",
					Value:  "true",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			AddOnConditions: []policyv1alpha1.MatchCondition{
				{
					ConditionType: "Ready",
					Operator:      policyv1alpha1.MatchConditionOpIn,
					StatusValues:  []metav1.ConditionStatus{metav1.ConditionFalse},
				},
			},
		},
	}

	// Inject a concurrent taint modification between the controller's Get and Patch.
	var mu sync.Mutex
	getCalled := false

	interceptFuncs := interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			mu.Lock()
			alreadyCalled := getCalled
			mu.Unlock()

			if alreadyCalled {
				// Simulate cluster-controller adding NotReady taint between Get and Patch.
				clusterObj := &clusterv1alpha1.Cluster{}
				if err := c.Get(ctx, types.NamespacedName{Name: "member1"}, clusterObj); err == nil {
					clusterObj.Spec.Taints = append(clusterObj.Spec.Taints, corev1.Taint{
						Key:    "cluster.karmada.io/not-ready",
						Effect: corev1.TaintEffectNoSchedule,
					})
					_ = c.Update(ctx, clusterObj)
				}
			}

			return c.Patch(ctx, obj, patch, opts...)
		},
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			err := c.Get(ctx, key, obj, opts...)
			if key.Name == "member1" {
				mu.Lock()
				getCalled = true
				mu.Unlock()
			}
			return err
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(gclient.NewSchema()).
		WithObjects(cluster, policy).
		WithStatusSubresource(&clusterv1alpha1.Cluster{}).
		WithInterceptorFuncs(interceptFuncs).
		Build()

	controller := &ClusterTaintPolicyController{
		Client:        fakeClient,
		EventRecorder: record.NewFakeRecorder(1024),
	}

	_, err := controller.Reconcile(ctx, controllerruntime.Request{
		NamespacedName: types.NamespacedName{Name: "member1"},
	})

	// With optimistic locking the patch must fail with a conflict error,
	// which controller-runtime will requeue automatically.
	if err == nil {
		t.Fatal("expected conflict error from optimistic lock, but Reconcile succeeded — " +
			"this means MergeFrom is not using optimistic locking and concurrent taint changes can be silently lost")
	}
	t.Logf("Reconcile correctly returned conflict error: %v", err)
}
