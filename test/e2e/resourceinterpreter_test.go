package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Resource interpreter webhook testing", func() {
	var policyNamespace, policyName string
	var workloadNamespace, workloadName string
	var workload *workloadv1alpha1.Workload
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName = workloadNamePrefix + rand.String(RandomStrLength)
		workloadNamespace = testNamespace
		workloadName = policyName

		workload = testhelper.NewWorkload(workloadNamespace, workloadName)
		policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: workload.APIVersion,
				Kind:       workload.Kind,
				Name:       workload.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateWorkload(dynamicClient, workload)
		ginkgo.DeferCleanup(func() {
			framework.RemoveWorkload(dynamicClient, workload.Namespace, workload.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("InterpreterOperation InterpretReplica testing", func() {
		ginkgo.It("InterpretReplica testing", func() {
			ginkgo.By("check if workload's replica is interpreted", func() {
				resourceBindingName := names.GenerateBindingName(workload.Kind, workload.Name)
				expectedReplicas := *workload.Spec.Replicas

				gomega.Eventually(func(g gomega.Gomega) (int32, error) {
					resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(workload.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof(fmt.Sprintf("ResourceBinding(%s/%s)'s replicas is %d, expected: %d.",
						resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.Replicas, expectedReplicas))
					return resourceBinding.Spec.Replicas, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
			})
		})
	})

	// Now only support push mode cluster for Retain testing
	// TODO(lonelyCZ): support pull mode cluster
	ginkgo.Context("InterpreterOperation Retain testing", func() {
		var waitTime time.Duration
		var updatedPaused bool
		var pushModeClusters []string

		ginkgo.BeforeEach(func() {
			waitTime = 5 * time.Second
			updatedPaused = true
			pushModeClusters = []string{"member1", "member2"}

			policy.Spec.Placement.ClusterAffinity.ClusterNames = pushModeClusters
		})

		ginkgo.It("Retain testing", func() {
			ginkgo.By("update workload's spec.paused to true", func() {
				for _, cluster := range pushModeClusters {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Spec.Paused = updatedPaused
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster)
				}
			})

			// Wait executeController to reconcile then check if it is retained
			time.Sleep(waitTime)
			ginkgo.By("check if workload's spec.paused is retained", func() {
				for _, cluster := range pushModeClusters {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)

						return memberWorkload.Spec.Paused, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(updatedPaused))
				}
			})
		})
	})

	ginkgo.Context("InterpreterOperation ReviseReplica testing", func() {
		ginkgo.BeforeEach(func() {
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
			workload.Spec.Replicas = pointer.Int32Ptr(int32(sumWeight))
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: workload.APIVersion,
					Kind:       workload.Kind,
					Name:       workload.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: staticWeightLists,
					},
				},
			})
		})

		ginkgo.It("ReviseReplica testing", func() {
			for index, clusterName := range framework.ClusterNames() {
				framework.WaitWorkloadPresentOnClusterFitWith(clusterName, workload.Namespace, workload.Name, func(workload *workloadv1alpha1.Workload) bool {
					return *workload.Spec.Replicas == int32(index+1)
				})
			}
		})
	})

	ginkgo.Context("InterpreterOperation AggregateStatus testing", func() {
		ginkgo.It("AggregateStatus testing", func() {
			ginkgo.By("check whether the workload status can be correctly collected", func() {
				// Simulate the workload resource controller behavior, update the status information of workload resources of member clusters manually.
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Status.ReadyReplicas = *workload.Spec.Replicas
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")
				}

				wantedReplicas := *workload.Spec.Replicas * int32(len(framework.Clusters()))
				klog.Infof("Waiting for workload(%s/%s) collecting correctly status", workloadNamespace, workloadName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentWorkload := framework.GetWorkload(dynamicClient, workloadNamespace, workloadName)

					klog.Infof("workload(%s/%s) readyReplicas: %d, wanted replicas: %d", workloadNamespace, workloadName, currentWorkload.Status.ReadyReplicas, wantedReplicas)
					if currentWorkload.Status.ReadyReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("InterpreterOperation InterpretDependency testing", func() {
		var configMapNamespace, configMapName string

		ginkgo.BeforeEach(func() {
			configMapNamespace = testNamespace
			configMapName = configMapNamePrefix + rand.String(RandomStrLength)

			workload.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				}}}
			// configmaps should be propagated automatically.
			policy.Spec.PropagateDeps = true

			cm := testhelper.NewConfigMap(configMapNamespace, configMapName, map[string]string{"RUN_ENV": "test"})

			framework.CreateConfigMap(kubeClient, cm)
			ginkgo.DeferCleanup(func() {
				framework.RemoveConfigMap(kubeClient, configMapNamespace, configMapName)
			})
		})

		ginkgo.It("InterpretDependency testing", func() {
			ginkgo.By("check if workload's dependency is interpreted", func() {
				clusterNames := framework.ClusterNames()
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					var configmapNum int
					for _, clusterName := range clusterNames {
						clusterClient := framework.GetClusterClient(clusterName)
						gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
						if _, err := clusterClient.CoreV1().ConfigMaps(configMapNamespace).Get(context.TODO(), configMapName, metav1.GetOptions{}); err != nil {
							if apierrors.IsNotFound(err) {
								continue
							}
							g.Expect(err).NotTo(gomega.HaveOccurred())
						}
						configmapNum++
					}
					return configmapNum, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(len(clusterNames)))
			})
		})
	})
})
