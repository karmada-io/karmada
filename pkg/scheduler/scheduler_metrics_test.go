/*
Copyright 2022 The Karmada Authors.

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

package scheduler

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics/testutil"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	internalqueue "github.com/karmada-io/karmada/pkg/scheduler/internal/queue"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
)

const incomingBindingMetricsName = "queue_incoming_bindings_total"

type trigger func(scheduler *Scheduler, obj interface{})

var (
	addBinding = func(scheduler *Scheduler, obj interface{}) {
		scheduler.onResourceBindingAdd(obj)
	}
	updateBinding = func(scheduler *Scheduler, obj interface{}) {
		oldRB := &workv1alpha2.ResourceBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ResourceBinding",
				APIVersion: workv1alpha2.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
		}
		scheduler.onResourceBindingUpdate(oldRB, obj)
	}
	policyChanged = func(scheduler *Scheduler, obj interface{}) {
		rb := obj.(*workv1alpha2.ResourceBinding)
		scheduler.onResourceBindingRequeue(rb, metrics.PolicyChanged)
	}
	crbPolicyChanged = func(scheduler *Scheduler, obj interface{}) {
		crb := obj.(*workv1alpha2.ClusterResourceBinding)
		scheduler.onClusterResourceBindingRequeue(crb, metrics.PolicyChanged)
	}
	clusterChanged = func(scheduler *Scheduler, obj interface{}) {
		rb := obj.(*workv1alpha2.ResourceBinding)
		scheduler.onResourceBindingRequeue(rb, metrics.ClusterChanged)
	}
	crbClusterChanged = func(scheduler *Scheduler, obj interface{}) {
		crb := obj.(*workv1alpha2.ClusterResourceBinding)
		scheduler.onClusterResourceBindingRequeue(crb, metrics.ClusterChanged)
	}
	scheduleAttemptSuccess = func(scheduler *Scheduler, obj interface{}) {
		rb := obj.(*workv1alpha2.ResourceBinding)
		scheduler.handleErr(nil, &internalqueue.QueuedBindingInfo{
			NamespacedKey: cache.ObjectName{Namespace: rb.Namespace, Name: rb.Namespace}.String(),
			Priority:      rb.Spec.SchedulePriorityValue(),
		})
	}
	scheduleAttemptFailure = func(scheduler *Scheduler, obj interface{}) {
		rb := obj.(*workv1alpha2.ResourceBinding)
		scheduler.handleErr(fmt.Errorf("schedule attempt failure"), &internalqueue.QueuedBindingInfo{
			NamespacedKey: cache.ObjectName{Namespace: rb.Namespace, Name: rb.Namespace}.String(),
			Priority:      rb.Spec.SchedulePriorityValue(),
		})
	}
)

func TestIncomingBindingMetrics(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()

	// enable "PriorityBasedScheduling" feature gate.
	_ = features.FeatureGate.Set("PriorityBasedScheduling=true")
	sche, err := NewScheduler(dynamicClient, karmadaClient, kubeClient)
	if err != nil {
		t.Errorf("create scheduler error: %s", err)
	}

	var rbInfos = make([]*workv1alpha2.ResourceBinding, 0, 3)
	var crbInfos = make([]*workv1alpha2.ClusterResourceBinding, 0, 3)
	for i := 1; i <= 3; i++ {
		rb := &workv1alpha2.ResourceBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ResourceBinding",
				APIVersion: workv1alpha2.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       fmt.Sprintf("test-rb-%d", i),
				Namespace:  "bar",
				Generation: 2,
			},
		}
		rbInfos = append(rbInfos, rb)
	}

	for i := 1; i <= 3; i++ {
		crb := &workv1alpha2.ClusterResourceBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterResourceBinding",
				APIVersion: workv1alpha2.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       fmt.Sprintf("test-rb-%d", i),
				Generation: 2,
			},
		}
		crbInfos = append(crbInfos, crb)
	}

	rbTests := []struct {
		name        string
		crbInvolved bool
		trigger     trigger
		want        string
	}{
		{
			name:        "add resourceBinding",
			crbInvolved: false,
			trigger:     addBinding,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="BindingAdd"} 3
`,
		},
		{
			name:        "update resourceBinding",
			crbInvolved: false,
			trigger:     updateBinding,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="BindingUpdate"} 3
`,
		},
		{
			name:        "policy associated with resourceBinding changed",
			crbInvolved: false,
			trigger:     policyChanged,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="PolicyChanged"} 3
`,
		},
		{
			name:        "policy associated with clusterResourceBinding changed",
			crbInvolved: true,
			trigger:     crbPolicyChanged,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="PolicyChanged"} 3
`,
		},
		{
			name:        "cluster which matches the existing propagationPolicy changed",
			crbInvolved: false,
			trigger:     clusterChanged,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="ClusterChanged"} 3
`,
		},
		{
			name:        "cluster which matches the existing clusterPropagationPolicy changed",
			crbInvolved: true,
			trigger:     crbClusterChanged,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="ClusterChanged"} 3
`,
		},
		{
			name:        "schedule attempt success",
			crbInvolved: false,
			trigger:     scheduleAttemptSuccess,
			want:        ``,
		},
		{
			name:        "schedule attempt failure",
			crbInvolved: false,
			trigger:     scheduleAttemptFailure,
			want: `
			# HELP karmada_scheduler_queue_incoming_bindings_total Number of ResourceBinding and ClusterResourceBinding objects added to scheduling queues by event type.
			# TYPE karmada_scheduler_queue_incoming_bindings_total counter
			karmada_scheduler_queue_incoming_bindings_total{event="ScheduleAttemptFailure"} 3
`,
		},
	}

	for _, test := range rbTests {
		t.Run(test.name, func(t *testing.T) {
			metrics.SchedulerQueueIncomingBindings.Reset()
			if test.crbInvolved {
				for _, crbInfo := range crbInfos {
					test.trigger(sche, crbInfo)
				}
			} else {
				for _, rbInfo := range rbInfos {
					test.trigger(sche, rbInfo)
				}
			}
			metricName := metrics.SchedulerSubsystem + "_" + incomingBindingMetricsName
			if err := testutil.CollectAndCompare(metrics.SchedulerQueueIncomingBindings, strings.NewReader(test.want), metricName); err != nil {
				t.Errorf("unexpected collecting result:\n%s", err)
			}
		})
	}
}
