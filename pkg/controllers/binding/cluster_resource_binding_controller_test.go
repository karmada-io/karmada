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

package binding

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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
	"github.com/karmada-io/karmada/pkg/events"
	testing2 "github.com/karmada-io/karmada/pkg/search/proxy/testing"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
	"github.com/karmada-io/karmada/test/helper"
)

func makeFakeCRBCByResource(rs *workv1alpha2.ObjectReference) (*ClusterResourceBindingController, error) {
	c := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithIndex(
		&workv1alpha1.Work{},
		indexregistry.WorkIndexByLabelClusterResourceBindingID,
		indexregistry.GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel),
	).Build()
	tempDyClient := fakedynamic.NewSimpleDynamicClient(scheme.Scheme)
	if rs == nil {
		return &ClusterResourceBindingController{
			Client:          c,
			RESTMapper:      testing2.RestMapper,
			InformerManager: genericmanager.NewSingleClusterInformerManager(context.TODO(), tempDyClient, 0),
			DynamicClient:   tempDyClient,
		}, nil
	}

	var obj runtime.Object
	var src string
	switch rs.Kind {
	case "Namespace":
		obj = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: rs.Name, UID: rs.UID, ResourceVersion: rs.ResourceVersion,
		}}
		src = "namespaces"
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

	return &ClusterResourceBindingController{
		Client:          c,
		RESTMapper:      helper.NewGroupRESTMapper(rs.Kind, meta.RESTScopeNamespace),
		InformerManager: testingutil.NewSingleClusterInformerManagerByRS(src, obj),
		DynamicClient:   tempDyClient,
		EventRecorder:   record.NewFakeRecorder(1024),
	}, nil
}

