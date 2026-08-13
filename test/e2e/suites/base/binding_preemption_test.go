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

package base

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = framework.SerialDescribe("[BindingPreemption] binding-level priority preemption", func() {
	const (
		lowPriority  int32 = 100
		highPriority int32 = 1000
	)

	var namespace, targetCluster string
	var lowPriorityClass, highPreemptingPriorityClass *schedulingv1.PriorityClass

	ginkgo.BeforeEach(func() {
		targetCluster = framework.ClusterNames()[0]

		namespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
		err := setupTestNamespace(namespace, kubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		ginkgo.DeferCleanup(func() {
			framework.RemoveNamespace(kubeClient, namespace)
		})

		lowPriorityClass = newBindingPreemptionPriorityClass("binding-preemption-low", lowPriority, ptr.To(corev1.PreemptNever))
		highPreemptingPriorityClass = newBindingPreemptionPriorityClass("binding-preemption-high", highPriority, ptr.To(corev1.PreemptLowerPriority))
		framework.CreatePriorityClass(kubeClient, lowPriorityClass)
		framework.CreatePriorityClass(kubeClient, highPreemptingPriorityClass)
		ginkgo.DeferCleanup(func() {
			framework.RemovePriorityClass(kubeClient, highPreemptingPriorityClass.Name)
			framework.RemovePriorityClass(kubeClient, lowPriorityClass.Name)
		})

		createBindingPreemptionResourceQuota(namespace, targetCluster)
	})

	ginkgo.It("preempts lower-priority bindings and schedules the higher-priority binding", func() {
		lowDeploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		lowBindingName := createBindingPreemptionDeployment(namespace, lowDeploymentName, targetCluster, lowPriorityClass.Name, nil)
		framework.AssertBindingScheduledClusters(karmadaClient, namespace, lowBindingName, [][]string{{targetCluster}})
		framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, namespace, lowDeploymentName, func(deployment *appsv1.Deployment) bool {
			return framework.CheckDeploymentReadyStatus(deployment, *deployment.Spec.Replicas)
		})

		highDeploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		highBindingName := createBindingPreemptionDeployment(namespace, highDeploymentName, targetCluster, highPreemptingPriorityClass.Name, nil)

		ginkgo.By("verifying the lower-priority binding is selected as the preemption victim", func() {
			framework.WaitResourceBindingFitWith(karmadaClient, namespace, lowBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				return hasBindingPreemptionEvictionTask(binding, targetCluster)
			})
			framework.WaitEventFitWith(kubeClient, namespace, highBindingName, func(event corev1.Event) bool {
				return event.Reason == events.EventReasonPreemptionInitiated
			})
			framework.WaitEventFitWith(kubeClient, namespace, lowBindingName, func(event corev1.Event) bool {
				return event.Reason == events.EventReasonBindingPreempted
			})
		})

		ginkgo.By("verifying the preemptor schedules after the victim is gracefully evicted", func() {
			framework.WaitResourceBindingFitWith(karmadaClient, namespace, highBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
				return cond != nil && cond.Status == metav1.ConditionTrue
			})
			framework.AssertBindingScheduledClusters(karmadaClient, namespace, highBindingName, [][]string{{targetCluster}})
			framework.WaitGracefulEvictionTasksDone(karmadaClient, namespace, lowBindingName)
		})
	})

	ginkgo.DescribeTable("does not preempt when an eligibility requirement is missing",
		func(preemptionPolicy *corev1.PreemptionPolicy, mutatePlacement func(*policyv1alpha1.Placement)) {
			lowDeploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
			lowBindingName := createBindingPreemptionDeployment(namespace, lowDeploymentName, targetCluster, lowPriorityClass.Name, nil)
			framework.AssertBindingScheduledClusters(karmadaClient, namespace, lowBindingName, [][]string{{targetCluster}})

			highPriorityClass := newBindingPreemptionPriorityClass("binding-preemption-blocked", highPriority, preemptionPolicy)
			framework.CreatePriorityClass(kubeClient, highPriorityClass)
			ginkgo.DeferCleanup(func() {
				framework.RemovePriorityClass(kubeClient, highPriorityClass.Name)
			})

			highDeploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
			highBindingName := createBindingPreemptionDeployment(namespace, highDeploymentName, targetCluster, highPriorityClass.Name, mutatePlacement)

			framework.WaitResourceBindingFitWith(karmadaClient, namespace, highBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
				if cond == nil {
					return false
				}
				return cond.Status == metav1.ConditionTrue ||
					(cond.Status == metav1.ConditionFalse && cond.Reason != workv1alpha2.BindingReasonPreempting)
			})
			gomega.Consistently(func(g gomega.Gomega) bool {
				binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(context.TODO(), lowBindingName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				return len(binding.Spec.GracefulEvictionTasks) == 0
			}, 20*time.Second, pollInterval).Should(gomega.BeTrue())
		},
		ginkgo.Entry("preemptionPolicy is unset", (*corev1.PreemptionPolicy)(nil), nil),
		ginkgo.Entry("replica scheduling is Duplicated", ptr.To(corev1.PreemptLowerPriority), func(placement *policyv1alpha1.Placement) {
			placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			}
		}),
		ginkgo.Entry("replica scheduling uses static weight", ptr.To(corev1.PreemptLowerPriority), func(placement *policyv1alpha1.Placement) {
			placement.ReplicaScheduling = helper.NewStaticWeightPolicyStrategy([]string{targetCluster}, []int64{1})
		}),
		ginkgo.Entry("placement uses ClusterAffinities", ptr.To(corev1.PreemptLowerPriority), func(placement *policyv1alpha1.Placement) {
			placement.ClusterAffinity = nil
			placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{
				AffinityName: "target",
				ClusterAffinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetCluster},
				},
			}}
		}),
		ginkgo.Entry("cluster spread allows multiple groups", ptr.To(corev1.PreemptLowerPriority), func(placement *policyv1alpha1.Placement) {
			placement.SpreadConstraints[0].MaxGroups = 2
		}),
	)
})

