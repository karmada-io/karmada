package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Resource interpreter webhook testing", func() {
	ginkgo.Context("InterpreterOperation InterpretReplica testing", func() {
		policyNamespace := testNamespace
		policyName := workloadNamePrefix + rand.String(RandomStrLength)
		workloadNamespace := testNamespace
		workloadName := policyName

		workload := testhelper.NewWorkload(workloadNamespace, workloadName)
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
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

		ginkgo.It("InterpretReplica testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateWorkload(dynamicClient, workload)

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

			framework.RemoveWorkload(dynamicClient, workload.Namespace, workload.Name)
			framework.WaitWorkloadDisappearOnClusters(framework.ClusterNames(), workload.Namespace, workload.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	// Now only support push mode cluster for Retain testing
	// TODO(lonelyCZ): support pull mode cluster
	ginkgo.Context("InterpreterOperation Retain testing", func() {
		var waitTime = 5 * time.Second
		var updatedPaused = true

		policyNamespace := testNamespace
		policyName := workloadNamePrefix + rand.String(RandomStrLength)
		workloadNamespace := testNamespace
		workloadName := policyName
		pushModeClusters := []string{"member1", "member2"}

		workload := testhelper.NewWorkload(workloadNamespace, workloadName)
		policy := testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: workload.APIVersion,
				Kind:       workload.Kind,
				Name:       workload.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: pushModeClusters,
			},
		})

		ginkgo.It("Retain testing", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateWorkload(dynamicClient, workload)

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

			framework.RemoveWorkload(dynamicClient, workload.Namespace, workload.Name)
			framework.WaitWorkloadDisappearOnClusters(pushModeClusters, workload.Namespace, workload.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})
})
