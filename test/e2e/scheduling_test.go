package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/helper"
)

// BasicPropagation focus on basic propagation functionality testing.
var _ = ginkgo.Describe("propagation with label and group constraints testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		var groupMatchedClusters []*clusterv1alpha1.Cluster
		var targetClusterNames []string
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		maxGroups := rand.Intn(2) + 1
		minGroups := maxGroups

		// set MaxGroups=MinGroups=1 or 2, label is location=CHN.
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

		ginkgo.It("deployment propagation with label and group constraints testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("collect the target clusters in resource binding", func() {
				bindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
				fmt.Printf("deploy kind is %s, name is %s\n", deployment.Kind, deployment.Name)
				binding := &workv1alpha1.ResourceBinding{}

				err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
					err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: deployment.Namespace, Name: bindingName}, binding)
					if err != nil {
						if errors.IsNotFound(err) {
							return false, nil
						}
						return false, err
					}
					return true, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				fmt.Printf("MaxGroups= %v, MinGroups= %v\n", maxGroups, minGroups)
				for _, cluster := range binding.Spec.Clusters {
					targetClusterNames = append(targetClusterNames, cluster.Name)
				}
				fmt.Printf("target clusters in resource binding are %s\n", targetClusterNames)
				gomega.Expect(len(targetClusterNames) == minGroups).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By("check if deployment present on right clusters", func() {
				for _, cluster := range clusters {
					for _, targetClusterName := range targetClusterNames {
						if cluster.Name == targetClusterName {
							clusterClient := getClusterClient(cluster.Name)
							gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

							klog.Infof("Check whether deployment(%s/%s) is present on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
							err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
								_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
								if err != nil {
									if errors.IsNotFound(err) {
										return false, nil
									}
									return false, err
								}
								groupMatchedClusters = append(groupMatchedClusters, cluster)
								fmt.Printf("Deployment(%s/%s) is present on cluster(%s).\n", deploymentNamespace, deploymentName, cluster.Name)
								return true, nil
							})
							gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						}
					}
				}
				fmt.Printf("there are %d target clusters\n", len(groupMatchedClusters))
				gomega.Expect(minGroups == len(groupMatchedClusters)).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By("updating deployment", func() {
				patch := map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": pointer.Int32Ptr(updateDeploymentReplicas),
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = kubeClient.AppsV1().Deployments(deploymentNamespace).Patch(context.TODO(), deploymentName, types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range groupMatchedClusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for deployment(%s/%s) synced on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						dep, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						if *dep.Spec.Replicas == updateDeploymentReplicas {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment has been deleted from member clusters", func() {
				for _, cluster := range groupMatchedClusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for deployment(%s/%s) disappear on cluster(%s)", deploymentNamespace, deploymentName, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})
	ginkgo.Context("CustomResourceDefinition propagation testing", func() {
		var groupMatchedClusters []*clusterv1alpha1.Cluster
		var targetClusterNames []string
		crdGroup := fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
		randStr := rand.String(RandomStrLength)
		crdSpecNames := apiextensionsv1.CustomResourceDefinitionNames{
			Kind:     fmt.Sprintf("Foo%s", randStr),
			ListKind: fmt.Sprintf("Foo%sList", randStr),
			Plural:   fmt.Sprintf("foo%ss", randStr),
			Singular: fmt.Sprintf("foo%s", randStr),
		}
		crd := helper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
		maxGroups := rand.Intn(2) + 1
		minGroups := maxGroups

		// set MaxGroups=MinGroups=1 or 2, label is location=CHN.
		crdPolicy := helper.NewConstraintsPolicyWithSingleCRD(crd.Name, crd, maxGroups, minGroups, clusterLabels)
		crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating crdPolicy(%s)", crdPolicy.Name), func() {
				_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), crdPolicy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing crdPolicy(%s)", crdPolicy.Name), func() {
				err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Delete(context.TODO(), crdPolicy.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("crd with specified label and group constraints propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating crd(%s)", crd.Name), func() {
				unstructObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: unstructObj}, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("get crd(%s)", crd.Name), func() {
				_, err := dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("collect the target clusters in cluster resource binding", func() {
				bindingName := names.GenerateBindingName(crd.Kind, crd.Name)
				fmt.Printf("crd kind is %s, name is %s\n", crd.Kind, crd.Name)
				binding := &workv1alpha1.ClusterResourceBinding{}

				fmt.Printf("MaxGroups= %v, MinGroups= %v\n", maxGroups, minGroups)
				err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
					err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: bindingName}, binding)
					if err != nil {
						if errors.IsNotFound(err) {
							return false, nil
						}
						return false, err
					}
					return true, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				for _, cluster := range binding.Spec.Clusters {
					targetClusterNames = append(targetClusterNames, cluster.Name)
				}
				fmt.Printf("target clusters in cluster resource binding are %s\n", targetClusterNames)
				gomega.Expect(len(targetClusterNames) == minGroups).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By("check if crd present on right clusters", func() {
				for _, cluster := range clusters {
					for _, targetClusterName := range targetClusterNames {
						if cluster.Name == targetClusterName {
							clusterDynamicClient := getClusterDynamicClient(cluster.Name)
							gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

							klog.Infof("Waiting for crd(%s) present on cluster(%s)", crd.Name, cluster.Name)
							err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
								_, err = clusterDynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
								if err != nil {
									if errors.IsNotFound(err) {
										return false, nil
									}
									return false, err
								}
								groupMatchedClusters = append(groupMatchedClusters, cluster)
								fmt.Printf("Crd(%s) is present on cluster(%s).\n", crd.Name, cluster.Name)
								return true, nil
							})
							gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						}
					}
				}
				fmt.Printf("there are %d target clusters\n", len(groupMatchedClusters))
				gomega.Expect(minGroups == len(groupMatchedClusters)).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By(fmt.Sprintf("removing crd(%s)", crd.Name), func() {
				err := dynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Delete(context.TODO(), crd.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if crd with specified label and group constraints disappeared from member clusters", func() {
				for _, cluster := range groupMatchedClusters {
					clusterDynamicClient := getClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for crd(%s) disappeared on cluster(%s)\n", crd.Name, cluster.Name)
					err := wait.Poll(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
						if err != nil {
							if errors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}
						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})
})
