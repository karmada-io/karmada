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
				ret:     framework.NewResult(framework.Noopperation, "ResourceQuotaEstimator has no operation on input replicaRequirements"),
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
				ret:     framework.NewResult(framework.Noopperation, "ResourceQuotaEstimator has no operation on input replicaRequirements"),
			},
		},
		"resource-quota-evaluate-all-unschedulable": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu":               *resource.NewQuantity(2, resource.DecimalSI),
					"memory":            *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu":    *resource.NewQuantity(1, resource.DecimalSI),
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
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "zero replica is estimated by ResourceQuotaEstimator"),
			},
		},
		"resource-quota-evaluate-all-with-multiple-selector-scopes": {
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu":               *resource.NewQuantity(2, resource.DecimalSI),
					"memory":            *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu":    *resource.NewQuantity(1, resource.DecimalSI),
					"ephemeral-storage": *resource.NewQuantity(1024*1024, resource.DecimalSI),
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
					"cpu":               *resource.NewMilliQuantity(200, resource.DecimalSI),
					"memory":            *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu":    *resource.NewQuantity(1, resource.DecimalSI),
					"ephemeral-storage": *resource.NewQuantity(1024*1024, resource.DecimalSI),
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
					"cpu":               *resource.NewMilliQuantity(200, resource.DecimalSI),
					"memory":            *resource.NewQuantity(2*(1024*1024), resource.DecimalSI),
					"nvidia.com/gpu":    *resource.NewQuantity(1, resource.DecimalSI),
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
				ret:     framework.NewResult(framework.Noopperation, "ResourceQuotaEstimator has no operation on input replicaRequirements"),
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
