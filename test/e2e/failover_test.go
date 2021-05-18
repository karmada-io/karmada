package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/helper"
)

// failover testing is used to test the rescheduling situation when some initially scheduled clusters fail
var _ = ginkgo.Describe("failover testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		var disabledClusters []*clusterv1alpha1.Cluster
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		maxGroups := 1
		minGroups := 1
		numOfFailedClusters := 1

		// targetClusterNames is a slice of cluster names in resource binding
		var targetClusterNames []string

		// set MaxGroups=MinGroups=1, label is location=CHN.
		policy := helper.NewPolicyWithGroupsDeployment(policyNamespace, policyName, deployment, maxGroups, minGroups, clusterLabels)

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("deployment failover testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				fmt.Printf("MaxGroups= %v, MinGroups= %v\n", maxGroups, minGroups)
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				fmt.Printf("View the results of the initial scheduling")
				targetClusterNames, _ = getTargetClusterNames(deployment)
				for _, clusterName := range targetClusterNames {
					fmt.Printf("%s is the target cluster\n", clusterName)
				}
			})

			ginkgo.By("set one cluster condition status to false", func() {
				temp := numOfFailedClusters
				for _, targetClusterName := range targetClusterNames {
					if temp > 0 {
						err := disableCluster(controlPlaneClient, targetClusterName)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						fmt.Printf("cluster %s is false\n", targetClusterName)
						currentCluster, _ := util.GetCluster(controlPlaneClient, targetClusterName)

						// wait for the current cluster status changing to false
						_ = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
							if !meta.IsStatusConditionPresentAndEqual(currentCluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse) {
								fmt.Printf("current cluster %s is false\n", targetClusterName)
								disabledClusters = append(disabledClusters, currentCluster)
								return true, nil
							}
							return false, nil
						})
						temp--
					}
				}
			})

			ginkgo.By("check whether deployment of failed cluster is rescheduled to other available cluster", func() {
				totalNum := 0

				// Since labels are added to all clusters, clusters are used here instead of written as clusters which have label.
				targetClusterNames, err := getTargetClusterNames(deployment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				for _, targetClusterName := range targetClusterNames {
					clusterClient := getClusterClient(targetClusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Check whether deployment(%s/%s) is present on cluster(%s)", deploymentNamespace, deploymentName, targetClusterName)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						fmt.Printf("Deployment(%s/%s) is present on cluster(%s).\n", deploymentNamespace, deploymentName, targetClusterName)
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					totalNum++
					gomega.Expect(totalNum == minGroups).Should(gomega.BeTrue())
				}
				fmt.Printf("reschedule in %d target cluster\n", totalNum)
			})

			ginkgo.By("recover not ready cluster", func() {
				for _, disabledCluster := range disabledClusters {
					fmt.Printf("cluster %s is waiting for recovering\n", disabledCluster.Name)
					originalAPIEndpoint := getClusterAPIEndpoint(disabledCluster.Name)

					err := recoverCluster(controlPlaneClient, disabledCluster.Name, originalAPIEndpoint)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})

// disableCluster will set wrong API endpoint of current cluster
func disableCluster(c client.Client, clusterName string) error {
	err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			if errors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		// set the APIEndpoint of matched cluster to a wrong value
		unavailableAPIEndpoint := "https://172.19.1.3:6443"
		clusterObj.Spec.APIEndpoint = unavailableAPIEndpoint
		if err := c.Update(context.TODO(), clusterObj); err != nil {
			if errors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return err
}

// recoverCluster will recover API endpoint of the disable cluster
func recoverCluster(c client.Client, clusterName string, originalAPIEndpoint string) error {
	err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			return false, err
		}
		clusterObj.Spec.APIEndpoint = originalAPIEndpoint
		if err := c.Update(context.TODO(), clusterObj); err != nil {
			if errors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		fmt.Printf("recovered API endpoint is %s\n", clusterObj.Spec.APIEndpoint)
		return true, nil
	})
	return err
}

// get the target cluster names from binding information
func getTargetClusterNames(deployment *appsv1.Deployment) (targetClusterNames []string, err error) {
	bindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
	fmt.Printf("deploy kind is %s, name is %s\n", deployment.Kind, deployment.Name)
	binding := &workv1alpha1.ResourceBinding{}

	fmt.Printf("collect the target clusters in resource binding\n")
	err = wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
		err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: deployment.Namespace, Name: bindingName}, binding)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	for _, cluster := range binding.Spec.Clusters {
		targetClusterNames = append(targetClusterNames, cluster.Name)
	}
	fmt.Printf("target clusters in resource binding are %s\n", targetClusterNames)
	return targetClusterNames, nil
}

// get the API endpoint of a specific cluster
func getClusterAPIEndpoint(clusterName string) (apiEndpoint string) {
	for _, cluster := range clusters {
		if cluster.Name == clusterName {
			apiEndpoint = cluster.Spec.APIEndpoint
			fmt.Printf("original API endpoint of the cluster %s is %s\n", clusterName, apiEndpoint)
		}
	}
	return apiEndpoint
}
