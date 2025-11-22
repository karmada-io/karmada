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

package resourcequota

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	frameworkruntime "github.com/karmada-io/karmada/pkg/estimator/server/framework/runtime"
	"github.com/karmada-io/karmada/pkg/features"
)

const (
	fooNamespace         = "foo"
	barNamespace         = "bar"
	fooPriorityClassName = "foo-priority"
	barPriorityClassName = "bar-priority"
)

var (
	fooPrioritySelector = corev1.ScopedResourceSelectorRequirement{
		ScopeName: corev1.ResourceQuotaScopePriorityClass,
		Operator:  corev1.ScopeSelectorOpIn,
		Values:    []string{fooPriorityClassName},
	}
	barPrioritySelector = corev1.ScopedResourceSelectorRequirement{
		ScopeName: corev1.ResourceQuotaScopePriorityClass,
		Operator:  corev1.ScopeSelectorOpIn,
		Values:    []string{barPriorityClassName},
	}
	hardResourceList = corev1.ResourceList{
		"cpu":            *resource.NewQuantity(1, resource.DecimalSI),
		"memory":         *resource.NewQuantity(4*(1024*1024), resource.DecimalSI),
		"nvidia.com/gpu": *resource.NewQuantity(5, resource.DecimalSI),
	}
	usedResourceList = corev1.ResourceList{
		"cpu":            *resource.NewMilliQuantity(200, resource.DecimalSI),
		"memory":         *resource.NewQuantity(1024*1024, resource.DecimalSI),
		"nvidia.com/gpu": *resource.NewQuantity(2, resource.DecimalSI),
	}
	hardLimitRequestResourceList = corev1.ResourceList{
		"limits.cpu":              *resource.NewQuantity(1, resource.DecimalSI),
		"limits.memory":           *resource.NewQuantity(4*(1024*1024), resource.DecimalSI),
		"limits.nvidia.com/gpu":   *resource.NewQuantity(5, resource.DecimalSI),
		"requests.cpu":            *resource.NewQuantity(1, resource.DecimalSI),
		"requests.memory":         *resource.NewQuantity(4*(1024*1024), resource.DecimalSI),
		"requests.nvidia.com/gpu": *resource.NewQuantity(5, resource.DecimalSI),
	}
	usedLimitRequestResourceList = corev1.ResourceList{
		"limits.cpu":              *resource.NewQuantity(500, resource.DecimalSI),
		"limits.memory":           *resource.NewQuantity(3*(1024*1024), resource.DecimalSI),
		"limits.nvidia.com/gpu":   *resource.NewQuantity(4, resource.DecimalSI),
		"requests.cpu":            *resource.NewMilliQuantity(200, resource.DecimalSI),
		"requests.memory":         *resource.NewQuantity(1024*1024, resource.DecimalSI),
		"requests.nvidia.com/gpu": *resource.NewQuantity(2, resource.DecimalSI),
	}
	fooResourceQuota = &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: fooNamespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hardResourceList,
			ScopeSelector: &corev1.ScopeSelector{
				MatchExpressions: []corev1.ScopedResourceSelectorRequirement{fooPrioritySelector},
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: hardResourceList,
			Used: usedResourceList,
		},
	}
	barResourceQuota = &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: barNamespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hardLimitRequestResourceList,
			ScopeSelector: &corev1.ScopeSelector{
				MatchExpressions: []corev1.ScopedResourceSelectorRequirement{barPrioritySelector},
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: hardLimitRequestResourceList,
			Used: usedLimitRequestResourceList,
		},
	}
	multipleSelectorScopesResourceQuota = &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: fooNamespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hardResourceList,
			ScopeSelector: &corev1.ScopeSelector{
				MatchExpressions: []corev1.ScopedResourceSelectorRequirement{
					fooPrioritySelector,
					barPrioritySelector,
				},
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: hardResourceList,
			Used: usedResourceList,
		},
	}
	noScopeSelectorResourceQuota = &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-scope",
			Namespace: fooNamespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hardResourceList,
			// No ScopeSelector - applies to all pods
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: hardResourceList,
			Used: usedResourceList,
		},
	}
)

