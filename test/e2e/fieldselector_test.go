package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("propagation with fieldSelector testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var originalClusterProviderInfo, originalClusterRegionInfo map[string]string
		var desiredProvider, undesiredRegion []string
		var desiredScheduleResult string
		var filedSelector *policyv1alpha1.FieldSelector
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName
			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)

			originalClusterProviderInfo = make(map[string]string)
			originalClusterRegionInfo = make(map[string]string)
			desiredProvider = []string{"huaweicloud"}
			undesiredRegion = []string{"cn-north-1"}
			desiredScheduleResult = "member1"

			// desire to schedule to clusters of huaweicloud but not in cn-north-1 region
			filedSelector = &policyv1alpha1.FieldSelector{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      util.ProviderField,
						Operator: corev1.NodeSelectorOpIn,
						Values:   desiredProvider,
					},
					{
						Key:      util.RegionField,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   undesiredRegion,
					},
				},
			}

			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames:  framework.ClusterNames(),
					FieldSelector: filedSelector,
				},
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("setting provider and region for clusters", func() {
				providerMap := []string{"huaweicloud", "huaweicloud", "kind"}
				regionMap := []string{"cn-south-1", "cn-north-1", "cn-east-1"}
				for index, cluster := range framework.ClusterNames() {
					if index > 2 {
						break
					}
					fmt.Printf("setting provider and region for cluster %v\n", cluster)
					gomega.Eventually(func(g gomega.Gomega) {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: cluster}, clusterObj)
						g.Expect(err).NotTo(gomega.HaveOccurred())

						originalClusterProviderInfo[cluster] = clusterObj.Spec.Provider
						originalClusterRegionInfo[cluster] = clusterObj.Spec.Region
						clusterObj.Spec.Provider = providerMap[index]
						clusterObj.Spec.Region = regionMap[index]
						err = controlPlaneClient.Update(context.TODO(), clusterObj)
						g.Expect(err).NotTo(gomega.HaveOccurred())
					}, pollTimeout, pollInterval).Should(gomega.Succeed())
				}
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By("recovering provider and region for clusters", func() {
				for index, cluster := range framework.ClusterNames() {
					if index > 2 {
						break
					}

					gomega.Eventually(func(g gomega.Gomega) {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: cluster}, clusterObj)
						g.Expect(err).NotTo(gomega.HaveOccurred())

						clusterObj.Spec.Provider = originalClusterProviderInfo[cluster]
						clusterObj.Spec.Region = originalClusterRegionInfo[cluster]
						err = controlPlaneClient.Update(context.TODO(), clusterObj)
						g.Expect(err).NotTo(gomega.HaveOccurred())
					}, pollTimeout, pollInterval).Should(gomega.Succeed())
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

		ginkgo.It("propagation with fieldSelector testing", func() {
			ginkgo.By("check whether deployment is scheduled to clusters which meeting the fieldSelector requirements", func() {
				targetClusterNames := framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)
				gomega.Expect(len(targetClusterNames)).Should(gomega.Equal(1))
				gomega.Expect(targetClusterNames[0]).Should(gomega.Equal(desiredScheduleResult))
			})
		})
	})
})
