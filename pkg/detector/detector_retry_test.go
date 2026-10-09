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

package detector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

// InterceptingClient wraps a client.Client to intercept Create calls
type InterceptingClient struct {
	client.Client
	// CreateFunc allows mocking the Create behavior
	CreateFunc func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

func (c *InterceptingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.CreateFunc != nil {
		return c.CreateFunc(ctx, obj, opts...)
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestApplyPolicy_RetryOnAlreadyExists(t *testing.T) {
	// 1. Setup
	scheme := setupTestScheme()
	// Initial object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
				"uid":       "test-uid",
			},
		},
	}
	// Policy
	policy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: policyv1alpha1.PropagationSpec{},
	}

	// 2. Real fake client backing the interceptor
	realFakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(obj).Build()

	// 3. Interceptor
	// We want to simulate:
	// Call 1: Create fails with AlreadyExists
	// Call 2: Create succeeds (conceptually Update, but CreateOrUpdate logic handles the Get internally)
	// Note: CreateOrUpdate calls Client.Get first. If it finds it, it calls Update. Use interceptor to mess with that?
	// Actually, CreateOrUpdate implementation:
	// 1. Get(key, obj)
	// 2. If NotFound -> Create()
	// 3. If exists -> Mutate -> Update()
	// To simulate the race:
	// 1. Get -> NotFound (concurrent reconciliation 1 sees nothing)
	// 2. Create -> AlreadyExists (concurrent reconciliation 2 beat us to it)
	// 3. RETRY LOOP
	// 4. Get -> Found (now we see it)
	// 5. Update -> Success

	var calls int
	interceptingClient := &InterceptingClient{
		Client: realFakeClient,
		CreateFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
			calls++
			if calls == 1 {
				// Inject the race condition: "Someone else created it just now"
				// We must actually CREATE it in the backing store so the next Get finds it
				_ = realFakeClient.Create(ctx, obj, opts...)
				// Return the error that triggers the retry
				return apierrors.NewAlreadyExists(schema.GroupResource{Group: "work.karmada.io", Resource: "resourcebindings"}, obj.GetName())
			}
			// Subsequent calls shouldn't happen for Create if the next Get succeeds,
			// because CreateOrUpdate will switch to Update.
			// However, if logic is wrong, this might be called.
			return realFakeClient.Create(ctx, obj, opts...)
		},
	}

	fakeRecorder := record.NewFakeRecorder(10)
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	d := &ResourceDetector{
		Client:              interceptingClient,
		DynamicClient:       fakeDynamicClient,
		EventRecorder:       fakeRecorder,
		ResourceInterpreter: &mockResourceInterpreter{},
		RESTMapper:          &mockRESTMapper{},
	}

	// 4. Execute
	err := d.ApplyPolicy(obj, keys.ClusterWideKey{Namespace: "default", Name: "test-deployment", Kind: "Deployment"}, false, policy)

	// 5. Verify
	assert.NoError(t, err)

	// Verify metrics or events
	// Since metrics are global, hard to test in unit tests without deep hacking, but check events?
	// If successful, should have an event
	select {
	case <-fakeRecorder.Events:
		// It might be "Apply policy(...) succeed" or "ResourceBinding is up to date".
		// We just want to ensure the event is consumed and no error occurred (checked by assert.NoError above).
	default:
		// No event might mean it was considered "up to date" on the second pass, which is fine
	}

	// Verify the binding exists
	binding := &workv1alpha2.ResourceBinding{}
	err = realFakeClient.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: "test-deployment-deployment"}, binding)
	assert.NoError(t, err, "ResourceBinding should exist after retry")
}

func TestApplyClusterPolicy_RetryOnAlreadyExists(t *testing.T) {
	// Setup
	scheme := setupTestScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "test-cluster-role",
				"uid":  "test-uid",
			},
		},
	}
	policy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-policy",
		},
		Spec: policyv1alpha1.PropagationSpec{},
	}

	realFakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(obj).Build()

	var calls int
	interceptingClient := &InterceptingClient{
		Client: realFakeClient,
		CreateFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
			calls++
			if calls == 1 {
				// Simulate race: store the object then return AlreadyExists
				_ = realFakeClient.Create(ctx, obj, opts...)
				return apierrors.NewAlreadyExists(schema.GroupResource{Group: "work.karmada.io", Resource: "clusterresourcebindings"}, obj.GetName())
			}
			return realFakeClient.Create(ctx, obj, opts...)
		},
	}

	fakeRecorder := record.NewFakeRecorder(10)
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	d := &ResourceDetector{
		Client:              interceptingClient,
		DynamicClient:       fakeDynamicClient,
		EventRecorder:       fakeRecorder,
		ResourceInterpreter: &mockResourceInterpreter{},
		RESTMapper:          &mockRESTMapper{},
	}

	// Execute
	err := d.ApplyClusterPolicy(obj, keys.ClusterWideKey{Name: "test-cluster-role", Kind: "ClusterRole"}, false, policy)

	// Verify
	assert.NoError(t, err)

	binding := &workv1alpha2.ClusterResourceBinding{}
	err = realFakeClient.Get(context.TODO(), client.ObjectKey{Name: "test-cluster-role-clusterrole"}, binding)
	assert.NoError(t, err, "ClusterResourceBinding should exist after retry")
}
