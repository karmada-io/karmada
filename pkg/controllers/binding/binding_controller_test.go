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

package binding

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	testing2 "github.com/karmada-io/karmada/pkg/search/proxy/testing"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
	"github.com/karmada-io/karmada/test/helper"
)

// makeFakeRBCByResource to make a fake ResourceBindingController with ObjectReference.
// Currently support kind: Pod,Node. If you want support more kind, pls add it.
// rs is nil means use default RestMapper, see: github.com/karmada-io/karmada/pkg/search/proxy/testing/constant.go
func makeFakeRBCByResource(rs *workv1alpha2.ObjectReference) (*ResourceBindingController, error) {
	c := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithIndex(
		&workv1alpha1.Work{},
		indexregistry.WorkIndexByLabelResourceBindingID,
		indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
	).Build()

	tempDyClient := fakedynamic.NewSimpleDynamicClient(scheme.Scheme)
	if rs == nil {
		return &ResourceBindingController{
			Client:          c,
			RESTMapper:      testing2.RestMapper,
			InformerManager: genericmanager.NewSingleClusterInformerManager(context.TODO(), tempDyClient, 0),
			DynamicClient:   tempDyClient,
		}, nil
	}

	var obj runtime.Object
	var src string
	switch rs.Kind {
	case "Pod":
		obj = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: rs.Name, Namespace: rs.Namespace, UID: rs.UID, ResourceVersion: rs.ResourceVersion,
		}}
		src = "pods"
	case "Node":
		obj = &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name: rs.Name, Namespace: rs.Namespace, UID: rs.UID, ResourceVersion: rs.ResourceVersion,
		}}
		src = "nodes"
	default:
		return nil, fmt.Errorf("%s not support yet, pls add for it", rs.Kind)
	}

	tempDyClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: appsv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: rs.Name, Namespaced: true, Kind: rs.Kind, Version: rs.APIVersion},
			},
		},
	}

	return &ResourceBindingController{
		Client:          c,
		RESTMapper:      helper.NewGroupRESTMapper(rs.Kind, meta.RESTScopeNamespace),
		InformerManager: testingutil.NewSingleClusterInformerManagerByRS(src, obj),
		DynamicClient:   tempDyClient,
		EventRecorder:   record.NewFakeRecorder(1024),
	}, nil
}

func TestResourceBindingController_Reconcile(t *testing.T) {
	tmpReq := controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-rb",
			Namespace: "default",
		},
	}
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
		req     controllerruntime.Request
	}{
		{
			name:    "Err is RB not found",
			want:    controllerruntime.Result{},
			wantErr: false,
			req:     tmpReq,
		},
		{
			name:    "RB found without deleting",
			want:    controllerruntime.Result{},
			wantErr: true,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
			},
			req: tmpReq,
		},
		{
			name:    "Req not found",
			want:    controllerruntime.Result{Requeue: false},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "haha-rb",
					Namespace: "default",
				},
			},
			req: tmpReq,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(nil)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}
			if tt.rb != nil {
				// Add a rb to the fake client.
				if err := c.Client.Create(context.Background(), tt.rb); err != nil {
					t.Fatalf("Failed to create rb: %v", err)
				}
			}
			// Run the reconcile function.
			got, err := c.Reconcile(context.Background(), tt.req)
			// Check the results.
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reconcile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceBindingController_syncBinding(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod",
	}
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
	}{
		{
			name:    "syncBinding success test",
			want:    controllerruntime.Result{},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(&rs)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}
			got, err := c.syncBinding(context.Background(), tt.rb)
			if (err != nil) != tt.wantErr {
				t.Errorf("syncBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("syncBinding() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type pendingComponentResultFixture struct {
	controller *ResourceBindingController
	binding    *workv1alpha2.ResourceBinding
	assigned   *workv1alpha1.Work
	reference  workv1alpha2.ObjectReference
	hash       string
	ctx        context.Context
	request    controllerruntime.Request
}

func newPendingComponentResultFixture(t *testing.T) *pendingComponentResultFixture {
	t.Helper()

	const bindingID = "resource-binding-id"
	reference := workv1alpha2.ObjectReference{
		APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
		UID: "source-uid", ResourceVersion: "1",
	}
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	hash, err := util.GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}
	binding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-rb",
			Namespace:  "default",
			Generation: 7,
			Labels:     map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource:      reference,
			SchedulerName: corev1.DefaultSchedulerName,
			Placement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
				SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
			}}},
			Components: components,
			Clusters: []workv1alpha2.TargetCluster{{Name: "member1", Components: []workv1alpha2.TargetComponent{
				{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
			}}},
		},
	}
	assigned := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
		Name:      names.GenerateWorkName(reference.Kind, reference.Name, reference.Namespace),
		Namespace: names.GenerateExecutionSpaceName("member1"),
		Labels:    map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
		Annotations: map[string]string{
			"test.karmada.io/content": "assigned-before-acceptance",
		},
	}}
	orphan := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
		Name:      "existing-work",
		Namespace: names.GenerateExecutionSpaceName("member2"),
		Labels:    map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
		Annotations: map[string]string{
			"test.karmada.io/content": "orphan-before-acceptance",
		},
	}}
	controller, err := makeFakeRBCByResource(&reference)
	if err != nil {
		t.Fatal(err)
	}
	controller.OverrideManager = noOpOverrideManager{}
	controller.ResourceInterpreter = newComponentRevisionInterpreter(components)
	ctx := context.Background()
	for _, object := range []client.Object{binding, assigned, orphan} {
		if err := controller.Client.Create(ctx, object); err != nil {
			t.Fatal(err)
		}
	}
	return &pendingComponentResultFixture{
		controller: controller,
		binding:    binding,
		assigned:   assigned,
		reference:  reference,
		hash:       hash,
		ctx:        ctx,
		request:    controllerruntime.Request{NamespacedName: client.ObjectKeyFromObject(binding)},
	}
}