func TestClusterResourceBindingController_Reconcile(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}
	req := controllerruntime.Request{NamespacedName: client.ObjectKey{Namespace: "", Name: "test"}}

	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
		del     bool
		req     controllerruntime.Request
	}{
		{
			name:    "Reconcile create crb",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
			req: req,
		},
		{
			name:    "Reconcile crb deleted",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
			del: true,
			req: req,
		},
		{
			name:    "Req not found",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-noexist",
				},
			},
			req: req,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("%s", err)
			}

			if tt.crb != nil {
				if err := c.Client.Create(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			if tt.del {
				if err := c.Client.Delete(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to delete ClusterResourceBinding: %v", err)
				}
			}

			result, err := c.Reconcile(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.Reconcile() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestClusterResourceBindingController_removeFinalizer(t *testing.T) {
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
		create  bool
	}{
		{
			name:    "Remove finalizer succeed",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
			},
			create: true,
		},
		{
			name:    "finalizers not exist",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			create: true,
		},
		{
			name:    "crb not found",
			want:    controllerruntime.Result{},
			wantErr: true,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
			},
			create: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(nil)
			if err != nil {
				t.Fatalf("Failed to create ClusterResourceBindingController: %v", err)
			}

			if tt.create && tt.crb != nil {
				if err := c.Client.Create(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			result, err := c.removeFinalizer(context.Background(), tt.crb)
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

func TestClusterResourceBindingController_syncBinding(t *testing.T) {
	const expectedSyncSucceedEventCount = 2 // syncBinding emits succeed events for binding and workload.

	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
	}{
		{
			name:    "sync binding",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("failed to create fake ClusterResourceBindingController: %v", err)
			}

			result, err := c.syncBinding(context.Background(), tt.crb)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.syncBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.syncBinding() = %v, want %v", result, tt.want)
			}

			recorder, ok := c.EventRecorder.(*record.FakeRecorder)
			if !ok {
				t.Fatalf("event recorder type = %T, want *record.FakeRecorder", c.EventRecorder)
			}

			eventsFound := collectFakeEvents(recorder, time.Second)
			if len(eventsFound) < expectedSyncSucceedEventCount {
				t.Fatalf("expected at least %d events, got %d (%v)", expectedSyncSucceedEventCount, len(eventsFound), eventsFound)
			}

			succeedEvents := 0
			for _, event := range eventsFound {
				if strings.Contains(event, events.EventReasonSyncWorkSucceed) {
					succeedEvents++
				}
			}
			if succeedEvents < expectedSyncSucceedEventCount {
				t.Fatalf("expected at least %d %q events, got %d (%v)", expectedSyncSucceedEventCount, events.EventReasonSyncWorkSucceed, succeedEvents, eventsFound)
			}
		})
	}
}

type pendingComponentRequirementsCRBFixture struct {
	controller   *ClusterResourceBindingController
	ctx          context.Context
	binding      *workv1alpha2.ClusterResourceBinding
	assigned     *workv1alpha1.Work
	reference    workv1alpha2.ObjectReference
	acceptedHash string
	worksBefore  []workv1alpha1.Work
	request      controllerruntime.Request
}

func newPendingComponentRequirementsCRBFixture(t *testing.T) *pendingComponentRequirementsCRBFixture {
	t.Helper()
	const bindingID = "cluster-resource-binding-id"
	reference := workv1alpha2.ObjectReference{
		APIVersion: "v1", Kind: "Namespace", Name: "test",
		UID: "source-uid", ResourceVersion: "1",
	}
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	hash, err := util.GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}
	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-crb",
			Generation: 7,
			Labels:     map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
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
		Labels:    map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
		Annotations: map[string]string{
			"test.karmada.io/content": "assigned-before-acceptance",
		},
	}}
	orphan := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
		Name:      "existing-work",
		Namespace: names.GenerateExecutionSpaceName("member2"),
		Labels:    map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
		Annotations: map[string]string{
			"test.karmada.io/content": "orphan-before-acceptance",
		},
	}}
	c, err := makeFakeCRBCByResource(&reference)
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
	worksBefore := snapshotWorks(t, c.Client)
	if len(worksBefore) != 2 {
		t.Fatalf("initial Work count = %d, want 2", len(worksBefore))
	}
	return &pendingComponentRequirementsCRBFixture{
		controller:   c,
		ctx:          ctx,
		binding:      binding,
		assigned:     assigned,
		reference:    reference,
		acceptedHash: hash,
		worksBefore:  worksBefore,
		request:      controllerruntime.Request{NamespacedName: client.ObjectKeyFromObject(binding)},
	}
}

func assertCRBPreservesWorksWhileComponentRequirementsPending(t *testing.T, fixture *pendingComponentRequirementsCRBFixture) {
	t.Helper()
	if _, err := fixture.controller.Reconcile(fixture.ctx, fixture.request); err != nil {
		t.Fatalf("Reconcile() while pending error = %v", err)
	}
	if after := snapshotWorks(t, fixture.controller.Client); !reflect.DeepEqual(after, fixture.worksBefore) {
		t.Fatalf("pending component result mutated the Work set: got %#v, want %#v", after, fixture.worksBefore)
	}
}

