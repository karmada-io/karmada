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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func TestExecutionController_Reconcile(t *testing.T) {
	tests := []struct {
		name               string
		work               *workv1alpha1.Work
		ns                 string
		expectRes          controllerruntime.Result
		expectCondition    *metav1.Condition
		expectEventMessage string
		existErr           bool
	}{
		{
			name:      "work dispatching is suspended, no error, no apply",
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
			work: newWork(func(work *workv1alpha1.Work) {
				work.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:            "work dispatching is suspended, adds false dispatching condition",
			ns:              "karmada-es-cluster",
			expectRes:       controllerruntime.Result{},
			expectCondition: &metav1.Condition{Type: workv1alpha1.WorkDispatching, Status: metav1.ConditionFalse},
			existErr:        false,

			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:               "work dispatching is suspended, adds event message",
			ns:                 "karmada-es-cluster",
			expectRes:          controllerruntime.Result{},
			expectEventMessage: fmt.Sprintf("%s %s %s", corev1.EventTypeNormal, events.EventReasonWorkDispatching, WorkSuspendDispatchingConditionMessage),
			existErr:           false,
			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:            "work dispatching is suspended, overwrites existing dispatching condition",
			ns:              "karmada-es-cluster",
			expectRes:       controllerruntime.Result{},
			expectCondition: &metav1.Condition{Type: workv1alpha1.WorkDispatching, Status: metav1.ConditionFalse},
			existErr:        false,
			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = ptr.To(true)
				meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
					Type:   workv1alpha1.WorkDispatching,
					Status: metav1.ConditionTrue,
					Reason: workDispatchingConditionReason,
				})
			}),
		},
		{
			name:      "suspend work with deletion timestamp is deleted",
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
				work.Spec.SuspendDispatching = ptr.To(true)
			}),
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

			eventRecorder := record.NewFakeRecorder(1)
			c := newController(tt.work, eventRecorder)
			res, err := c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectRes, res)
			if tt.existErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectCondition != nil {
				assert.NoError(t, c.Client.Get(context.Background(), req.NamespacedName, tt.work))
				assert.True(t, meta.IsStatusConditionPresentAndEqual(tt.work.Status.Conditions, tt.expectCondition.Type, tt.expectCondition.Status))
			}

			if tt.expectEventMessage != "" {
				assert.Equal(t, 1, len(eventRecorder.Events))
				e := <-eventRecorder.Events
				assert.Equal(t, tt.expectEventMessage, e)
			}
		})
	}
}

func newController(work *workv1alpha1.Work, eventRecorder *record.FakeRecorder) Controller {
	cluster := newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
	pod := testhelper.NewPod("default", "test")
	client := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster, work, pod).WithStatusSubresource(work).Build()
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind(pod.Kind), meta.RESTScopeNamespace)
	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, pod)
	informerManager := genericmanager.GetInstance()
	informerManager.ForCluster(cluster.Name, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
	informerManager.Start(cluster.Name)
	informerManager.WaitForCacheSync(cluster.Name)
	return Controller{
		Client:          client,
		InformerManager: informerManager,
		EventRecorder:   eventRecorder,
		RESTMapper:      restMapper,
		ObjectWatcher:   objectwatcher.NewObjectWatcher(client, restMapper, util.NewClusterDynamicClientSetForAgent, nil),
	}
}

func newWork(applyFunc func(work *workv1alpha1.Work)) *workv1alpha1.Work {
	pod := testhelper.NewPod("default", "test")
	bytes, _ := json.Marshal(pod)
	work := testhelper.NewWork("work", "karmada-es-cluster", string(uuid.NewUUID()), bytes)
	if applyFunc != nil {
		applyFunc(work)
	}
	return work
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
