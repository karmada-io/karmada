package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("propagation with fieldSelector testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)

		originalClusterProviderInfo := make(map[string]string)
		originalClusterRegionInfo := make(map[string]string)
		desiredProvider := []string{"huaweicloud"}
		undesiredRegion := []string{"cn-north-1"}
		desiredScheduleResult := "member1"

		// desire to schedule to clusters of huaweicloud but not in cn-north-1 region
		filedSelector := &policyv1alpha1.FieldSelector{
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

		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames:  clusterNames,
				FieldSelector: filedSelector,
			},
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("setting provider and region for clusters", func() {
				providerMap := []string{"huaweicloud", "huaweicloud", "kind"}
				regionMap := []string{"cn-south-1", "cn-north-1", "cn-east-1"}
				for index, cluster := range clusterNames {
					if index > 2 {
						break
					}
					fmt.Printf("setting provider and region for cluster %v", cluster)
					gomega.Eventually(func() error {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: cluster}, clusterObj)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						originalClusterProviderInfo[cluster] = clusterObj.Spec.Provider
						originalClusterRegionInfo[cluster] = clusterObj.Spec.Region
						clusterObj.Spec.Provider = providerMap[index]
						clusterObj.Spec.Region = regionMap[index]
						return controlPlaneClient.Update(context.TODO(), clusterObj)
					}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
				}
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By("recovering provider and region for clusters", func() {
				for index, cluster := range clusterNames {
					if index > 2 {
						break
					}

					gomega.Eventually(func() error {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: cluster}, clusterObj)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						clusterObj.Spec.Provider = originalClusterProviderInfo[cluster]
						clusterObj.Spec.Region = originalClusterRegionInfo[cluster]
						return controlPlaneClient.Update(context.TODO(), clusterObj)
					}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
				}
			})
		})

		ginkgo.It("propagation with fieldSelector testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check whether deployment is scheduled to clusters which meeting the fieldSelector requirements", func() {
				targetClusterNames, err := getTargetClusterNames(deployment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(len(targetClusterNames) == 1).Should(gomega.BeTrue())
				gomega.Expect(targetClusterNames[0] == desiredScheduleResult).Should(gomega.BeTrue())
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
