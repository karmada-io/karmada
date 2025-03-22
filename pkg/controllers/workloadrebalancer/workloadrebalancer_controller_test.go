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

package workloadrebalancer

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/helper"
)

var (
	now        = metav1.Now()
	oneHourAgo = metav1.NewTime(time.Now().Add(-1 * time.Hour))

	deploy1    = helper.NewDeployment("test-ns", fmt.Sprintf("test-1-%s", randomSuffix()))
	binding1   = newResourceBinding(deploy1)
	deploy1Obj = newObjectReference(deploy1)

	// use deploy2 to mock a resource whose resource-binding not found.
	deploy2    = helper.NewDeployment("test-ns", fmt.Sprintf("test-2-%s", randomSuffix()))
	deploy2Obj = newObjectReference(deploy2)

	deploy3    = helper.NewDeployment("test-ns", fmt.Sprintf("test-3-%s", randomSuffix()))
	binding3   = newResourceBinding(deploy3)
	deploy3Obj = newObjectReference(deploy3)

	pendingRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("rebalancer-with-pending-workloads-%s", randomSuffix()), CreationTimestamp: now},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			// Put deploy2Obj before deploy1Obj to test whether the results of status are sorted.
			Workloads: []appsv1alpha1.ObjectReference{deploy2Obj, deploy1Obj},
		},
	}
	succeedRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("rebalancer-with-succeed-workloads-%s", randomSuffix()), CreationTimestamp: oneHourAgo},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			Workloads: []appsv1alpha1.ObjectReference{deploy1Obj},
		},
		Status: appsv1alpha1.WorkloadRebalancerStatus{
			ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
				{
					Workload: deploy1Obj,
					Result:   appsv1alpha1.RebalanceSuccessful,
				},
			},
		},
	}
	notFoundRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("rebalancer-with-workloads-whose-binding-not-found-%s", randomSuffix()), CreationTimestamp: now},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			Workloads: []appsv1alpha1.ObjectReference{deploy2Obj},
		},
		Status: appsv1alpha1.WorkloadRebalancerStatus{
			ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
				{
					Workload: deploy2Obj,
					Result:   appsv1alpha1.RebalanceFailed,
					Reason:   appsv1alpha1.RebalanceObjectNotFound,
				},
			},
		},
	}
	failedRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("rebalancer-with-failed-workloads-%s", randomSuffix()), CreationTimestamp: now},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			Workloads: []appsv1alpha1.ObjectReference{deploy1Obj},
		},
		Status: appsv1alpha1.WorkloadRebalancerStatus{
			ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
				{
					// failed workload doesn't have a `Result` field and continue to retry.
					Workload: deploy1Obj,
				},
			},
		},
	}
	modifiedRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("rebalancer-which-experienced-modification-%s", randomSuffix()), CreationTimestamp: oneHourAgo},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			Workloads: []appsv1alpha1.ObjectReference{deploy3Obj},
		},
		Status: appsv1alpha1.WorkloadRebalancerStatus{
			ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
				{
					Workload: deploy1Obj,
					Result:   appsv1alpha1.RebalanceSuccessful,
				},
				{
					Workload: deploy2Obj,
					Result:   appsv1alpha1.RebalanceFailed,
					Reason:   appsv1alpha1.RebalanceObjectNotFound,
				},
			},
		},
	}
	ttlFinishedRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ttl-finished-rebalancer-%s", randomSuffix()), CreationTimestamp: oneHourAgo},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			TTLSecondsAfterFinished: ptr.To[int32](5),
			Workloads:               []appsv1alpha1.ObjectReference{deploy1Obj},
		},
		Status: appsv1alpha1.WorkloadRebalancerStatus{
			FinishTime: &oneHourAgo,
			ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
				{
					Workload: deploy1Obj,
					Result:   appsv1alpha1.RebalanceSuccessful,
				},
			},
		},
	}

	clusterRole = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("test-cluster-role-%s", randomSuffix())},
	}
	clusterBinding = newClusterResourceBinding(clusterRole)
	clusterRoleObj = newClusterRoleObjectReference(clusterRole)

	clusterRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("cluster-rebalancer-%s", randomSuffix()), CreationTimestamp: now},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			Workloads: []appsv1alpha1.ObjectReference{clusterRoleObj},
		},
	}
)

