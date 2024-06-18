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

package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	karmadaresource "github.com/karmada-io/karmada/test/e2e/framework/resource/karmada"
)

var _ = framework.SerialDescribe("remedy testing", func() {
	var remedyName string
	var remedy *remedyv1alpha1.Remedy
	var targetCluster string

	ginkgo.Context("remedy.spec.decisionMatches is not empty", func() {
		ginkgo.BeforeEach(func() {
			targetCluster = framework.ClusterNames()[0]
			remedyName = remedyNamePrefix + rand.String(RandomStrLength)
			remedy = &remedyv1alpha1.Remedy{
				ObjectMeta: metav1.ObjectMeta{Name: remedyName},
				Spec: remedyv1alpha1.RemedySpec{
					DecisionMatches: []remedyv1alpha1.DecisionMatch{
						{
							ClusterConditionMatch: &remedyv1alpha1.ClusterConditionRequirement{
								ConditionType:   remedyv1alpha1.ServiceDomainNameResolutionReady,
								Operator:        remedyv1alpha1.ClusterConditionEqual,
								ConditionStatus: string(metav1.ConditionFalse),
							},
						},
					},
					Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
				},
			}

			karmadaresource.CreateRemedy(karmadaClient, remedy)
		})

		ginkgo.It("Cluster domain name resolution function encounters an exception and recover", func() {
			ginkgo.By(fmt.Sprintf("update Cluster(%s) %s condition to false", targetCluster, remedyv1alpha1.ServiceDomainNameResolutionReady), func() {
				clusterObj, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), targetCluster, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				meta.SetStatusCondition(&clusterObj.Status.Conditions, metav1.Condition{
					Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
					Status: metav1.ConditionFalse,
				})
				_, err = karmadaClient.ClusterV1alpha1().Clusters().UpdateStatus(context.TODO(), clusterObj, metav1.UpdateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("wait cluster status has TrafficControl RemedyAction", func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					actions := sets.NewString(cluster.Status.RemedyActions...)
					return actions.Has(string(remedyv1alpha1.TrafficControl))
				})
			})

			ginkgo.By(fmt.Sprintf("recover Cluster(%s) %s condition to true", targetCluster, remedyv1alpha1.ServiceDomainNameResolutionReady), func() {
				clusterObj, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), targetCluster, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				meta.SetStatusCondition(&clusterObj.Status.Conditions, metav1.Condition{
					Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
					Status: metav1.ConditionTrue,
				})
				_, err = karmadaClient.ClusterV1alpha1().Clusters().UpdateStatus(context.TODO(), clusterObj, metav1.UpdateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("wait cluster status doesn't has TrafficControl RemedyAction", func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					actions := sets.NewString(cluster.Status.RemedyActions...)
					return !actions.Has(string(remedyv1alpha1.TrafficControl))
				})
			})

			ginkgo.By("cleanup: remove Remedy resource", func() {
				karmadaresource.RemoveRemedy(karmadaClient, remedyName)
			})
		})

		ginkgo.It("Cluster domain name resolution function encounters an exception, then remove the remedy resource", func() {
			ginkgo.By(fmt.Sprintf("update Cluster(%s) %s condition to false", targetCluster, remedyv1alpha1.ServiceDomainNameResolutionReady), func() {
				clusterObj, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), targetCluster, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				meta.SetStatusCondition(&clusterObj.Status.Conditions, metav1.Condition{
					Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
					Status: metav1.ConditionFalse,
				})
				_, err = karmadaClient.ClusterV1alpha1().Clusters().UpdateStatus(context.TODO(), clusterObj, metav1.UpdateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("wait cluster status has TrafficControl RemedyAction", func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					actions := sets.NewString(cluster.Status.RemedyActions...)
					return actions.Has(string(remedyv1alpha1.TrafficControl))
				})
			})

			ginkgo.By("remove Remedy resource", func() {
				karmadaresource.RemoveRemedy(karmadaClient, remedyName)
			})

			ginkgo.By("wait cluster status doesn't has TrafficControl RemedyAction", func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					actions := sets.NewString(cluster.Status.RemedyActions...)
					return !actions.Has(string(remedyv1alpha1.TrafficControl))
				})
			})

			ginkgo.By(fmt.Sprintf("cleanup: recover Cluster(%s) %s to true", targetCluster, remedyv1alpha1.ServiceDomainNameResolutionReady), func() {
				clusterObj, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), targetCluster, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				meta.SetStatusCondition(&clusterObj.Status.Conditions, metav1.Condition{
					Type:   string(remedyv1alpha1.ServiceDomainNameResolutionReady),
					Status: metav1.ConditionTrue,
				})
				_, err = karmadaClient.ClusterV1alpha1().Clusters().UpdateStatus(context.TODO(), clusterObj, metav1.UpdateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("test with nil decision matches remedy", func() {
		ginkgo.BeforeEach(func() {
			targetCluster = framework.ClusterNames()[0]
			remedyName = remedyNamePrefix + rand.String(RandomStrLength)
			remedy = &remedyv1alpha1.Remedy{
				ObjectMeta: metav1.ObjectMeta{Name: remedyName},
				Spec: remedyv1alpha1.RemedySpec{
					Actions: []remedyv1alpha1.RemedyAction{remedyv1alpha1.TrafficControl},
				},
			}

		})

		ginkgo.It("Create an immediately type remedy, then remove it", func() {
			ginkgo.By("create Remedy resource", func() {
				karmadaresource.CreateRemedy(karmadaClient, remedy)
			})

			ginkgo.By("wait cluster status has TrafficControl RemedyAction", func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					actions := sets.NewString(cluster.Status.RemedyActions...)
					return actions.Has(string(remedyv1alpha1.TrafficControl))
				})
			})

			ginkgo.By("remove Remedy resource", func() {
				karmadaresource.RemoveRemedy(karmadaClient, remedyName)
			})

			ginkgo.By("wait cluster status doesn't has TrafficControl RemedyAction", func() {
				framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
					actions := sets.NewString(cluster.Status.RemedyActions...)
					return !actions.Has(string(remedyv1alpha1.TrafficControl))
				})
			})
		})
	})
})
