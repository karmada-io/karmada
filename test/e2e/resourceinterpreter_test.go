package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
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

	ginkgo.Context("InterpreterOperation Retain testing", func() {
		var waitTime time.Duration
		var updatedPaused bool

		ginkgo.BeforeEach(func() {
			waitTime = 5 * time.Second
			updatedPaused = true

			policy.Spec.Placement.ClusterAffinity.ClusterNames = framework.ClusterNames()
		})

		ginkgo.It("Retain testing", func() {
			ginkgo.By("update workload's spec.paused to true", func() {
				for _, cluster := range framework.ClusterNames() {
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
				for _, cluster := range framework.ClusterNames() {
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

	ginkgo.Context("InterpreterOperation InterpretStatus testing", func() {
		ginkgo.It("InterpretStatus testing", func() {
			for _, cluster := range framework.ClusterNames() {
				clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
				gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

				memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
				memberWorkload.Status.ReadyReplicas = *workload.Spec.Replicas
				framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")

				workName := names.GenerateWorkName(workload.Kind, workload.Name, workload.Namespace)
				workNamespace, err := names.GenerateExecutionSpaceName(cluster)
				gomega.Expect(err).Should(gomega.BeNil())

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					work, err := karmadaClient.WorkV1alpha1().Works(workNamespace).Get(context.TODO(), workName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if len(work.Status.ManifestStatuses) == 0 || work.Status.ManifestStatuses[0].Status == nil {
						return false, nil
					}

					var observedStatus workloadv1alpha1.WorkloadStatus
					err = json.Unmarshal(work.Status.ManifestStatuses[0].Status.Raw, &observedStatus)
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof("work(%s/%s) readyReplicas: %d, want: %d", workNamespace, workName, observedStatus.ReadyReplicas, *workload.Spec.Replicas)

					// not collect status.conditions in webhook
					klog.Infof("work(%s/%s) length of conditions: %v, want: %v", workNamespace, workName, len(observedStatus.Conditions), 0)

					if observedStatus.ReadyReplicas == *workload.Spec.Replicas && len(observedStatus.Conditions) == 0 {
						return true, nil
					}
					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			}
		})
	})
})
