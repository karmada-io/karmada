package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
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

		ginkgo.It("deployment propagation with label and group constraints testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By("collect the target clusters in resource binding", func() {
				var err error
				targetClusterNames, err = getTargetClusterNames(deployment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(len(targetClusterNames) == minGroups).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By("check if the scheduled condition is true", func() {
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
					rb, err := getResourceBinding(deployment)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(rb.Status.Conditions, workv1alpha2.Scheduled), nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if deployment present on right clusters", func() {
				for _, targetClusterName := range targetClusterNames {
					clusterClient := framework.GetClusterClient(targetClusterName)
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
					clusterClient := framework.GetClusterClient(cluster.Name)
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

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
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

		ginkgo.It("crd with specified label and group constraints propagation testing", func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			framework.GetCRD(dynamicClient, crd.Name)

			ginkgo.By("collect the target clusters in cluster resource binding", func() {
				bindingName := names.GenerateBindingName(crd.Kind, crd.Name)
				fmt.Printf("crd kind is %s, name is %s\n", crd.Kind, crd.Name)
				binding := &workv1alpha2.ClusterResourceBinding{}

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

			ginkgo.By("check if the scheduled condition is true", func() {
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
					crb, err := getClusterResourceBinding(crd)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(crb.Status.Conditions, workv1alpha2.Scheduled), nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if crd present on right clusters", func() {
				for _, targetClusterName := range targetClusterNames {
					clusterDynamicClient := framework.GetClusterDynamicClient(targetClusterName)
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

			framework.RemoveCRD(dynamicClient, crd.Name)
			framework.WaitCRDDisappearedOnClusters(framework.GetClusterNamesFromClusters(groupMatchedClusters), crd.Name)
			framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
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
				ClusterNames: framework.ClusterNames(),
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		})

		ginkgo.It("replicas duplicated testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By("check if deployment's replicas are duplicate on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (*int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, *deployment.Spec.Replicas))
						return memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(deployment.Spec.Replicas))
				}
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
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
				ClusterNames: framework.ClusterNames(),
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		})

		ginkgo.It("replicas duplicated testing when rescheduling", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By("make sure deployment has been propagated to member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						_, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())
						return true, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			ginkgo.By("check if deployment's replicas have been updated on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (*int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, *deployment.Spec.Replicas))
						return memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(deployment.Spec.Replicas))
				}
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
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
				ClusterNames: framework.ClusterNames(),
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
		})

		ginkgo.It("replicas divided and weighted testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			expectedReplicas := int32(2)
			updateReplicas := expectedReplicas * int32(len(framework.Clusters()))
			deployment.Spec.Replicas = &updateReplicas
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
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
				ClusterNames: framework.ClusterNames(),
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
		})

		ginkgo.It("replicas divided and weighted testing when rescheduling", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			framework.WaitDeploymentPresentOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)

			expectedReplicas := int32(3)
			updateReplicas := expectedReplicas * int32(len(framework.Clusters()))
			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateReplicas)

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})

			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
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
				ClusterNames: framework.ClusterNames(),
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
			for index, clusterName := range framework.ClusterNames() {
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

			framework.CreatePropagationPolicy(karmadaClient, policy)
			sumReplicas := int32(sumWeight)
			deployment.Spec.Replicas = &sumReplicas
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for index, cluster := range framework.Clusters() {
					expectedReplicas := int32(index + 1)

					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
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
				ClusterNames: framework.ClusterNames(),
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference:          &policyv1alpha1.ClusterPreferences{},
			},
		})

		ginkgo.BeforeEach(func() {
			staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
			for index, clusterName := range framework.ClusterNames() {
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
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
		})

		ginkgo.AfterEach(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})

		ginkgo.It("replicas divided and weighted testing when rescheduling", func() {
			framework.WaitDeploymentPresentOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)

			sumWeight := 0
			for index := range framework.ClusterNames() {
				sumWeight += index + 1
			}
			klog.Infof("sumWeight of clusters is %d", sumWeight)
			updateReplicas := 2 * int32(sumWeight)
			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateReplicas)

			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for index, cluster := range framework.Clusters() {
					expectedReplicas := 2 * int32(index+1)

					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Info(fmt.Sprintf("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas))
						return *memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})
		})
	})
})

// get the resource binding associated with the workload
func getResourceBinding(workload interface{}) (*workv1alpha2.ResourceBinding, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	obj := unstructured.Unstructured{Object: uncastObj}
	bindingName := names.GenerateBindingName(obj.GetKind(), obj.GetName())
	binding := &workv1alpha2.ResourceBinding{}

	err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: obj.GetNamespace(), Name: bindingName}, binding)
	return binding, err
}

// get the cluster resource binding associated with the workload
func getClusterResourceBinding(workload interface{}) (*workv1alpha2.ClusterResourceBinding, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	obj := unstructured.Unstructured{Object: uncastObj}
	bindingName := names.GenerateBindingName(obj.GetKind(), obj.GetName())
	binding := &workv1alpha2.ClusterResourceBinding{}

	err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: bindingName}, binding)
	return binding, err
}
