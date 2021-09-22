package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
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
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: clusterLabels,
				},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MaxGroups:     maxGroups,
					MinGroups:     minGroups,
				},
			},
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				klog.Infof("created policy(%s/%s)", policyNamespace, policyName)
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("deployment propagation with label and group constraints testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deployment.Namespace, deployment.Name), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				klog.Infof("created deployment(%s/%s)", deployment.Namespace, deployment.Name)
			})

			ginkgo.By("collect the target clusters in resource binding", func() {
				var err error
				targetClusterNames, err = getTargetClusterNames(deployment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(len(targetClusterNames) == minGroups).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By("check if deployment present on right clusters", func() {
				for _, targetClusterName := range targetClusterNames {
					clusterClient := getClusterClient(targetClusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					klog.Infof("Check whether deployment(%s/%s) is present on cluster(%s)", deploymentNamespace, deploymentName, targetClusterName)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						targetCluster, _ := util.GetCluster(controlPlaneClient, targetClusterName)
						groupMatchedClusters = append(groupMatchedClusters, targetCluster)
						fmt.Printf("Deployment(%s/%s) is present on cluster(%s).\n", deploymentNamespace, deploymentName, targetClusterName)
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
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
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
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
		crdPolicy := helper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: crd.APIVersion,
				Kind:       crd.Kind,
				Name:       crd.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: clusterLabels,
				},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MaxGroups:     maxGroups,
					MinGroups:     minGroups,
				},
			},
		})
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
				gomega.Eventually(func() int {
					err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: bindingName}, binding)
					if err != nil {
						return -1
					}

					return len(binding.Spec.Clusters)
				}, pollTimeout, pollInterval).Should(gomega.Equal(minGroups))

				for _, cluster := range binding.Spec.Clusters {
					targetClusterNames = append(targetClusterNames, cluster.Name)
				}
				fmt.Printf("target clusters in cluster resource binding are %s\n", targetClusterNames)
			})

			ginkgo.By("check if crd present on right clusters", func() {
				for _, targetClusterName := range targetClusterNames {
					clusterDynamicClient := getClusterDynamicClient(targetClusterName)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for crd(%s) present on cluster(%s)", crd.Name, targetClusterName)
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						targetCluster, _ := util.GetCluster(controlPlaneClient, targetClusterName)
						groupMatchedClusters = append(groupMatchedClusters, targetCluster)
						fmt.Printf("Crd(%s) is present on cluster(%s).\n", crd.Name, targetClusterName)
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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
					err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crdGVR).Namespace(crd.Namespace).Get(context.TODO(), crd.Name, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
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

/*
	ReplicaScheduling focus on dealing with the number of replicas testing when propagating resources that have replicas
	in spec (e.g. deployments, statefulsets) to member clusters with ReplicaSchedulingStrategy.
	Test Case Overview:
		Case 1:
			`ReplicaSchedulingType` value is `Duplicated`.
		Case 2:
			`ReplicaSchedulingType` value is `Duplicated`, trigger rescheduling when replicas have changed.
		Case 3:
			`ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`, `WeightPreference` is nil.
		Case 4:
			`ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`, `WeightPreference` is nil, trigger rescheduling when replicas have changed.
		Case 5:
			`ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`, `WeightPreference` isn't nil.
		Case 6:
			`ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`, `WeightPreference` isn't nil, trigger rescheduling when replicas have changed.
*/
var _ = ginkgo.Describe("[ReplicaScheduling] ReplicaSchedulingStrategy testing", func() {
	// Case 1: `ReplicaSchedulingType` value is `Duplicated`.
	ginkgo.Context("ReplicaSchedulingType is Duplicated.", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := policyNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		})

		ginkgo.It("replicas duplicated testing", func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment's replicas are duplicate on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() *int32 {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, *deployment.Spec.Replicas))
						return memberDeployment.Spec.Replicas
					}, pollTimeout, pollInterval).Should(gomega.Equal(deployment.Spec.Replicas))
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	// Case 2: `ReplicaSchedulingType` value is `Duplicated`, trigger rescheduling when replicas have changed.
	ginkgo.Context("ReplicaSchedulingType is Duplicated, trigger rescheduling when replicas have changed.", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := policyNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("replicas duplicated testing when rescheduling", func() {
			ginkgo.By("make sure deployment has been propagated to member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() bool {
						_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						return true
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})

			ginkgo.By(fmt.Sprintf("Update deployment(%s/%s)'s replicas", deploymentNamespace, deploymentName), func() {
				updateReplicas := int32(2)
				deployment.Spec.Replicas = &updateReplicas

				gomega.Eventually(func() error {
					_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
					return err
				}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
				klog.Infof("Update deployment(%s/%s)'s replicas to %d", deploymentNamespace, deploymentName, *deployment.Spec.Replicas)
			})

			ginkgo.By("check if deployment's replicas have been updated on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() *int32 {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, *deployment.Spec.Replicas))
						return memberDeployment.Spec.Replicas
					}, pollTimeout, pollInterval).Should(gomega.Equal(deployment.Spec.Replicas))
				}
			})
		})
	})

	// Case 3: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` is nil.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference is nil.", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := policyNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
		})

		ginkgo.It("replicas divided and weighted testing", func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			expectedReplicas := int32(2)
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				updateReplicas := expectedReplicas * int32(len(clusters))
				deployment.Spec.Replicas = &updateReplicas
				_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() int32 {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	// Case 4: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` is nil, trigger rescheduling when replicas have changed.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference is "+
		"nil, trigger rescheduling when replicas have changed.", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := policyNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("replicas divided and weighted testing when rescheduling", func() {
			ginkgo.By("make sure deployment has been propagated to member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() bool {
						_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						return true
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})

			expectedReplicas := int32(3)
			ginkgo.By(fmt.Sprintf("Update deployment(%s/%s)'s replicas to 3*len(clusters)", policyNamespace, policyName), func() {
				updateReplicas := expectedReplicas * int32(len(clusters))
				deployment.Spec.Replicas = &updateReplicas

				gomega.Eventually(func() error {
					_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
					return err
				}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() int32 {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})
		})
	})

	// Case 5: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` isn't nil.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference isn't nil.", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := policyNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference:          &policyv1alpha1.ClusterPreferences{},
			},
		})

		ginkgo.It("replicas divided and weighted testing", func() {
			sumWeight := 0
			staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
			for index, clusterName := range clusterNames {
				staticWeightList := policyv1alpha1.StaticClusterWeight{
					TargetCluster: policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{clusterName},
					},
					Weight: int64(index + 1),
				}
				sumWeight += index + 1
				staticWeightLists = append(staticWeightLists, staticWeightList)
			}
			klog.Infof("StaticWeightList of policy is %+v", staticWeightLists)
			policy.Spec.Placement.ReplicaScheduling.WeightPreference.StaticWeightList = staticWeightLists
			klog.Infof("Sum weight of clusters is %d", sumWeight)

			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				sumReplicas := int32(sumWeight)
				deployment.Spec.Replicas = &sumReplicas

				_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for index, cluster := range clusters {
					expectedReplicas := int32(index + 1)

					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() int32 {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

	})

	// Case 6: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` isn't nil, trigger rescheduling when replicas have changed.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference isn't "+
		"nil, trigger rescheduling when replicas have changed.", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := policyNamespace
		deploymentName := policyName

		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		policy := helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference:          &policyv1alpha1.ClusterPreferences{},
			},
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
				for index, clusterName := range clusterNames {
					staticWeightList := policyv1alpha1.StaticClusterWeight{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{clusterName},
						},
						Weight: int64(index + 1),
					}
					staticWeightLists = append(staticWeightLists, staticWeightList)
				}
				klog.Infof("StaticWeightList of policy is %+v", staticWeightLists)
				policy.Spec.Placement.ReplicaScheduling.WeightPreference.StaticWeightList = staticWeightLists

				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing policy(%s/%s)", policyNamespace, policyName), func() {
				err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.It("replicas divided and weighted testing when rescheduling", func() {
			ginkgo.By("make sure deployment has been propagated to member clusters", func() {
				for _, cluster := range clusters {
					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() bool {
						_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						return true
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})

			ginkgo.By(fmt.Sprintf("Update deployment(%s/%s)'s replicas to 2*sumWeight", policyNamespace, policyName), func() {
				sumWeight := 0
				for index := range clusterNames {
					sumWeight += index + 1
				}
				klog.Infof("sumWeight of clusters is %d", sumWeight)
				updateReplicas := 2 * int32(sumWeight)
				deployment.Spec.Replicas = &updateReplicas

				gomega.Eventually(func() error {
					_, err := kubeClient.AppsV1().Deployments(deploymentNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
					return err
				}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for index, cluster := range clusters {
					expectedReplicas := 2 * int32(index+1)

					clusterClient := getClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func() int32 {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})
		})
	})
})