func TestRebalancerController_Reconcile(t *testing.T) {
	tests := []struct {
		name                string
		req                 controllerruntime.Request
		existObjects        []client.Object
		existObjsWithStatus []client.Object
		wantErr             bool
		wantStatus          appsv1alpha1.WorkloadRebalancerStatus
		needsCleanup        bool
	}{
		{
			name: "reconcile pendingRebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: pendingRebalancer.Name},
			},
			existObjects:        []client.Object{deploy1, binding1, pendingRebalancer},
			existObjsWithStatus: []client.Object{pendingRebalancer},
			wantStatus: appsv1alpha1.WorkloadRebalancerStatus{
				ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
					{
						Workload: deploy1Obj,
						Result:   appsv1alpha1.RebalanceSuccessful,
					},
					{
						Workload: deploy2Obj,
						Result:   appsv1alpha1.RebalanceFailed,
						Reason:   appsv1alpha1.RebalanceObjectNotFound,
					},
				},
			},
		},
		{
			name: "reconcile succeedRebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: succeedRebalancer.Name},
			},
			existObjects:        []client.Object{deploy1, binding1, succeedRebalancer},
			existObjsWithStatus: []client.Object{succeedRebalancer},
			wantStatus: appsv1alpha1.WorkloadRebalancerStatus{
				ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
					{
						Workload: deploy1Obj,
						Result:   appsv1alpha1.RebalanceSuccessful,
					},
				},
			},
		},
		{
			name: "reconcile notFoundRebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: notFoundRebalancer.Name},
			},
			existObjects:        []client.Object{notFoundRebalancer},
			existObjsWithStatus: []client.Object{notFoundRebalancer},
			wantStatus: appsv1alpha1.WorkloadRebalancerStatus{
				ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
					{
						Workload: deploy2Obj,
						Result:   appsv1alpha1.RebalanceFailed,
						Reason:   appsv1alpha1.RebalanceObjectNotFound,
					},
				},
			},
		},
		{
			name: "reconcile failedRebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: failedRebalancer.Name},
			},
			existObjects:        []client.Object{deploy1, binding1, failedRebalancer},
			existObjsWithStatus: []client.Object{failedRebalancer},
			wantStatus: appsv1alpha1.WorkloadRebalancerStatus{
				ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
					{
						Workload: deploy1Obj,
						Result:   appsv1alpha1.RebalanceSuccessful,
					},
				},
			},
		},
		{
			name: "reconcile modifiedRebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: modifiedRebalancer.Name},
			},
			existObjects:        []client.Object{deploy1, deploy3, binding1, binding3, modifiedRebalancer},
			existObjsWithStatus: []client.Object{modifiedRebalancer},
			wantStatus: appsv1alpha1.WorkloadRebalancerStatus{
				ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
					{
						Workload: deploy1Obj,
						Result:   appsv1alpha1.RebalanceSuccessful,
					},
					{
						Workload: deploy3Obj,
						Result:   appsv1alpha1.RebalanceSuccessful,
					},
				},
			},
		},
		{
			name: "reconcile ttlFinishedRebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: ttlFinishedRebalancer.Name},
			},
			existObjects:        []client.Object{deploy1, binding1, ttlFinishedRebalancer},
			existObjsWithStatus: []client.Object{ttlFinishedRebalancer},
			needsCleanup:        true,
		},
		{
			name: "reconcile cluster-wide resource rebalancer",
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: clusterRebalancer.Name},
			},
			existObjects:        []client.Object{clusterRole, clusterBinding, clusterRebalancer},
			existObjsWithStatus: []client.Object{clusterRebalancer},
			wantStatus: appsv1alpha1.WorkloadRebalancerStatus{
				ObservedWorkloads: []appsv1alpha1.ObservedWorkload{
					{
						Workload: clusterRoleObj,
						Result:   appsv1alpha1.RebalanceSuccessful,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runRebalancerTest(t, tt)
		})
	}
}