func acceptPendingComponentResult(t *testing.T, fixture *pendingComponentResultFixture) {
	t.Helper()

	current := &workv1alpha2.ResourceBinding{}
	if err := fixture.controller.Client.Get(fixture.ctx, client.ObjectKeyFromObject(fixture.binding), current); err != nil {
		t.Fatal(err)
	}
	old := current.DeepCopy()
	current.Annotations = map[string]string{util.AcceptedComponentRequirementsHashAnnotation: fixture.hash}
	if err := fixture.controller.Client.Update(fixture.ctx, current); err != nil {
		t.Fatal(err)
	}
	if current.Generation != old.Generation {
		t.Fatalf("annotation-only acceptance changed generation from %d to %d", old.Generation, current.Generation)
	}
	if !bindingEventPredicate().Update(event.UpdateEvent{ObjectOld: old, ObjectNew: current}) {
		t.Fatal("accepted requirements annotation update should trigger reconciliation")
	}
}

func assertRecoveredAssignedWork(t *testing.T, fixture *pendingComponentResultFixture) {
	t.Helper()

	works := snapshotWorks(t, fixture.controller.Client)
	if len(works) != 1 {
		t.Fatalf("recovered Work count = %d, want 1: %#v", len(works), works)
	}
	if works[0].Name != fixture.assigned.Name || works[0].Namespace != fixture.assigned.Namespace {
		t.Fatalf("recovered Work = %s/%s, want %s/%s", works[0].Namespace, works[0].Name, fixture.assigned.Namespace, fixture.assigned.Name)
	}
	if reflect.DeepEqual(works[0], *fixture.assigned) {
		t.Fatal("accepted requirements should update the assigned Work")
	}
	workload := workloadFromWork(t, &works[0])
	if workload.GetKind() != fixture.reference.Kind || workload.GetName() != fixture.reference.Name || workload.GetNamespace() != fixture.reference.Namespace {
		t.Fatalf("recovered Work manifest = %s %s/%s, want %s %s/%s",
			workload.GetKind(), workload.GetNamespace(), workload.GetName(), fixture.reference.Kind, fixture.reference.Namespace, fixture.reference.Name)
	}
}

func TestResourceBindingControllerSyncBindingPreservesWorksWhileComponentResultPending(t *testing.T) {
	enableMultiplePodTemplatesScheduling(t)
	fixture := newPendingComponentResultFixture(t)

	before := snapshotWorks(t, fixture.controller.Client)
	if len(before) != 2 {
		t.Fatalf("initial Work count = %d, want 2", len(before))
	}
	if _, err := fixture.controller.Reconcile(fixture.ctx, fixture.request); err != nil {
		t.Fatalf("Reconcile() while pending error = %v", err)
	}
	if after := snapshotWorks(t, fixture.controller.Client); !reflect.DeepEqual(after, before) {
		t.Fatalf("pending component result mutated the Work set: got %#v, want %#v", after, before)
	}

	acceptPendingComponentResult(t, fixture)
	if _, err := fixture.controller.Reconcile(fixture.ctx, fixture.request); err != nil {
		t.Fatalf("Reconcile() after acceptance error = %v", err)
	}
	assertRecoveredAssignedWork(t, fixture)
}

