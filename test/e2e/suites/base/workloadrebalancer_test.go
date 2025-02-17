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

package base

import (
	"sort"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

// test case dimension:
//
//	schedule strategy: static weight, dynamic weight, aggregated
//	resource type: workload type like deployment, non-workload type like clusterrole
//	expected result: successful, not found failure
var _ = framework.SerialDescribe("workload rebalancer testing", func() {
	var namespace string
	var deployName, newAddedDeployName, notExistDeployName, clusterroleName string
	var deployObjRef, newAddedDeployObjRef, notExistDeployObjRef, clusterroleObjRef appsv1alpha1.ObjectReference
	var deployBindingName, clusterroleBindingName string
	var cppName string
	var rebalancerName string

	var deploy, newAddedDeploy, notExistDeploy *appsv1.Deployment
	var clusterrole *rbacv1.ClusterRole
	var policy *policyv1alpha1.ClusterPropagationPolicy
	var rebalancer *appsv1alpha1.WorkloadRebalancer
	var targetClusters []string
	var taint corev1.Taint

	ginkgo.BeforeEach(func() {
		namespace = testNamespace
		randomStr := rand.String(RandomStrLength)
		deployName = deploymentNamePrefix + randomStr
		notExistDeployName = deployName + "-2"
		newAddedDeployName = deployName + "-3"
		clusterroleName = clusterRoleNamePrefix + rand.String(RandomStrLength)
		deployBindingName = names.GenerateBindingName(util.DeploymentKind, deployName)
		clusterroleBindingName = names.GenerateBindingName(util.ClusterRoleKind, clusterroleName)
		cppName = cppNamePrefix + randomStr
		rebalancerName = workloadRebalancerPrefix + randomStr

		// sort member clusters in increasing order
		targetClusters = framework.ClusterNames()[0:2]
		sort.Strings(targetClusters)
		taint = corev1.Taint{Key: "workload-rebalancer-test-" + randomStr, Effect: corev1.TaintEffectNoExecute}

		deploy = helper.NewDeployment(namespace, deployName)
		notExistDeploy = helper.NewDeployment(namespace, notExistDeployName)
		newAddedDeploy = helper.NewDeployment(namespace, newAddedDeployName)
		clusterrole = helper.NewClusterRole(clusterroleName, nil)
		policy = helper.NewClusterPropagationPolicy(cppName, []policyv1alpha1.ResourceSelector{
			{APIVersion: deploy.APIVersion, Kind: deploy.Kind, Name: deploy.Name, Namespace: deploy.Namespace},
			{APIVersion: newAddedDeploy.APIVersion, Kind: newAddedDeploy.Kind, Name: newAddedDeploy.Name, Namespace: newAddedDeploy.Namespace},
			{APIVersion: clusterrole.APIVersion, Kind: clusterrole.Kind, Name: clusterrole.Name},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: targetClusters},
		})

		deployObjRef = appsv1alpha1.ObjectReference{APIVersion: deploy.APIVersion, Kind: deploy.Kind, Name: deploy.Name, Namespace: deploy.Namespace}
		notExistDeployObjRef = appsv1alpha1.ObjectReference{APIVersion: notExistDeploy.APIVersion, Kind: notExistDeploy.Kind, Name: notExistDeploy.Name, Namespace: notExistDeploy.Namespace}
		newAddedDeployObjRef = appsv1alpha1.ObjectReference{APIVersion: newAddedDeploy.APIVersion, Kind: newAddedDeploy.Kind, Name: newAddedDeploy.Name, Namespace: newAddedDeploy.Namespace}
		clusterroleObjRef = appsv1alpha1.ObjectReference{APIVersion: clusterrole.APIVersion, Kind: clusterrole.Kind, Name: clusterrole.Name}

		rebalancer = helper.NewWorkloadRebalancer(rebalancerName, []appsv1alpha1.ObjectReference{deployObjRef, clusterroleObjRef, notExistDeployObjRef})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deploy)
		framework.CreateDeployment(kubeClient, newAddedDeploy)
		framework.CreateClusterRole(kubeClient, clusterrole)

		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deploy.Namespace, deploy.Name)
			framework.RemoveDeployment(kubeClient, newAddedDeploy.Namespace, newAddedDeploy.Name)
			framework.RemoveClusterRole(kubeClient, clusterrole.Name)
			framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
		})
	})

	var checkWorkloadRebalancerResult = func(expectedWorkloads []appsv1alpha1.ObservedWorkload) {
		// 1. check rebalancer status: match to `expectedWorkloads`.
		framework.WaitRebalancerObservedWorkloads(karmadaClient, rebalancerName, expectedWorkloads)
		// 2. check deploy: referenced binding's `spec.rescheduleTriggeredAt` and `status.lastScheduledTime` should be updated.
		framework.WaitResourceBindingFitWith(karmadaClient, namespace, deployBindingName, func(rb *workv1alpha2.ResourceBinding) bool {
			return bindingHasRescheduled(rb.Spec, rb.Status, rebalancer.CreationTimestamp)
		})
		// 3. check clusterrole: referenced binding's `spec.rescheduleTriggeredAt` and `status.lastScheduledTime` should be updated.
		framework.WaitClusterResourceBindingFitWith(karmadaClient, clusterroleBindingName, func(crb *workv1alpha2.ClusterResourceBinding) bool {
			return bindingHasRescheduled(crb.Spec, crb.Status, rebalancer.CreationTimestamp)
		})
	}

	// 1. dynamic weight scheduling
	ginkgo.Context("dynamic weight schedule type", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
				},
			}
			policy.Spec.Placement.ClusterTolerations = []corev1.Toleration{{
				Key:               taint.Key,
				Effect:            taint.Effect,
				Operator:          corev1.TolerationOpExists,
				TolerationSeconds: ptr.To[int64](0),
			}}
		})

		ginkgo.It("reschedule when policy is dynamic weight schedule type", func() {
			ginkgo.By("step1: check first schedule result", func() {
				// after first schedule, deployment is assigned as 1:2 or 2:1 in target clusters and clusterrole propagated to each cluster.
				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, [][]string{targetClusters})
				framework.WaitClusterRolePresentOnClustersFitWith(targetClusters, clusterroleName, func(_ *rbacv1.ClusterRole) bool { return true })
			})

			ginkgo.By("step2: add taints to cluster to mock cluster failure", func() {
				err := taintCluster(controlPlaneClient, targetClusters[0], taint)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, [][]string{targetClusters[1:]})
				framework.WaitGracefulEvictionTasksDone(karmadaClient, namespace, deployBindingName)

				err = recoverTaintedCluster(controlPlaneClient, targetClusters[0], taint)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("step3: trigger a reschedule by WorkloadRebalancer", func() {
				framework.CreateWorkloadRebalancer(karmadaClient, rebalancer)

				// actual replicas propagation of deployment should reschedule back to `targetClusters`,
				// which represents rebalancer changed deployment replicas propagation.
				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, [][]string{targetClusters})

				expectedWorkloads := []appsv1alpha1.ObservedWorkload{
					{Workload: deployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: notExistDeployObjRef, Result: appsv1alpha1.RebalanceFailed, Reason: appsv1alpha1.RebalanceObjectNotFound},
					{Workload: clusterroleObjRef, Result: appsv1alpha1.RebalanceSuccessful},
				}
				checkWorkloadRebalancerResult(expectedWorkloads)
			})

			ginkgo.By("step4: update WorkloadRebalancer spec workloads", func() {
				// update workload list from {deploy, clusterrole, notExistDeployObjRef} to {clusterroleObjRef, newAddedDeployObjRef}
				updatedWorkloads := []appsv1alpha1.ObjectReference{clusterroleObjRef, newAddedDeployObjRef}
				framework.UpdateWorkloadRebalancer(karmadaClient, rebalancerName, &updatedWorkloads, nil)

				expectedWorkloads := []appsv1alpha1.ObservedWorkload{
					{Workload: deployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: newAddedDeployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: clusterroleObjRef, Result: appsv1alpha1.RebalanceSuccessful},
				}
				framework.WaitRebalancerObservedWorkloads(karmadaClient, rebalancerName, expectedWorkloads)
			})

			ginkgo.By("step5: auto clean WorkloadRebalancer", func() {
				ttlSecondsAfterFinished := int32(5)
				framework.UpdateWorkloadRebalancer(karmadaClient, rebalancerName, nil, &ttlSecondsAfterFinished)

				framework.WaitRebalancerDisappear(karmadaClient, rebalancerName)
			})
		})

		ginkgo.It("create rebalancer with ttl and verify it can auto clean", func() {
			rebalancer.Spec.TTLSecondsAfterFinished = ptr.To[int32](5)
			framework.CreateWorkloadRebalancer(karmadaClient, rebalancer)
			framework.WaitRebalancerDisappear(karmadaClient, rebalancerName)
		})
	})

	// 2. static weight scheduling
	ginkgo.Context("static weight schedule type", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = helper.NewStaticWeightPolicyStrategy(targetClusters, []int64{2, 1})
			policy.Spec.Placement.ClusterTolerations = []corev1.Toleration{{
				Key:               taint.Key,
				Effect:            taint.Effect,
				Operator:          corev1.TolerationOpExists,
				TolerationSeconds: ptr.To[int64](0),
			}}
		})

		ginkgo.It("reschedule when policy is static weight schedule type", func() {
			ginkgo.By("step1: check first schedule result", func() {
				// after first schedule, deployment is assigned as 2:1 in target clusters and clusterrole propagated to each cluster.
				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, [][]string{targetClusters})
				framework.WaitClusterRolePresentOnClustersFitWith(targetClusters, clusterroleName, func(_ *rbacv1.ClusterRole) bool { return true })
			})

			ginkgo.By("step2: add taints to cluster to mock cluster failure", func() {
				err := taintCluster(controlPlaneClient, targetClusters[0], taint)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, [][]string{targetClusters[1:]})
				framework.WaitGracefulEvictionTasksDone(karmadaClient, namespace, deployBindingName)

				err = recoverTaintedCluster(controlPlaneClient, targetClusters[0], taint)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("step3: trigger a reschedule by WorkloadRebalancer", func() {
				framework.CreateWorkloadRebalancer(karmadaClient, rebalancer)

				// actual replicas propagation of deployment should reschedule back to `targetClusters`,
				// which represents rebalancer changed deployment replicas propagation.
				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, [][]string{targetClusters})

				expectedWorkloads := []appsv1alpha1.ObservedWorkload{
					{Workload: deployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: notExistDeployObjRef, Result: appsv1alpha1.RebalanceFailed, Reason: appsv1alpha1.RebalanceObjectNotFound},
					{Workload: clusterroleObjRef, Result: appsv1alpha1.RebalanceSuccessful},
				}
				checkWorkloadRebalancerResult(expectedWorkloads)
			})

			ginkgo.By("step4: update WorkloadRebalancer spec workloads", func() {
				// update workload list from {deploy, clusterrole, notExistDeployObjRef} to {clusterroleObjRef, newAddedDeployObjRef}
				updatedWorkloads := []appsv1alpha1.ObjectReference{clusterroleObjRef, newAddedDeployObjRef}
				framework.UpdateWorkloadRebalancer(karmadaClient, rebalancerName, &updatedWorkloads, nil)

				expectedWorkloads := []appsv1alpha1.ObservedWorkload{
					{Workload: deployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: newAddedDeployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: clusterroleObjRef, Result: appsv1alpha1.RebalanceSuccessful},
				}
				framework.WaitRebalancerObservedWorkloads(karmadaClient, rebalancerName, expectedWorkloads)
			})

			ginkgo.By("step5: auto clean WorkloadRebalancer", func() {
				ttlSecondsAfterFinished := int32(5)
				framework.UpdateWorkloadRebalancer(karmadaClient, rebalancerName, nil, &ttlSecondsAfterFinished)

				framework.WaitRebalancerDisappear(karmadaClient, rebalancerName)
			})
		})
	})

	// 3. aggregated scheduling
	ginkgo.Context("aggregated schedule type", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			}
		})

		ginkgo.It("reschedule when policy is aggregated schedule type", func() {
			ginkgo.By("step1: check first schedule result", func() {
				// after first schedule, deployment is assigned to exactly one of the target clusters while clusterrole propagated to each cluster.
				possibleScheduledClusters := getPossibleClustersInAggregatedScheduling(targetClusters)
				framework.AssertBindingScheduledClusters(karmadaClient, namespace, deployBindingName, possibleScheduledClusters)
				framework.WaitClusterRolePresentOnClustersFitWith(targetClusters, clusterroleName, func(_ *rbacv1.ClusterRole) bool { return true })
			})

			ginkgo.By("step2: trigger a reschedule by WorkloadRebalancer", func() {
				framework.CreateWorkloadRebalancer(karmadaClient, rebalancer)

				expectedWorkloads := []appsv1alpha1.ObservedWorkload{
					{Workload: deployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: notExistDeployObjRef, Result: appsv1alpha1.RebalanceFailed, Reason: appsv1alpha1.RebalanceObjectNotFound},
					{Workload: clusterroleObjRef, Result: appsv1alpha1.RebalanceSuccessful},
				}
				checkWorkloadRebalancerResult(expectedWorkloads)
			})

			ginkgo.By("step3: update WorkloadRebalancer spec workloads", func() {
				// update workload list from {deploy, clusterrole, notExistDeployObjRef} to {clusterroleObjRef, newAddedDeployObjRef}
				updatedWorkloads := []appsv1alpha1.ObjectReference{clusterroleObjRef, newAddedDeployObjRef}
				framework.UpdateWorkloadRebalancer(karmadaClient, rebalancerName, &updatedWorkloads, nil)

				expectedWorkloads := []appsv1alpha1.ObservedWorkload{
					{Workload: deployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: newAddedDeployObjRef, Result: appsv1alpha1.RebalanceSuccessful},
					{Workload: clusterroleObjRef, Result: appsv1alpha1.RebalanceSuccessful},
				}
				framework.WaitRebalancerObservedWorkloads(karmadaClient, rebalancerName, expectedWorkloads)
			})

			ginkgo.By("step4: auto clean WorkloadRebalancer", func() {
				ttlSecondsAfterFinished := int32(5)
				framework.UpdateWorkloadRebalancer(karmadaClient, rebalancerName, nil, &ttlSecondsAfterFinished)

				framework.WaitRebalancerDisappear(karmadaClient, rebalancerName)
			})
		})
	})
})

func bindingHasRescheduled(spec workv1alpha2.ResourceBindingSpec, status workv1alpha2.ResourceBindingStatus, rebalancerCreationTime metav1.Time) bool {
	if *spec.RescheduleTriggeredAt != rebalancerCreationTime || status.LastScheduledTime.Before(spec.RescheduleTriggeredAt) {
		klog.Errorf("rebalancerCreationTime: %+v, rescheduleTriggeredAt / lastScheduledTime: %+v / %+v",
			rebalancerCreationTime, *spec.RescheduleTriggeredAt, status.LastScheduledTime)
		return false
	}
	return true
}

func getPossibleClustersInAggregatedScheduling(targetClusters []string) [][]string {
	possibleScheduledClusters := make([][]string, 0)
	for _, cluster := range targetClusters {
		possibleScheduledClusters = append(possibleScheduledClusters, []string{cluster})
	}
	return possibleScheduledClusters
}