func newBindingPreemptionPriorityClass(namePrefix string, value int32, preemptionPolicy *corev1.PreemptionPolicy) *schedulingv1.PriorityClass {
	priorityClass := helper.NewPriorityClass(fmt.Sprintf("%s-%s", namePrefix, rand.String(RandomStrLength)), value)
	priorityClass.PreemptionPolicy = preemptionPolicy
	return priorityClass
}

func createBindingPreemptionResourceQuota(namespace, targetCluster string) {
	quotaName := resourceQuotaPrefix + rand.String(RandomStrLength)
	quota := &corev1.ResourceQuota{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ResourceQuota",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU: resource.MustParse("30m"),
			},
		},
	}
	framework.CreateResourceQuota(kubeClient, quota)
	ginkgo.DeferCleanup(func() {
		framework.RemoveResourceQuota(kubeClient, namespace, quotaName)
	})

	policy := helper.NewPropagationPolicy(namespace, ppNamePrefix+rand.String(RandomStrLength),
		[]policyv1alpha1.ResourceSelector{{
			APIVersion: quota.APIVersion,
			Kind:       quota.Kind,
			Name:       quota.Name,
		}},
		policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetCluster},
			},
		})
	framework.CreatePropagationPolicy(karmadaClient, policy)
	ginkgo.DeferCleanup(func() {
		framework.RemovePropagationPolicy(karmadaClient, namespace, policy.Name)
	})

	framework.WaitResourceQuotaPresentOnCluster(targetCluster, namespace, quotaName)
}

func createBindingPreemptionDeployment(namespace, deploymentName, targetCluster, priorityClassName string, mutatePlacement func(*policyv1alpha1.Placement)) string {
	deployment := helper.NewDeployment(namespace, deploymentName)
	policy := helper.NewPropagationPolicy(namespace, ppNamePrefix+rand.String(RandomStrLength),
		[]policyv1alpha1.ResourceSelector{{
			APIVersion: deployment.APIVersion,
			Kind:       deployment.Kind,
			Name:       deployment.Name,
		}},
		bindingPreemptionPlacement(targetCluster))
	policy.Spec.SchedulePriority = &policyv1alpha1.SchedulePriority{
		PriorityClassName:   priorityClassName,
		PriorityClassSource: policyv1alpha1.KubePriorityClass,
	}
	if mutatePlacement != nil {
		mutatePlacement(&policy.Spec.Placement)
	}

	framework.CreatePropagationPolicy(karmadaClient, policy)
	framework.CreateDeployment(kubeClient, deployment)
	ginkgo.DeferCleanup(func() {
		framework.RemoveDeployment(kubeClient, namespace, deploymentName)
		framework.RemovePropagationPolicy(karmadaClient, namespace, policy.Name)
	})

	return names.GenerateBindingName(util.DeploymentKind, deploymentName)
}

func bindingPreemptionPlacement(targetCluster string) policyv1alpha1.Placement {
	return policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: []string{targetCluster},
		},
		SpreadConstraints: []policyv1alpha1.SpreadConstraint{
			{
				SpreadByField: policyv1alpha1.SpreadByFieldCluster,
				MinGroups:     1,
				MaxGroups:     1,
			},
		},
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
		},
	}
}

func hasBindingPreemptionEvictionTask(binding *workv1alpha2.ResourceBinding, targetCluster string) bool {
	for _, task := range binding.Spec.GracefulEvictionTasks {
		if task.FromCluster == targetCluster &&
			task.Producer == workv1alpha2.EvictionProducerScheduler &&
			task.Reason == workv1alpha2.EvictionReasonBindingPreempted &&
			task.GracePeriodSeconds != nil &&
			*task.GracePeriodSeconds == 30 {
			return true
		}
	}
	return false
}