func TestShouldWaitForComponentScheduleResult(t *testing.T) {
	enableMultiplePodTemplatesScheduling(t)

	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	spec := workv1alpha2.ResourceBindingSpec{
		Placement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}}},
		Components: components,
		Clusters: []workv1alpha2.TargetCluster{{Name: "member1", Components: []workv1alpha2.TargetComponent{
			{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
		}}},
	}
	hash, err := util.GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}
	suspended := true

	tests := []struct {
		name        string
		mutate      func(*workv1alpha2.ResourceBindingSpec)
		annotations map[string]string
		want        bool
	}{
		{name: "empty scheduler name uses default scheduler", want: true},
		{name: "explicit default scheduler", mutate: func(spec *workv1alpha2.ResourceBindingSpec) { spec.SchedulerName = corev1.DefaultSchedulerName }, want: true},
		{name: "custom scheduler remains outside default scheduler protocol", mutate: func(spec *workv1alpha2.ResourceBindingSpec) { spec.SchedulerName = "custom-scheduler" }},
		{name: "suspended scheduling keeps accepted result fenced", mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
			spec.Suspension = &workv1alpha2.Suspension{Scheduling: &suspended}
		}, want: true},
		{name: "accepted default scheduler result is ready", annotations: map[string]string{util.AcceptedComponentRequirementsHashAnnotation: hash}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpec := spec.DeepCopy()
			if tt.mutate != nil {
				tt.mutate(gotSpec)
			}
			if got := shouldWaitForComponentScheduleResult(gotSpec, tt.annotations); got != tt.want {
				t.Fatalf("shouldWaitForComponentScheduleResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceBindingControllerSyncBindingBypassesMissingHashFence(t *testing.T) {
	enableMultiplePodTemplatesScheduling(t)

	tests := []struct {
		name         string
		legacyResult bool
		mutate       func(*workv1alpha2.ResourceBindingSpec)
	}{
		{
			name: "custom scheduler",
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.SchedulerName = "custom-scheduler"
			},
		},
		{
			name:         "custom scheduler with legacy scalar result",
			legacyResult: true,
			mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
				spec.SchedulerName = "custom-scheduler"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const bindingID = "resource-binding-id"
			reference := workv1alpha2.ObjectReference{
				APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				UID: "source-uid", ResourceVersion: "1",
			}
			components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
			binding := &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb", Namespace: "default",
					Labels: map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: reference, SchedulerName: corev1.DefaultSchedulerName,
					Placement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
					}}},
					Components: components,
					Clusters: []workv1alpha2.TargetCluster{{Name: "member1", Components: []workv1alpha2.TargetComponent{
						{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
					}}},
				},
			}
			tt.mutate(&binding.Spec)
			if tt.legacyResult {
				binding.Spec.Clusters[0].Components = nil
				binding.Spec.Clusters[0].Replicas = 4
			}
			if !util.IsBindingComponentResultPending(&binding.Spec, nil) {
				t.Fatal("test setup must require the missing-hash fence for an active default-scheduler binding")
			}

			assigned := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: names.GenerateWorkName(reference.Kind, reference.Name, reference.Namespace), Namespace: names.GenerateExecutionSpaceName("member1"),
				Labels: map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
			}}
			orphan := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: "existing-work", Namespace: names.GenerateExecutionSpaceName("member2"),
				Labels: map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
			}}
			c, err := makeFakeRBCByResource(&reference)
			if err != nil {
				t.Fatal(err)
			}
			c.OverrideManager = noOpOverrideManager{}
			c.ResourceInterpreter = newComponentRevisionInterpreter(components)
			ctx := context.Background()
			for _, object := range []client.Object{binding, assigned, orphan} {
				if err := c.Client.Create(ctx, object); err != nil {
					t.Fatal(err)
				}
			}

			req := controllerruntime.Request{NamespacedName: client.ObjectKeyFromObject(binding)}
			if _, err := c.Reconcile(ctx, req); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}
			works := snapshotWorks(t, c.Client)
			if len(works) != 1 {
				t.Fatalf("Work count = %d, want 1: %#v", len(works), works)
			}
			if works[0].Name != assigned.Name || works[0].Namespace != assigned.Namespace {
				t.Fatalf("remaining Work = %s/%s, want %s/%s", works[0].Namespace, works[0].Name, assigned.Namespace, assigned.Name)
			}
			workload := workloadFromWork(t, &works[0])
			if workload.GetKind() != reference.Kind || workload.GetName() != reference.Name {
				t.Fatalf("Work manifest = %s/%s, want %s/%s", workload.GetKind(), workload.GetName(), reference.Kind, reference.Name)
			}
		})
	}
}

