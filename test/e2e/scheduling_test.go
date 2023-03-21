package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

// BasicPropagation focus on basic propagation functionality testing.
var _ = ginkgo.Describe("propagation with label and group constraints testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		var groupMatchedClusters []string
		var targetClusterNames []string
		var policyNamespace, policyName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var maxGroups, minGroups int
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName
			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			maxGroups = rand.Intn(2) + 1
			minGroups = maxGroups

			// set MaxGroups=MinGroups=1 or 2, label is location=CHN.
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("deployment propagation with label and group constraints testing", func() {
			ginkgo.By("collect the target clusters in resource binding", func() {
				targetClusterNames = framework.ExtractTargetClustersFrom(controlPlaneClient, deployment)
				gomega.Expect(len(targetClusterNames) == minGroups).ShouldNot(gomega.BeFalse())
			})

			ginkgo.By("check if the scheduled condition is true", func() {
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					rb, err := getResourceBinding(deployment)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(rb.Status.Conditions, workv1alpha2.Scheduled), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("check if deployment present on right clusters", func() {
				for _, targetClusterName := range targetClusterNames {
					framework.WaitDeploymentPresentOnClusterFitWith(targetClusterName, deployment.Namespace, deployment.Name,
						func(deployment *appsv1.Deployment) bool {
							return true
						})
					groupMatchedClusters = append(groupMatchedClusters, targetClusterName)
				}
				fmt.Printf("there are %d target clusters\n", len(groupMatchedClusters))
				gomega.Expect(minGroups == len(groupMatchedClusters)).ShouldNot(gomega.BeFalse())
			})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			framework.WaitDeploymentPresentOnClustersFitWith(groupMatchedClusters, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})
	ginkgo.Context("CustomResourceDefinition propagation testing", func() {
		var groupMatchedClusters []*clusterv1alpha1.Cluster
		var targetClusterNames []string
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition
		var maxGroups, minGroups int
		var crdPolicy *policyv1alpha1.ClusterPropagationPolicy
		var crdGVR schema.GroupVersionResource

		ginkgo.BeforeEach(func() {
			crdGroup = fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
			randStr = rand.String(RandomStrLength)
			crdSpecNames = apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     fmt.Sprintf("Foo%s", randStr),
				ListKind: fmt.Sprintf("Foo%sList", randStr),
				Plural:   fmt.Sprintf("foo%ss", randStr),
				Singular: fmt.Sprintf("foo%s", randStr),
			}
			crd = helper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
			maxGroups = rand.Intn(2) + 1
			minGroups = maxGroups

			// set MaxGroups=MinGroups=1 or 2, label is location=CHN.
			crdPolicy = helper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
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
			crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			framework.GetCRD(dynamicClient, crd.Name)
			ginkgo.DeferCleanup(func() {
				framework.RemoveCRD(dynamicClient, crd.Name)
				framework.WaitCRDDisappearedOnClusters(framework.GetClusterNamesFromClusters(groupMatchedClusters), crd.Name)
				framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
			})
		})

		ginkgo.It("crd with specified label and group constraints propagation testing", func() {
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
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					crb, err := getClusterResourceBinding(crd)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(crb.Status.Conditions, workv1alpha2.Scheduled), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
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
		})
	})
	ginkgo.Context("Job propagation testing", func() {
		var groupMatchedClusters []string
		var targetClusterNames []string
		var policyNamespace, policyName string
		var jobNamespace, jobName string
		var job *batchv1.Job
		var maxGroups, minGroups int
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = jobNamePrefix + rand.String(RandomStrLength)
			jobNamespace = testNamespace
			jobName = policyName
			job = helper.NewJob(jobNamespace, jobName)
			maxGroups = rand.Intn(2) + 1
			minGroups = maxGroups

			// set MaxGroups=MinGroups=1 or 2, label is location=CHN.
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: job.APIVersion,
					Kind:       job.Kind,
					Name:       job.Name,
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateJob(kubeClient, job)
			framework.GetJob(kubeClient, job.Namespace, job.Name)
			ginkgo.DeferCleanup(func() {
				framework.RemoveJob(kubeClient, job.Namespace, job.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("Job propagation with label and group constraints testing", func() {
			ginkgo.By("collect the target clusters in resource binding", func() {
				bindingName := names.GenerateBindingName(job.Kind, job.Name)
				binding := &workv1alpha2.ResourceBinding{}

				fmt.Printf("MaxGroups= %v, MinGroups= %v\n", maxGroups, minGroups)
				gomega.Eventually(func() int {
					err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: policyNamespace, Name: bindingName}, binding)
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
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					rb, err := getResourceBinding(job)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(rb.Status.Conditions, workv1alpha2.Scheduled), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("check if job present on right clusters", func() {
				for _, targetClusterName := range targetClusterNames {
					framework.WaitJobPresentOnClusterFitWith(targetClusterName, job.Namespace, job.Name,
						func(job *batchv1.Job) bool {
							return true
						})
					groupMatchedClusters = append(groupMatchedClusters, targetClusterName)
				}
				fmt.Printf("there are %d target clusters\n", len(groupMatchedClusters))
				gomega.Expect(minGroups == len(groupMatchedClusters)).ShouldNot(gomega.BeFalse())
			})

			patch := map[string]interface{}{"spec": map[string]interface{}{"parallelism": pointer.Int32(updateParallelism)}}
			bytes, err := json.Marshal(patch)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.UpdateJobWithPatchBytes(kubeClient, job.Namespace, job.Name, bytes, types.StrategicMergePatchType)
			framework.WaitJobPresentOnClustersFitWith(groupMatchedClusters, job.Namespace, job.Name,
				func(job *batchv1.Job) bool {
					return *job.Spec.Parallelism == updateParallelism
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
	var policyNamespace, policyName string
	var deploymentNamespace, deploymentName string
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName = deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace = policyNamespace
		deploymentName = policyName

		deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
		policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	// Case 1: `ReplicaSchedulingType` value is `Duplicated`.
	ginkgo.Context("ReplicaSchedulingType is Duplicated.", func() {
		ginkgo.It("replicas duplicated testing", func() {
			klog.Infof("check if deployment's replicas are duplicate on member clusters")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deploy *appsv1.Deployment) bool {
					klog.Infof("Deployment(%s/%s)'s replcas is %d, expected: %d.",
						deploy.Namespace, deploy.Name, *deploy.Spec.Replicas, *deployment.Spec.Replicas)
					return *deploy.Spec.Replicas == *deployment.Spec.Replicas
				})
		})
	})

	// Case 2: `ReplicaSchedulingType` value is `Duplicated`, trigger rescheduling when replicas have changed.
	ginkgo.Context("ReplicaSchedulingType is Duplicated, trigger rescheduling when replicas have changed.", func() {
		ginkgo.It("replicas duplicated testing when rescheduling", func() {
			klog.Infof("make sure deployment has been propagated to member clusters")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			klog.Infof("check if deployment's replicas have been updated on member clusters")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deploy *appsv1.Deployment) bool {
					klog.Infof("Deployment(%s/%s)'s replcas is %d, expected: %d.",
						deploy.Namespace, deploy.Name, *deploy.Spec.Replicas, updateDeploymentReplicas)
					return *deploy.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})

	// Case 3: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` is nil.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference is nil.", func() {
		var expectedReplicas int32
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			}

			expectedReplicas = int32(2)
			updateReplicas := expectedReplicas * int32(len(framework.Clusters()))
			deployment.Spec.Replicas = &updateReplicas
		})

		ginkgo.It("replicas divided and weighted testing", func() {
			klog.Infof("check if deployment's replicas are divided equally on member clusters")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deploy *appsv1.Deployment) bool {
					klog.Infof("Deployment(%s/%s)'s replcas is %d, expected: %d.",
						deploy.Namespace, deploy.Name, *deploy.Spec.Replicas, expectedReplicas)
					return *deploy.Spec.Replicas == expectedReplicas
				})
		})
	})

	// Case 4: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` is nil, trigger rescheduling when replicas have changed.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference is "+
		"nil, trigger rescheduling when replicas have changed.", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			}
		})

		ginkgo.It("replicas divided and weighted testing when rescheduling", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

			expectedReplicas := int32(3)
			updateReplicas := expectedReplicas * int32(len(framework.Clusters()))
			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateReplicas)

			klog.Infof("check if deployment's replicas are divided equally on member clusters")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deploy *appsv1.Deployment) bool {
					klog.Infof("Deployment(%s/%s)'s replcas is %d, expected: %d.",
						deploy.Namespace, deploy.Name, *deploy.Spec.Replicas, expectedReplicas)
					return *deploy.Spec.Replicas == expectedReplicas
				})
		})
	})

	// Case 5: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` isn't nil.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference isn't nil.", func() {
		ginkgo.BeforeEach(func() {
			policy.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference:          &policyv1alpha1.ClusterPreferences{},
			}

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

			sumReplicas := int32(sumWeight)
			deployment.Spec.Replicas = &sumReplicas
		})

		ginkgo.It("replicas divided and weighted testing", func() {
			ginkgo.By("check if deployment's replicas are divided equally on member clusters", func() {
				for index, cluster := range framework.Clusters() {
					expectedReplicas := int32(index + 1)

					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (int32, error) {
						memberDeployment, err := clusterClient.AppsV1().Deployments(deploymentNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Infof("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas)
						return *memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})
		})
	})

	// Case 6: `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` isn't nil, trigger rescheduling when replicas have changed.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference isn't "+
		"nil, trigger rescheduling when replicas have changed.", func() {
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
			policy.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					StaticWeightList: staticWeightLists,
				},
			}
		})

		ginkgo.It("replicas divided and weighted testing when rescheduling", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

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

						klog.Infof("Deployment(%s/%s)'s replcas is %d on cluster(%s), expected: %d.",
							deploymentNamespace, deploymentName, *memberDeployment.Spec.Replicas, cluster.Name, expectedReplicas)
						return *memberDeployment.Spec.Replicas, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})
		})
	})
})

