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

package base

import (
	"context"
	"strconv"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = framework.SerialDescribe("spread-by-region testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var regionGroups, clusterGroups, updatedRegionGroups, numOfRegionClusters int
		var policy *policyv1alpha1.PropagationPolicy
		var regionClusters []string

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName
			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			clusterGroups = 1
			regionGroups = 2
			updatedRegionGroups = 1
			numOfRegionClusters = 2

			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
				},
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     clusterGroups,
						MinGroups:     clusterGroups,
					},
					{
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     regionGroups,
						MinGroups:     regionGroups,
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			clusters := framework.ClusterNames()
			temp := numOfRegionClusters
			for _, clusterName := range clusters {
				if temp > 0 {
					err := framework.SetClusterRegion(controlPlaneClient, clusterName, strconv.Itoa(temp))
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					regionClusters = append(regionClusters, clusterName)
					temp--
				}
			}
			ginkgo.DeferCleanup(func() {
				for _, clusterName := range regionClusters {
					err := framework.SetClusterRegion(controlPlaneClient, clusterName, "")
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("multiple region deployment testing", func() {
			ginkgo.By("check whether deployment is scheduled to multiple regions", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(regionClusters, deployment.Namespace, deployment.Name,
					func(*appsv1.Deployment) bool {
						return true
					})
			})

			ginkgo.By("update propagation policy to propagate to one region", func() {
				updateSpreadConstraints := []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     clusterGroups,
						MinGroups:     clusterGroups,
					},
					{
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     updatedRegionGroups,
						MinGroups:     updatedRegionGroups,
					},
				}
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/placement/spreadConstraints",
						"value": updateSpreadConstraints,
					},
				}
				framework.PatchPropagationPolicy(karmadaClient, policyNamespace, policyName, patch, types.JSONPatchType)
				bindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
				binding := &workv1alpha2.ResourceBinding{}
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: deployment.Namespace, Name: bindingName}, binding)
					g.Expect(err).NotTo(gomega.HaveOccurred())

					targetClusterNames := make([]string, 0, len(binding.Spec.Clusters))
					for _, cluster := range binding.Spec.Clusters {
						targetClusterNames = append(targetClusterNames, cluster.Name)
					}

					return len(targetClusterNames) == updatedRegionGroups, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})