func TestResourceBindingControllerSyncBindingPreservesWorksForStaleSourceSnapshot(t *testing.T) {
	enableMultiplePodTemplatesScheduling(t)

	const bindingID = "resource-binding-id"
	sourceReference := workv1alpha2.ObjectReference{
		APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
		UID: "source-uid", ResourceVersion: "2",
	}
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	acceptedHash, err := util.GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		bindingUID types.UID
		sourceHash string
	}{
		{name: "source specification advanced", bindingUID: sourceReference.UID, sourceHash: "v1:sha256:stale"},
		{name: "source was recreated", bindingUID: "old-source-uid", sourceHash: "v1:sha256:stale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindingReference := sourceReference
			bindingReference.UID = tt.bindingUID
			bindingReference.ResourceVersion = "1"
			binding := &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rb", Namespace: "default",
					Labels: map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
					Annotations: map[string]string{
						util.AcceptedComponentRequirementsHashAnnotation: acceptedHash,
						util.ResourceTemplateSpecificationHashAnnotation: tt.sourceHash,
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: bindingReference, Placement: singleClusterComponentPlacement(), Components: components,
					Clusters: []workv1alpha2.TargetCluster{{Name: "member1", Components: []workv1alpha2.TargetComponent{
						{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
					}}},
				},
			}
			orphan := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: "existing-work", Namespace: names.GenerateExecutionSpaceName("member2"),
				Labels: map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: bindingID},
			}}
			controller, err := makeFakeRBCByResource(&sourceReference)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if err := controller.Client.Create(ctx, orphan); err != nil {
				t.Fatal(err)
			}
			before := snapshotWorks(t, controller.Client)

			result, err := controller.syncBinding(ctx, binding)
			if err != nil {
				t.Fatalf("syncBinding() error = %v", err)
			}
			if result != (controllerruntime.Result{}) {
				t.Fatalf("syncBinding() = %v, want empty result", result)
			}
			if after := snapshotWorks(t, controller.Client); !reflect.DeepEqual(after, before) {
				t.Fatalf("stale source snapshot mutated Works: got %#v, want %#v", after, before)
			}
		})
	}
}

func TestBindingEventPredicate(t *testing.T) {
	p := bindingEventPredicate()

	tests := []struct {
		name string
		old  client.Object
		new  client.Object
		want bool
	}{
		{
			name: "generation change",
			old:  &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 1}},
			new:  &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}},
			want: true,
		},
		{
			name: "accepted hash backfill",
			old:  &workv1alpha2.ResourceBinding{},
			new: &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				util.AcceptedComponentRequirementsHashAnnotation: "v1:sha256:hash",
			}}},
			want: true,
		},
		{
			name: "cluster binding accepted hash update",
			old:  &workv1alpha2.ClusterResourceBinding{},
			new: &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				util.AcceptedComponentRequirementsHashAnnotation: "v1:sha256:hash",
			}}},
			want: true,
		},
		{
			name: "source specification hash backfill",
			old: &workv1alpha2.ResourceBinding{Spec: workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{
				Name: "member1", Components: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 1}},
			}}}},
			new: &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				util.ResourceTemplateSpecificationHashAnnotation: "v1:sha256:hash",
			}}, Spec: workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{
				Name: "member1", Components: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 1}},
			}}}},
			want: true,
		},
		{
			name: "cluster binding source specification hash backfill",
			old: &workv1alpha2.ClusterResourceBinding{Spec: workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{
				Name: "member1", Components: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 1}},
			}}}},
			new: &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				util.ResourceTemplateSpecificationHashAnnotation: "v1:sha256:hash",
			}}, Spec: workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{
				Name: "member1", Components: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 1}},
			}}}},
			want: true,
		},
		{
			name: "ordinary binding source hash does not trigger work sync",
			old:  &workv1alpha2.ResourceBinding{},
			new: &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				util.ResourceTemplateSpecificationHashAnnotation: "v1:sha256:hash",
			}}},
		},
		{
			name: "unrelated annotation update",
			old:  &workv1alpha2.ResourceBinding{},
			new:  &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"example.com/key": "value"}}},
		},
		{
			name: "non-binding annotation update",
			old:  &policyv1alpha1.OverridePolicy{},
			new:  &policyv1alpha1.OverridePolicy{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{util.AcceptedComponentRequirementsHashAnnotation: "v1:sha256:hash"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Update(event.UpdateEvent{ObjectOld: tt.old, ObjectNew: tt.new}); got != tt.want {
				t.Errorf("bindingEventPredicate().Update() = %v, want %v", got, tt.want)
			}
		})
	}
}