func runRebalancerTest(t *testing.T, tt struct {
	name                string
	req                 controllerruntime.Request
	existObjects        []client.Object
	existObjsWithStatus []client.Object
	wantErr             bool
	wantStatus          appsv1alpha1.WorkloadRebalancerStatus
	needsCleanup        bool
}) {
	c := &RebalancerController{
		Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).
			WithObjects(tt.existObjects...).
			WithStatusSubresource(tt.existObjsWithStatus...).Build(),
	}
	_, err := c.Reconcile(context.TODO(), tt.req)
	// 1. check whether it has error
	if (err != nil) != tt.wantErr {
		t.Fatalf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
	}

	// 2. check final WorkloadRebalancer status
	rebalancerGet := &appsv1alpha1.WorkloadRebalancer{}
	err = c.Client.Get(context.TODO(), tt.req.NamespacedName, rebalancerGet)
	if err != nil {
		if apierrors.IsNotFound(err) && tt.needsCleanup {
			t.Logf("WorkloadRebalancer %s has been cleaned up as expected", tt.req.NamespacedName)
			return
		}
		t.Fatalf("get WorkloadRebalancer failed: %+v", err)
	}

	tt.wantStatus.FinishTime = rebalancerGet.Status.FinishTime
	if rebalancerGet.Status.FinishTime == nil {
		// If FinishTime is nil, set it to a non-nil value for comparison
		now := metav1.Now()
		tt.wantStatus.FinishTime = &now
		rebalancerGet.Status.FinishTime = &now
	}
	if !reflect.DeepEqual(rebalancerGet.Status, tt.wantStatus) {
		t.Fatalf("update WorkloadRebalancer failed, got: %+v, want: %+v", rebalancerGet.Status, tt.wantStatus)
	}

	// 3. check binding's rescheduleTriggeredAt
	checkBindings(t, c, rebalancerGet)
}

func checkBindings(t *testing.T, c *RebalancerController, rebalancerGet *appsv1alpha1.WorkloadRebalancer) {
	for _, item := range rebalancerGet.Status.ObservedWorkloads {
		if item.Result != appsv1alpha1.RebalanceSuccessful {
			continue
		}
		if item.Workload.Namespace == "" {
			// This is a cluster-wide resource
			checkClusterBinding(t, c, item, rebalancerGet)
		} else {
			// This is a namespace-scoped resource
			checkResourceBinding(t, c, item, rebalancerGet)
		}
	}
}

func checkClusterBinding(t *testing.T, c *RebalancerController, item appsv1alpha1.ObservedWorkload, rebalancerGet *appsv1alpha1.WorkloadRebalancer) {
	clusterBindingGet := &workv1alpha2.ClusterResourceBinding{}
	clusterBindingName := names.GenerateBindingName(item.Workload.Kind, item.Workload.Name)
	err := c.Client.Get(context.TODO(), client.ObjectKey{Name: clusterBindingName}, clusterBindingGet)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("get cluster binding (%s) failed: %+v", clusterBindingName, err)
		}
		return // Skip the check if the binding is not found
	}
	if !clusterBindingGet.Spec.RescheduleTriggeredAt.Equal(&rebalancerGet.CreationTimestamp) {
		t.Fatalf("rescheduleTriggeredAt of cluster binding got: %+v, want: %+v", clusterBindingGet.Spec.RescheduleTriggeredAt, rebalancerGet.CreationTimestamp)
	}
}

func checkResourceBinding(t *testing.T, c *RebalancerController, item appsv1alpha1.ObservedWorkload, rebalancerGet *appsv1alpha1.WorkloadRebalancer) {
	bindingGet := &workv1alpha2.ResourceBinding{}
	bindingName := names.GenerateBindingName(item.Workload.Kind, item.Workload.Name)
	err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: item.Workload.Namespace, Name: bindingName}, bindingGet)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			t.Fatalf("get binding (%s) failed: %+v", bindingName, err)
		}
		return // Skip the check if the binding is not found
	}
	if !bindingGet.Spec.RescheduleTriggeredAt.Equal(&rebalancerGet.CreationTimestamp) {
		t.Fatalf("rescheduleTriggeredAt of binding got: %+v, want: %+v", bindingGet.Spec.RescheduleTriggeredAt, rebalancerGet.CreationTimestamp)
	}
}