type testContext struct {
	ctx             context.Context
	client          *fake.Clientset
	informerFactory informers.SharedInformerFactory
	p               *resourceQuotaEstimator
	enabled         bool
}

type expect struct {
	replica int32
	ret     *framework.Result
}

func setup(t *testing.T, resourceQuotaList []*corev1.ResourceQuota, enablePlugin bool) (result *testContext) {
	t.Helper()
	tc := &testContext{
		enabled: enablePlugin,
	}
	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)
	tc.ctx = ctx

	tc.client = fake.NewSimpleClientset()
	tc.informerFactory = informers.NewSharedInformerFactory(tc.client, 0)

	opts := []frameworkruntime.Option{
		frameworkruntime.WithInformerFactory(tc.informerFactory),
	}
	fh, err := frameworkruntime.NewFramework(nil, opts...)
	if err != nil {
		t.Fatal(err)
	}

	// override feature-gates
	runtimeutil.Must(utilfeature.DefaultMutableFeatureGate.Add(features.DefaultFeatureGates))
	err = features.FeatureGate.Set(fmt.Sprintf("%s=%t", features.ResourceQuotaEstimate, tc.enabled))
	require.NoError(t, err, "override feature-gates")

	pl, err := New(fh)
	if err != nil {
		t.Fatal(err)
	}
	tc.p = pl.(*resourceQuotaEstimator)

	for _, resourceQuota := range resourceQuotaList {
		_, err := tc.client.CoreV1().ResourceQuotas(resourceQuota.Namespace).Create(tc.ctx, resourceQuota, metav1.CreateOptions{})
		require.NoError(t, err, "create resourceQuota")
	}

	tc.informerFactory.Start(tc.ctx.Done())
	t.Cleanup(func() {
		for _, resourceQuota := range resourceQuotaList {
			err := tc.client.CoreV1().ResourceQuotas(resourceQuota.Namespace).Delete(tc.ctx, resourceQuota.Name, metav1.DeleteOptions{})
			require.NoError(t, err, "delete resourceQuota")
		}
		// Need to cancel before waiting for the shutdown.
		cancel()
		// Now we can wait for all goroutines to stop.
		tc.informerFactory.Shutdown()
	})
	tc.informerFactory.WaitForCacheSync(tc.ctx.Done())
	informersSynced := tc.informerFactory.WaitForCacheSync(tc.ctx.Done())
	for rtype, synced := range informersSynced {
		if !synced {
			require.NoError(t, err, "can't create lister", rtype.Name())
		}
	}
	return tc
}