func acceptCRBComponentRequirements(t *testing.T, fixture *pendingComponentRequirementsCRBFixture) {
	t.Helper()
	current := &workv1alpha2.ClusterResourceBinding{}
	if err := fixture.controller.Client.Get(fixture.ctx, client.ObjectKeyFromObject(fixture.binding), current); err != nil {
		t.Fatal(err)
	}
	old := current.DeepCopy()
	current.Annotations = map[string]string{util.AcceptedComponentRequirementsHashAnnotation: fixture.acceptedHash}
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

func assertCRBWorksRecoveredAfterComponentRequirementsAcceptance(t *testing.T, fixture *pendingComponentRequirementsCRBFixture) {
	t.Helper()
	if _, err := fixture.controller.Reconcile(fixture.ctx, fixture.request); err != nil {
		t.Fatalf("Reconcile() after acceptance error = %v", err)
	}
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

func TestClusterResourceBindingControllerSyncBindingPreservesWorksWhileComponentRequirementsPending(t *testing.T) {
	enableMultiplePodTemplatesScheduling(t)
	fixture := newPendingComponentRequirementsCRBFixture(t)

	assertCRBPreservesWorksWhileComponentRequirementsPending(t, fixture)
	acceptCRBComponentRequirements(t, fixture)
	assertCRBWorksRecoveredAfterComponentRequirementsAcceptance(t, fixture)
}

func TestClusterResourceBindingControllerSyncBindingBypassesMissingHashFence(t *testing.T) {
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
			const bindingID = "cluster-resource-binding-id"
			reference := workv1alpha2.ObjectReference{
				APIVersion: "v1", Kind: "Namespace", Name: "test",
				UID: "source-uid", ResourceVersion: "1",
			}
			components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
			binding := &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-crb",
					Labels: map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
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
				Labels: map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
			}}
			orphan := &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: "existing-work", Namespace: names.GenerateExecutionSpaceName("member2"),
				Labels: map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
			}}
			c, err := makeFakeCRBCByResource(&reference)
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

func TestClusterResourceBindingControllerSyncBindingPreservesWorksForStaleSourceSnapshot(t *testing.T) {
	enableMultiplePodTemplatesScheduling(t)

	const bindingID = "cluster-resource-binding-id"
	sourceReference := workv1alpha2.ObjectReference{
		APIVersion: "v1", Kind: "Namespace", Name: "test", UID: "source-uid", ResourceVersion: "2",
	}
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	acceptedHash, err := util.GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}
	bindingReference := sourceReference
	bindingReference.ResourceVersion = "1"
	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-crb",
			Labels: map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
			Annotations: map[string]string{
				util.AcceptedComponentRequirementsHashAnnotation: acceptedHash,
				util.ResourceTemplateSpecificationHashAnnotation: "v1:sha256:stale",
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
		Labels: map[string]string{workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID},
	}}
	controller, err := makeFakeCRBCByResource(&sourceReference)
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
}

func collectFakeEvents(recorder *record.FakeRecorder, timeout time.Duration) []string {
	found := make([]string, 0)
	deadline := time.After(timeout)

	for {
		select {
		case e := <-recorder.Events:
			found = append(found, e)
		case <-deadline:
			return found
		}
	}
}

func TestClusterResourceBindingController_removeOrphanWorks(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}
	tests := []struct {
		name    string
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
	}{
		{
			name:    "removeOrphanWorks test",
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("failed to create fake ClusterResourceBindingController: %v", err)
			}

			err = c.removeOrphanWorks(context.Background(), tt.crb)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.syncBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestClusterResourceBindingController_newOverridePolicyFunc(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}

	tests := []struct {
		name string
		want []reconcile.Request
		req  client.Object
		crb  *workv1alpha2.ClusterResourceBinding
	}{
		{
			name: "not clusteroverridepolicy",
			want: nil,
			req:  &policyv1alpha1.OverridePolicy{},
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "newOverridePolicyFunc test succeed",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: rs.Name}}},
			req: &policyv1alpha1.ClusterOverridePolicy{
				Spec: policyv1alpha1.OverrideSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: rs.APIVersion,
							Kind:       rs.Kind,
							Name:       rs.Name,
						},
					},
				},
			},
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "ResourceSelector is empty",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: rs.Name}}},
			req: &policyv1alpha1.ClusterOverridePolicy{
				Spec: policyv1alpha1.OverrideSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{},
				},
			},
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("failed to create fake ClusterResourceBindingController: %v", err)
			}

			if tt.crb != nil {
				if err := c.Client.Create(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			got := c.newOverridePolicyFunc()
			result := got(context.Background(), tt.req)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.newOverridePolicyFunc() get = %v, want %v", result, tt.want)
				return
			}
		})
	}
}