func enableMultiplePodTemplatesScheduling(t *testing.T) {
	t.Helper()
	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { features.FeatureGate = originalFeatureGates })
}

func singleClusterComponentPlacement() *policyv1alpha1.Placement {
	return &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster,
		MinGroups:     1,
		MaxGroups:     1,
	}}}
}

func TestResourceBindingController_removeOrphanWorks(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod",
	}
	tests := []struct {
		name    string
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
	}{
		{
			name:    "removeOrphanWorks success test",
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(&rs)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}
			if err := c.removeOrphanWorks(context.TODO(), tt.rb); (err != nil) != tt.wantErr {
				t.Errorf("removeOrphanWorks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourceBindingController_newOverridePolicyFunc(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod",
	}
	tests := []struct {
		name string
		want []reconcile.Request
		req  client.Object
		rb   *workv1alpha2.ResourceBinding
	}{
		{
			name: "newOverridePolicyFunc success test",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-rb"}}},
			req: &policyv1alpha1.OverridePolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: rs.Namespace},
				Spec: policyv1alpha1.OverrideSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: rs.APIVersion,
						Kind:       rs.Kind,
						Namespace:  rs.Namespace,
						Name:       rs.Name,
					},
				}},
			},
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "namespace not match",
			want: nil,
			req: &policyv1alpha1.OverridePolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: rs.Namespace},
				Spec: policyv1alpha1.OverrideSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: rs.APIVersion,
						Kind:       rs.Kind,
						Namespace:  rs.Namespace,
						Name:       rs.Name,
					},
				}},
			},
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "test",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "ResourceSelector is empty",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-rb"}}},
			req: &policyv1alpha1.OverridePolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: rs.Namespace},
				Spec:       policyv1alpha1.OverrideSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{}},
			},
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "client is nil",
			want: nil,
			req:  nil,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(&rs)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}

			if tt.rb != nil {
				if err := c.Client.Create(context.Background(), tt.rb); err != nil {
					t.Errorf("create rb %v", err)
					return
				}
			}

			got := c.newOverridePolicyFunc()
			result := got(context.Background(), tt.req)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("newOverridePolicyFunc() got() result is %v not same as want: %v", result, tt.want)
			}
		})
	}
}

func TestResourceBindingController_removeFinalizer(t *testing.T) {
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
		create  bool
	}{
		{
			name:    "Remove finalizer succeed",
			want:    controllerruntime.Result{},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.BindingControllerFinalizer},
				},
			},
			create: true,
		},
		{
			name:    "finalizers not exist",
			want:    controllerruntime.Result{},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			create: true,
		},
		{
			name:    "rb not found",
			want:    controllerruntime.Result{},
			wantErr: true,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.BindingControllerFinalizer},
				},
			},
			create: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeRBCByResource(nil)
			if err != nil {
				t.Fatalf("Failed to create ClusterResourceBindingController: %v", err)
			}

			if tt.create && tt.rb != nil {
				if err := c.Client.Create(context.Background(), tt.rb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			result, err := c.removeFinalizer(context.Background(), tt.rb)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.removeFinalizer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.removeFinalizer() = %v, want %v", result, tt.want)
			}
		})
	}
}