func TestResourceQuotaEstimatorPlugin(t *testing.T) {
	tests := map[string]struct {
		replicaRequirements pb.ReplicaRequirements
		resourceQuotaList   []*corev1.ResourceQuota
		enabled             bool
		expect              expect
	}{
		"empty-resource-quota-list": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{},
			enabled:           true,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator found no quota constraints"),
			},
		},
		"resource-quota-evaluate-cpu-only": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				fooResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 4,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"resource-quota-evaluate-memory-only": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"memory": *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				fooResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"resource-quota-evaluate-extended-resource-only": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				fooResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 3,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"resource-quota-evaluate-not-supported-ephemeral-storage": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"ephemeral-storage": *resource.NewQuantity(1024*1024, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				fooResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator found no quota constraints"),
			},
		},
		"resource-quota-evaluate-all-unschedulable": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu":            *resource.NewQuantity(2, resource.DecimalSI),
					"memory":         *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				fooResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "zero replica is estimated by ResourceQuotaEstimator"),
			},
		},
		"resource-quota-evaluate-all-with-multiple-selector-scopes": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu":            *resource.NewQuantity(2, resource.DecimalSI),
					"memory":         *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				multipleSelectorScopesResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "zero replica is estimated by ResourceQuotaEstimator"),
			},
		},
		"request-resource-quota-evaluate-all": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu":            *resource.NewMilliQuantity(200, resource.DecimalSI),
					"memory":         *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
				},
				Namespace:         barNamespace,
				PriorityClassName: barPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				barResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"resource-quota-evaluate-all": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu":            *resource.NewMilliQuantity(200, resource.DecimalSI),
					"memory":         *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				fooResourceQuota,
			},
			enabled: true,
			expect: expect{
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"resource-quota-not-supported-scopes": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
				Namespace:         fooNamespace,
				PriorityClassName: fooPriorityClassName,
			},
			resourceQuotaList: []*corev1.ResourceQuota{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{
								{ScopeName: corev1.ResourceQuotaScopeTerminating},
								{ScopeName: corev1.ResourceQuotaScopeNotTerminating},
								{ScopeName: corev1.ResourceQuotaScopeBestEffort},
								{ScopeName: corev1.ResourceQuotaScopeNotBestEffort},
								{ScopeName: corev1.ResourceQuotaScopeCrossNamespacePodAffinity},
							},
						},
					},
				},
			},
			enabled: true,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator found no quota constraints"),
			},
		},
		"feature-gate-disabled": {
			enabled: false,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Noopperation, "ResourceQuotaEstimator is disabled"),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testCtx := setup(t, tt.resourceQuotaList, tt.enabled)
			requirement := tt.replicaRequirements
			replica, ret := testCtx.p.Estimate(testCtx.ctx, nil, &requirement)

			require.Equal(t, tt.expect.ret.Code(), ret.Code())
			assert.ElementsMatch(t, tt.expect.ret.Reasons(), ret.Reasons())
			require.Equal(t, tt.expect.replica, replica)
		})
	}
}

