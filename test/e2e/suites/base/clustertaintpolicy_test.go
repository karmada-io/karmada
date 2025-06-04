/*
Copyright 2025 The Karmada Authors.

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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/e2e/framework"
	karmadaresource "github.com/karmada-io/karmada/test/e2e/framework/resource/karmada"
)

// The following tests involve adding NoSchedule and NoExecute effect taints to clusters
// in the test environment, which will impact the resource scheduling of other concurrently
// executed test cases. Therefore, these tests need to be configured to run serially.
var _ = framework.SerialDescribe("Taint cluster with ClusterTaintPolicy", func() {
	var policyName string
	var clusterTaintPolicy *policyv1alpha1.ClusterTaintPolicy
	var targetClusterName string

	ginkgo.JustBeforeEach(func() {
		karmadaresource.CreateClusterTaintPolicy(karmadaClient, clusterTaintPolicy)
		ginkgo.DeferCleanup(func() {
			karmadaresource.RemoveClusterTaintPolicy(karmadaClient, policyName)
		})
	})

	ginkgo.BeforeEach(func() {
		policyName = clusterTaintPolicyNamePrefix + rand.String(RandomStrLength)
		clusterTaintPolicy = &policyv1alpha1.ClusterTaintPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policyv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ClusterTaintPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: policyName,
			},
		}
	})

	ginkgo.Context("One condition match", func() {
		ginkgo.BeforeEach(func() {
			targetClusterName = framework.ClusterNames()[1]
			clusterTaintPolicy.Spec = policyv1alpha1.ClusterTaintPolicySpec{
				TargetClusters: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetClusterName},
				},
				AddOnConditions: []policyv1alpha1.MatchCondition{
					{
						ConditionType: "NetworkReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown},
					},
				},
				RemoveOnConditions: []policyv1alpha1.MatchCondition{
					{
						ConditionType: "NetworkReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionTrue},
					},
				},
				Taints: []policyv1alpha1.Taint{
					{
						Key:    "testing/network-not-ready",
						Effect: "NoSchedule",
					},
				},
			}
		})

		ginkgo.It("taint should be added and removed upon single condition match", func() {
			ginkgo.By(fmt.Sprintf("update cluster(%s) NetworkReady condition to false", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "NetworkReady",
					Status: metav1.ConditionFalse,
				})
			})

			ginkgo.By(fmt.Sprintf("wait for the cluster(%s) taint added", targetClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/network-not-ready", Effect: "NoSchedule"})
				})
			})

			ginkgo.By(fmt.Sprintf("update cluster(%s) NetworkReady condition to true", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "NetworkReady",
					Status: metav1.ConditionTrue,
				})
			})

			ginkgo.By(fmt.Sprintf("wait for the cluster(%s) taint removed", targetClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return !helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/network-not-ready", Effect: "NoSchedule"})
				})
			})
		})

	})

	ginkgo.Context("Multi conditions match", func() {
		ginkgo.BeforeEach(func() {
			targetClusterName = framework.ClusterNames()[0]
			clusterTaintPolicy.Spec = policyv1alpha1.ClusterTaintPolicySpec{
				TargetClusters: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetClusterName},
				},
				AddOnConditions: []policyv1alpha1.MatchCondition{
					{
						ConditionType: "NetworkReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown},
					},
					{
						ConditionType: "StorageReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown},
					},
				},
				RemoveOnConditions: []policyv1alpha1.MatchCondition{
					{
						ConditionType: "NetworkReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionTrue},
					},
					{
						ConditionType: "StorageReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionTrue},
					},
				},
				Taints: []policyv1alpha1.Taint{
					{
						Key:    "testing/all-not-ready",
						Effect: "NoSchedule",
					},
				},
			}
		})

		ginkgo.It("taint should be added and removed upon multi conditions match", func() {
			ginkgo.By(fmt.Sprintf("update cluster(%s) NetworkReady condition to false", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "NetworkReady",
					Status: metav1.ConditionFalse,
				})
			})

			ginkgo.By(fmt.Sprintf("update cluster(%s) StorageReady condition to false", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "StorageReady",
					Status: metav1.ConditionFalse,
				})
			})

			ginkgo.By(fmt.Sprintf("wait for the cluster(%s) taint added", targetClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/all-not-ready", Effect: "NoSchedule"})
				})
			})

			ginkgo.By(fmt.Sprintf("update cluster(%s) NetworkReady condition to true", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "NetworkReady",
					Status: metav1.ConditionTrue,
				})
			})

			ginkgo.By(fmt.Sprintf("update cluster(%s) StorageReady condition to true", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "StorageReady",
					Status: metav1.ConditionTrue,
				})
			})

			ginkgo.By(fmt.Sprintf("wait for the cluster(%s) taint removed", targetClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return !helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/all-not-ready", Effect: "NoSchedule"})
				})
			})
		})

	})

	ginkgo.Context("Control with multiple taints", func() {
		ginkgo.BeforeEach(func() {
			targetClusterName = framework.ClusterNames()[1]
			clusterTaintPolicy.Spec = policyv1alpha1.ClusterTaintPolicySpec{
				TargetClusters: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetClusterName},
				},
				AddOnConditions: []policyv1alpha1.MatchCondition{
					{
						ConditionType: "NetworkReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown},
					},
				},
				RemoveOnConditions: []policyv1alpha1.MatchCondition{
					{
						ConditionType: "NetworkReady",
						Operator:      policyv1alpha1.MatchConditionOpIn,
						StatusValues:  []metav1.ConditionStatus{metav1.ConditionTrue},
					},
				},
				Taints: []policyv1alpha1.Taint{
					{
						Key:    "testing/network-not-ready",
						Effect: "NoSchedule",
					},
					{
						Key:    "testing/network-not-ready",
						Effect: "NoExecute",
					},
				},
			}
		})

		ginkgo.It("multiple taints should be added and removed upon conditions match", func() {
			ginkgo.By(fmt.Sprintf("update cluster(%s) NetworkReady condition to false", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "NetworkReady",
					Status: metav1.ConditionFalse,
				})
			})

			ginkgo.By(fmt.Sprintf("wait for the cluster(%s) taint added", targetClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/network-not-ready", Effect: "NoSchedule"}) &&
						helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/network-not-ready", Effect: "NoExecute"})
				})
			})

			ginkgo.By(fmt.Sprintf("update cluster(%s) NetworkReady condition to true", targetClusterName), func() {
				framework.UpdateClusterStatusCondition(karmadaClient, targetClusterName, metav1.Condition{
					Type:   "NetworkReady",
					Status: metav1.ConditionTrue,
				})
			})

			ginkgo.By(fmt.Sprintf("wait for the cluster(%s) taint removed", targetClusterName), func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetClusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return !helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/network-not-ready", Effect: "NoSchedule"}) &&
						!helper.TaintExists(cluster.Spec.Taints, &corev1.Taint{Key: "testing/network-not-ready", Effect: "NoExecute"})
				})
			})
		})
	})
})