// JobReplicaScheduling focus on job replica schedule testing.
var _ = ginkgo.Describe("[JobReplicaScheduling] JobReplicaSchedulingStrategy testing", func() {
	// `ReplicaSchedulingType` value is `Divided`, `ReplicaDivisionPreference` value is `Weighted`,
	// `WeightPreference` isn't nil, `spec.completions` isn't nil.
	ginkgo.Context("ReplicaSchedulingType is Divided, ReplicaDivisionPreference is Weighted, WeightPreference isn't nil, spec.completions isn`t nil.", func() {
		var policyNamespace, policyName string
		var jobNamespace, jobName string
		var job *batchv1.Job
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = jobNamePrefix + rand.String(RandomStrLength)
			jobNamespace = policyNamespace
			jobName = policyName

			job = helper.NewJob(jobNamespace, jobName)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: job.APIVersion,
					Kind:       job.Kind,
					Name:       job.Name,
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
		})

		ginkgo.AfterEach(func() {
			framework.RemoveJob(kubeClient, job.Namespace, job.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})

		ginkgo.It("job replicas divided and weighted testing", func() {
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
			job.Spec.Parallelism = &sumReplicas
			job.Spec.Completions = &sumReplicas
			framework.CreateJob(kubeClient, job)

			ginkgo.By("check if job's parallelism are divided equally on member clusters", func() {
				for index, cluster := range framework.Clusters() {
					expectedReplicas := int32(index + 1)

					clusterClient := framework.GetClusterClient(cluster.Name)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (int32, error) {
						memberJob, err := clusterClient.BatchV1().Jobs(jobNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Infof("Job(%s/%s)'s parallelism is %d on cluster(%s), expected: %d.",
							jobNamespace, jobName, *memberJob.Spec.Parallelism, cluster.Name, expectedReplicas)
						return *memberJob.Spec.Parallelism, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
				}
			})
		})
	})
})

// get the resource binding associated with the workload
func getResourceBinding(workload interface{}) (*workv1alpha2.ResourceBinding, error) {
	obj, err := utilhelper.ToUnstructured(workload)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	bindingName := names.GenerateBindingName(obj.GetKind(), obj.GetName())
	binding := &workv1alpha2.ResourceBinding{}

	err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Namespace: obj.GetNamespace(), Name: bindingName}, binding)
	return binding, err
}

// get the cluster resource binding associated with the workload
func getClusterResourceBinding(workload interface{}) (*workv1alpha2.ClusterResourceBinding, error) {
	obj, err := utilhelper.ToUnstructured(workload)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	bindingName := names.GenerateBindingName(obj.GetKind(), obj.GetName())
	binding := &workv1alpha2.ClusterResourceBinding{}

	err = controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: bindingName}, binding)
	return binding, err
}