func TestResourceQuotaEstimator_EstimateComponents(t *testing.T) {
	tests := map[string]struct {
		resourceQuotaList []*corev1.ResourceQuota
		components        []pb.Component
		namespace         string
		enabled           bool
		expect            expect
	}{
		// ============================================
		// Plugin-level edge cases
		// ============================================
		"feature-gate-disabled": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   false,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Noopperation, "ResourceQuotaEstimator is disabled"),
			},
		},
		"empty-components-list": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components:        []pb.Component{},
			enabled:           true,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator received empty components list"),
			},
		},
		"no-resource-quota-in-namespace": {
			resourceQuotaList: []*corev1.ResourceQuota{}, // Empty list
			components: []pb.Component{
				{
					Name: "app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator found no quota constraints"),
			},
		},
		"priority-class-scope-mismatch": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: barPriorityClassName, // Mismatch
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator found no quota constraints"),
			},
		},
		"empty-priority-class-name": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: "", // Empty priority class
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// fooResourceQuota has PriorityClass scope selector
				// Empty priorityClassName won't match the scope selector
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Success, "ResourceQuotaEstimator found no quota constraints"),
			},
		},
		"empty-priority-class-name-with-no-scope-quota": {
			resourceQuotaList: []*corev1.ResourceQuota{noScopeSelectorResourceQuota},
			components: []pb.Component{
				{
					Name: "app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: "", // Empty priority class
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// noScopeSelectorResourceQuota has no scope selector - applies to all pods
				// Available: 800m CPU (1000m - 200m)
				// Per set: 100m CPU
				// Max sets: 800m/100m = 8
				replica: 8,
				ret:     framework.NewResult(framework.Success),
			},
		},

		// ============================================
		// Resource aggregation and filtering
		// ============================================
		"single-component-basic": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "webserver",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(500*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// Available: 800m CPU (1000m - 200m), 3MB memory (4MB - 1MB)
				// Per set: 100m CPU, 0.5MB memory
				// Max sets: min(800m/100m, 3MB/0.5MB) = min(8, 6) = 6
				replica: 6,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"multi-component-complex-aggregation": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "app1",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(50, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(200*1024, resource.DecimalSI),
						},
					},
					Replicas: 3,
				},
				{
					Name: "app2",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(500*1024, resource.DecimalSI),
						},
					},
					Replicas: 2,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// Per set: (50m*3 + 100m*2) = 350m CPU, (200*1024*3 + 500*1024*2) = 1,638,400 bytes = 1600KB memory
				// Available: 800m CPU, 3MB (3,145,728 bytes = 3072KB) memory
				// Max sets: min(800m/350m, 3072KB/1600KB) = min(2.28, 1.92) = 1
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},

		// ============================================
		// Resource constraint bottlenecks
		// ============================================
		"memory-bottleneck": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "memory-intensive",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(50, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(2*1024*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// Available: 800m CPU, 3MB memory
				// Per set: 50m CPU, 2MB memory
				// Max sets: min(800m/50m, 3MB/2MB) = min(16, 1) = 1
				// Tests single-resource bottleneck scenario
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"gpu-extended-resource-bottleneck": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "ml-worker",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:                    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory:                 *resource.NewQuantity(500*1024, resource.DecimalSI),
							corev1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(1, resource.DecimalSI),
						},
					},
					Replicas: 2,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// Available: 800m CPU, 3MB memory, 3 GPUs (5 - 2)
				// Per set: 200m CPU, 1MB memory, 2 GPUs
				// Max sets: min(800m/200m, 3MB/1MB, 3/2) = min(4, 3, 1) = 1
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"quota-exhausted-zero-sets": {
			resourceQuotaList: []*corev1.ResourceQuota{fooResourceQuota},
			components: []pb.Component{
				{
					Name: "large-app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(10, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(10*1024*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// Per set: 10000m CPU, 10MB memory
				// Available: 800m CPU, 3MB memory
				// Both constraints violated → 0 sets
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "zero component sets estimated by ResourceQuotaEstimator"),
			},
		},

		// ============================================
		// Multiple quotas and special cases
		// ============================================
		"multiple-quotas-minimum-constraint": {
			resourceQuotaList: []*corev1.ResourceQuota{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpu-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							"cpu": *resource.NewQuantity(1, resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{fooPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							"cpu": *resource.NewQuantity(1, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "memory-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							"memory": *resource.NewQuantity(3*(1024*1024), resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{fooPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							"memory": *resource.NewQuantity(3*(1024*1024), resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							"memory": *resource.NewQuantity(1024*1024, resource.DecimalSI),
						},
					},
				},
			},
			components: []pb.Component{
				{
					Name: "app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(1024*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// CPU quota: 800m / 200m = 4 sets
				// Memory quota: 2MB / 1MB = 2 sets
				// Minimum: 2 sets
				replica: 2,
				ret:     framework.NewResult(framework.Success),
			},
		},

		// ============================================
		// Multi-priority component sets
		// ============================================
		"multiple-priorities-different-quotas": {
			resourceQuotaList: []*corev1.ResourceQuota{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-priority-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{fooPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(1024*1024, resource.DecimalSI),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar-priority-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(2*1024*1024, resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{barPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(2*1024*1024, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(512*1024, resource.DecimalSI),
						},
					},
				},
			},
			components: []pb.Component{
				{
					Name: "frontend",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(300, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(500*1024, resource.DecimalSI),
						},
					},
					Replicas: 2,
				},
				{
					Name: "backend",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: barPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(400*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// foo-priority group (frontend): 2 replicas × 300m = 600m CPU, 2 × 500KB = 1MB memory per set
				//   Available: 1800m CPU, 3MB memory
				//   Max sets: min(1800m/600m, 3MB/1MB) = min(3, 3) = 3
				//
				// bar-priority group (backend): 1 replica × 200m = 200m CPU, 1 × 400KB = 400KB memory per set
				//   Available: 900m CPU, 1.5MB memory
				//   Max sets: min(900m/200m, 1.5MB/400KB) = min(4, 3) = 3
				//
				// Final result: min(3, 3) = 3
				replica: 3,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"multiple-priorities-one-group-bottleneck": {
			resourceQuotaList: []*corev1.ResourceQuota{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-priority-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{fooPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(1600, resource.DecimalSI), // High usage
							corev1.ResourceMemory: *resource.NewQuantity(1024*1024, resource.DecimalSI),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar-priority-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{barPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI), // Low usage
							corev1.ResourceMemory: *resource.NewQuantity(512*1024, resource.DecimalSI),
						},
					},
				},
			},
			components: []pb.Component{
				{
					Name: "high-priority-app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(300, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(500*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
				{
					Name: "low-priority-app",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: barPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(200*1024, resource.DecimalSI),
						},
					},
					Replicas: 1,
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// foo-priority group (high-priority-app): 300m CPU, 500KB memory per set
				//   Available: 400m CPU, 3MB memory
				//   Max sets: min(400m/300m, 3MB/500KB) = min(1, 6) = 1
				//
				// bar-priority group (low-priority-app): 100m CPU, 200KB memory per set
				//   Available: 1900m CPU, 3.5MB memory
				//   Max sets: min(1900m/100m, 3.5MB/200KB) = min(19, 17) = 17
				//
				// Final result: min(1, 17) = 1 (bottlenecked by foo-priority group)
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		"mixed-scope-and-no-scope-quotas-prevents-over-admission": {
			// This test validates the fix for the over-admission bug with mixed quota scopes.
			// It tests the most complex scenario:
			// 1. A no-scope quota that applies to ALL components
			// 2. A scoped quota that applies only to high-priority components
			// The algorithm must correctly:
			// - Aggregate ALL components when evaluating the no-scope quota
			// - Filter only matching components when evaluating the scoped quota
			// - Return the minimum (most restrictive) result
			resourceQuotaList: []*corev1.ResourceQuota{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "global-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI), // 1000m CPU total for ALL pods
							corev1.ResourceMemory: *resource.NewQuantity(2*1024*1024, resource.DecimalSI),
						},
						// No ScopeSelector - applies to ALL pods
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(2*1024*1024, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(0, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "high-priority-quota",
						Namespace: fooNamespace,
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI), // 2000m CPU for high-priority only
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						ScopeSelector: &corev1.ScopeSelector{
							MatchExpressions: []corev1.ScopedResourceSelectorRequirement{fooPrioritySelector},
						},
					},
					Status: corev1.ResourceQuotaStatus{
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(4*1024*1024, resource.DecimalSI),
						},
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(0, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(0, resource.DecimalSI),
						},
					},
				},
			},
			components: []pb.Component{
				{
					Name: "high-priority-component",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: fooPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(600, resource.DecimalSI), // 600m per replica
							corev1.ResourceMemory: *resource.NewQuantity(1*1024*1024, resource.DecimalSI),
						},
					},
					Replicas: 1, // 600m CPU per set
				},
				{
					Name: "low-priority-component",
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						PriorityClassName: barPriorityClassName,
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI), // 500m per replica
							corev1.ResourceMemory: *resource.NewQuantity(1*1024*1024, resource.DecimalSI),
						},
					},
					Replicas: 1, // 500m CPU per set
				},
			},
			namespace: fooNamespace,
			enabled:   true,
			expect: expect{
				// Evaluation against global-quota (no scope - applies to ALL components):
				//   Aggregate: 600m (high-priority) + 500m (low-priority) = 1100m CPU per set
				//   Available: 1000m CPU
				//   Result: 1000m / 1100m = 0 sets
				//
				// Evaluation against high-priority-quota (scoped - applies ONLY to high-priority):
				//   Filter: Only high-priority-component matches
				//   Aggregate: 600m CPU per set
				//   Available: 2000m CPU
				//   Result: 2000m / 600m = 3 sets
				//
				// Final result: min(0, 3) = 0 sets (global-quota is the bottleneck)
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "zero component sets estimated by ResourceQuotaEstimator"),
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testCtx := setup(t, tt.resourceQuotaList, tt.enabled)
			sets, ret := testCtx.p.EstimateComponents(testCtx.ctx, nil, tt.components, tt.namespace)

			require.Equal(t, tt.expect.ret.Code(), ret.Code())
			assert.ElementsMatch(t, tt.expect.ret.Reasons(), ret.Reasons())
			require.Equal(t, tt.expect.replica, sets)
		})
	}
}