func TestRebalancerController_updateWorkloadRebalancerStatus(t *testing.T) {
	tests := []struct {
		name               string
		rebalancer         *appsv1alpha1.WorkloadRebalancer
		modifiedRebalancer *appsv1alpha1.WorkloadRebalancer
		wantErr            bool
	}{
		{
			name:               "add newStatus to pendingRebalancer",
			rebalancer:         pendingRebalancer,
			modifiedRebalancer: succeedRebalancer,
			wantErr:            false,
		},
		{
			name:               "update status of failedRebalancer to newStatus",
			rebalancer:         failedRebalancer,
			modifiedRebalancer: succeedRebalancer,
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &RebalancerController{
				Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(tt.rebalancer, tt.modifiedRebalancer).
					WithStatusSubresource(tt.rebalancer, tt.modifiedRebalancer).Build(),
			}
			wantStatus := tt.modifiedRebalancer.Status
			err := c.updateWorkloadRebalancerStatus(context.TODO(), tt.rebalancer, &wantStatus)
			if (err == nil && tt.wantErr) || (err != nil && !tt.wantErr) {
				t.Fatalf("updateWorkloadRebalancerStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			rebalancerGet := &appsv1alpha1.WorkloadRebalancer{}
			if err := c.Client.Get(context.TODO(), client.ObjectKey{Name: tt.rebalancer.Name}, rebalancerGet); err != nil {
				t.Fatalf("get WorkloadRebalancer failed: %+v", err)
			}
			if !reflect.DeepEqual(rebalancerGet.Status, wantStatus) {
				t.Fatalf("update WorkloadRebalancer failed, got: %+v, want: %+v", rebalancerGet.Status, wantStatus)
			}
		})
	}
}

func newResourceBinding(obj *appsv1.Deployment) *workv1alpha2.ResourceBinding {
	return &workv1alpha2.ResourceBinding{
		TypeMeta:   metav1.TypeMeta{Kind: "work.karmada.io/v1alpha2", APIVersion: "ResourceBinding"},
		ObjectMeta: metav1.ObjectMeta{Namespace: obj.Namespace, Name: names.GenerateBindingName(obj.Kind, obj.Name)},
		Spec:       workv1alpha2.ResourceBindingSpec{RescheduleTriggeredAt: &oneHourAgo},
		Status:     workv1alpha2.ResourceBindingStatus{LastScheduledTime: &oneHourAgo},
	}
}

func newObjectReference(obj *appsv1.Deployment) appsv1alpha1.ObjectReference {
	return appsv1alpha1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Name,
		Namespace:  obj.Namespace,
	}
}

func newClusterResourceBinding(obj *rbacv1.ClusterRole) *workv1alpha2.ClusterResourceBinding {
	return &workv1alpha2.ClusterResourceBinding{
		TypeMeta:   metav1.TypeMeta{Kind: "ClusterResourceBinding", APIVersion: "work.karmada.io/v1alpha2"},
		ObjectMeta: metav1.ObjectMeta{Name: names.GenerateBindingName("ClusterRole", obj.Name)},
		Spec:       workv1alpha2.ResourceBindingSpec{RescheduleTriggeredAt: &oneHourAgo},
		Status:     workv1alpha2.ResourceBindingStatus{LastScheduledTime: &oneHourAgo},
	}
}

func newClusterRoleObjectReference(obj *rbacv1.ClusterRole) appsv1alpha1.ObjectReference {
	return appsv1alpha1.ObjectReference{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Name:       obj.Name,
	}
}

// Helper function for generating random suffix
func randomSuffix() string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		// In a test setup, it's unlikely we'll hit this error
		panic(fmt.Sprintf("failed to generate random number: %v", err))
	}
	return fmt.Sprintf("%d", n)
}
