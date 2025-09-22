package cluster

import (
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	gmtesting "github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager/testing"
)

// fakeGenericLister implements cache.GenericLister for tests.
// It returns pre-configured objects or error in List.
type fakeGenericLister struct {
	objects []runtime.Object
	err     error
}

func (f *fakeGenericLister) List(_ labels.Selector) ([]runtime.Object, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.objects, nil
}

func (f *fakeGenericLister) Get(_ string) (runtime.Object, error)              { return nil, nil }
func (f *fakeGenericLister) ByNamespace(_ string) cache.GenericNamespaceLister { return nil }

func makeCluster(name string, ready bool) *clusterv1alpha1.Cluster {
	condStatus := metav1.ConditionFalse
	if ready {
		condStatus = metav1.ConditionTrue
	}
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{{
				Type:               string(clusterv1alpha1.ClusterConditionReady),
				Status:             condStatus,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
}

func TestDynamicRateLimiter_When_Scenarios(t *testing.T) {
	clusterGVR := clusterv1alpha1.SchemeGroupVersion.WithResource("clusters")

	// helper to compute expected duration from rate
	expectedDur := func(rate float32) time.Duration {
		if rate == 0 {
			return maxEvictionDelay
		}
		return time.Duration(1 / rate * float32(time.Second))
	}

	tests := []struct {
		name     string
		objs     []runtime.Object
		listErr  error
		opts     EvictionQueueOptions
		expected time.Duration
	}{
		{
			name:     "no clusters => use default rate",
			objs:     nil,
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 50, SecondaryResourceEvictionRate: 5, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: expectedDur(50),
		},
		{
			name:     "healthy => default rate",
			objs:     []runtime.Object{makeCluster("c1", true), makeCluster("c2", true)},
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 20, SecondaryResourceEvictionRate: 2, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: expectedDur(20),
		},
		{
			name: "unhealthy large-scale => secondary rate",
			objs: func() []runtime.Object {
				var out []runtime.Object
				// 11 clusters total (> threshold 10), 5 unhealthy -> failureRate ~0.45 > 0.3
				for i := 0; i < 6; i++ {
					out = append(out, makeCluster("h"+string(rune('a'+i)), true))
				}
				for i := 0; i < 5; i++ {
					out = append(out, makeCluster("u"+string(rune('a'+i)), false))
				}
				return out
			}(),
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 30, SecondaryResourceEvictionRate: 3, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: expectedDur(3),
		},
		{
			name:     "unhealthy small-scale => halt (max delay)",
			objs:     []runtime.Object{makeCluster("c1", false), makeCluster("c2", true)}, // 1/2 unhealthy -> 0.5 > 0.3, total=2 <= threshold
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 10, SecondaryResourceEvictionRate: 1, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: maxEvictionDelay,
		},
		{
			name:     "lister error => halt",
			objs:     nil,
			listErr:  errors.New("boom"),
			opts:     EvictionQueueOptions{ResourceEvictionRate: 10, SecondaryResourceEvictionRate: 1, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: maxEvictionDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := gmtesting.NewFakeSingleClusterManager(true, true, func(gvr schema.GroupVersionResource) cache.GenericLister {
				if gvr != clusterGVR {
					return nil
				}
				return &fakeGenericLister{objects: tt.objs, err: tt.listErr}
			})
			limiter := NewDynamicRateLimiter[any](mgr, tt.opts)
			d := limiter.When(struct{}{})
			if d != tt.expected {
				t.Fatalf("unexpected duration: got %v, want %v", d, tt.expected)
			}
		})
	}
}
