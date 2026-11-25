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

package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	genericfaketesting "github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager/testing"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// TestUpdateResourceStatusWithRealInterpreterNoLua exercises the full
// status-controller -> actual resource interpreter -> UpdateStatus path.
// An empty informer store models an installation without Lua RIC or webhooks.
func TestUpdateResourceStatusWithRealInterpreterNoLua(t *testing.T) {
	for _, tc := range []struct {
		name        string
		multiTarget bool
	}{
		{name: "single target preserves full status"},
		{name: "two targets but only one Work observed uses stock aggregation", multiTarget: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pod := &corev1.Pod{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{Name: "status-real", Namespace: "default"},
				Spec: corev1.PodSpec{Containers: []corev1.Container{
					{Name: "runner", Image: "busybox:1.36.1"},
					{Name: "sidecar", Image: "busybox:1.36.1"},
				}},
			}
			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, pod)
			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
			mapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)

			fakeInformer := genericfaketesting.NewFakeSingleClusterManager(true, false,
				func(gvr schema.GroupVersionResource) cache.GenericLister {
					return cache.NewGenericLister(
						cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
						gvr.GroupResource())
				})
			interpreter := resourceinterpreter.NewResourceInterpreter(fakeInformer, nil)
			require.NoError(t, interpreter.Start(ctx))

			memberStatus := corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.244.0.99",
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionFalse},
				},
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "runner", Ready: false, State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"},
					}},
					{Name: "sidecar", Ready: true, State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					}},
				},
			}
			raw, err := helper.BuildStatusRawExtension(&memberStatus)
			require.NoError(t, err)

			bindingSpec := workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "status-real",
				},
				Clusters: []workv1alpha2.TargetCluster{{Name: "member-a"}},
			}
			if tc.multiTarget {
				bindingSpec.Clusters = append(bindingSpec.Clusters, workv1alpha2.TargetCluster{Name: "member-b"})
			}
			bindingStatus := workv1alpha2.ResourceBindingStatus{
				AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
					{ClusterName: "member-a", Applied: true, Status: raw},
				},
			}
			require.NoError(t, updateResourceStatus(
				ctx, dynamicClient, mapper, interpreter, record.NewFakeRecorder(5),
				bindingSpec, bindingStatus,
			))

			gotObj, err := dynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).
				Namespace("default").Get(ctx, "status-real", metav1.GetOptions{})
			require.NoError(t, err)
			var got corev1.Pod
			require.NoError(t, helper.ConvertToTypedObject(gotObj, &got))
			require.Equal(t, corev1.PodRunning, got.Status.Phase)
			require.Len(t, got.Status.ContainerStatuses, 2)
			require.NotNil(t, got.Status.ContainerStatuses[0].State.Terminated)
			require.Equal(t, int32(0), got.Status.ContainerStatuses[0].State.Terminated.ExitCode)

			if tc.multiTarget {
				// Existing Karmada behavior must remain intact.
				require.Empty(t, got.Status.ContainerStatuses[0].Name)
				require.Empty(t, got.Status.ContainerStatuses[1].Name)
				require.Empty(t, got.Status.Conditions)
				require.Empty(t, got.Status.PodIP)
			} else {
				require.Equal(t, memberStatus, got.Status)
			}

			foundStatusUpdate := false
			for _, a := range dynamicClient.Actions() {
				if a.GetVerb() == "update" && a.GetSubresource() == "status" {
					foundStatusUpdate = true
				}
			}
			require.True(t, foundStatusUpdate, "status controller must persist Pod.status")
		})
	}
}
