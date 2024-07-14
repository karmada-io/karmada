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
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
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

	deploy1    = helper.NewDeployment("test-ns", "test-1")
	binding1   = newResourceBinding(deploy1)
	deploy1Obj = newObjectReference(deploy1)

	// use deploy2 to mock a resource whose resource-binding not found.
	deploy2    = helper.NewDeployment("test-ns", "test-2")
	deploy2Obj = newObjectReference(deploy2)

	deploy3    = helper.NewDeployment("test-ns", "test-3")
	binding3   = newResourceBinding(deploy3)
	deploy3Obj = newObjectReference(deploy3)

	pendingRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "rebalancer-with-pending-workloads", CreationTimestamp: now},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			// Put deploy2Obj before deploy1Obj to test whether the results of status are sorted.
			Workloads: []appsv1alpha1.ObjectReference{deploy2Obj, deploy1Obj},
		},
	}
	succeedRebalancer = &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "rebalancer-with-succeed-workloads", CreationTimestamp: oneHourAgo},
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
		ObjectMeta: metav1.ObjectMeta{Name: "rebalancer-with-workloads-whose-binding-not-found", CreationTimestamp: now},
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
		ObjectMeta: metav1.ObjectMeta{Name: "rebalancer-with-failed-workloads", CreationTimestamp: now},
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
		ObjectMeta: metav1.ObjectMeta{Name: "rebalancer-which-experienced-modification", CreationTimestamp: oneHourAgo},
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
		ObjectMeta: metav1.ObjectMeta{Name: "ttl-finished-rebalancer", CreationTimestamp: oneHourAgo},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &RebalancerController{
				Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(tt.existObjects...).
					WithStatusSubresource(tt.existObjsWithStatus...).Build(),
			}
			_, err := c.Reconcile(context.TODO(), tt.req)
			// 1. check whether it has error
			if (err == nil && tt.wantErr) || (err != nil && !tt.wantErr) {
				t.Fatalf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
			}
			// 2. check final WorkloadRebalancer status
			rebalancerGet := &appsv1alpha1.WorkloadRebalancer{}
			if err := c.Client.Get(context.TODO(), tt.req.NamespacedName, rebalancerGet); err != nil {
				if apierrors.IsNotFound(err) && tt.needsCleanup {
					t.Logf("WorkloadRebalancer %s has be cleaned up as expected", tt.req.NamespacedName)
					return
				}
				t.Fatalf("get WorkloadRebalancer failed: %+v", err)
			}
			// we can't predict `FinishTime` in `wantStatus`, so not compare this field.
			tt.wantStatus.FinishTime = rebalancerGet.Status.FinishTime
			if !reflect.DeepEqual(rebalancerGet.Status, tt.wantStatus) {
				t.Fatalf("update WorkloadRebalancer failed, got: %+v, want: %+v", rebalancerGet.Status, tt.wantStatus)
			}
			// 3. check binding's rescheduleTriggeredAt
			for _, item := range rebalancerGet.Status.ObservedWorkloads {
				if item.Result != appsv1alpha1.RebalanceSuccessful {
					continue
				}
				bindingGet := &workv1alpha2.ResourceBinding{}
				bindingName := names.GenerateBindingName(item.Workload.Kind, item.Workload.Name)
				if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: item.Workload.Namespace, Name: bindingName}, bindingGet); err != nil {
					t.Fatalf("get bindding (%s) failed: %+v", bindingName, err)
				}
				if !bindingGet.Spec.RescheduleTriggeredAt.Equal(&rebalancerGet.CreationTimestamp) {
					t.Fatalf("rescheduleTriggeredAt of binding got: %+v, want: %+v", bindingGet.Spec.RescheduleTriggeredAt, rebalancerGet.CreationTimestamp)
				}
			}
		})
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
