package e2e

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("propagation with taint and toleration testing", func() {
	ginkgo.Context("Deployment propagation testing", func() {
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deploymentNamespace := testNamespace
		deploymentName := policyName
		deployment := helper.NewDeployment(deploymentNamespace, deploymentName)
		tolerationKey := "cluster-toleration.karmada.io"
		tolerationValue := "member1"

		// set clusterTolerations to tolerate taints in member1.
		clusterTolerations := []corev1.Toleration{
			{
				Key:      tolerationKey,
				Operator: corev1.TolerationOpEqual,
				Value:    tolerationValue,
				Effect:   corev1.TaintEffectNoSchedule,
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
				ClusterNames: clusterNames,
			},
			ClusterTolerations: clusterTolerations,
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("creating policy(%s/%s)", policyNamespace, policyName), func() {
				_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policyNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("adding taints to clusters", func() {
				for _, clusterName := range clusterNames {
					taints := constructAddedTaints(tolerationKey, clusterName)

					gomega.Eventually(func() bool {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						clusterObj.Spec.Taints = append(clusterObj.Spec.Taints, taints...)
						klog.Infof("update taints(%s) of cluster(%s)", clusterObj.Spec.Taints, clusterName)

						err = controlPlaneClient.Update(context.TODO(), clusterObj)
						if err != nil {
							klog.Errorf("Failed to update cluster(%s), err: %v", clusterName, err)
							return false
						}
						return true
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
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
			ginkgo.By("removing taints in cluster", func() {
				for _, clusterName := range clusterNames {
					gomega.Eventually(func() bool {
						clusterObj := &clusterv1alpha1.Cluster{}
						err := controlPlaneClient.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

						clusterObj.Spec.Taints = removeTargetFromSource(clusterObj.Spec.Taints, constructAddedTaints(tolerationKey, clusterName))
						klog.Infof("update taints(%s) of cluster(%s)", clusterObj.Spec.Taints, clusterName)

						err = controlPlaneClient.Update(context.TODO(), clusterObj)
						if err != nil {
							klog.Errorf("Failed to update cluster(%s), err: %v", clusterName, err)
							return false
						}
						return true
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				}
			})
		})

		ginkgo.It("deployment with cluster tolerations testing", func() {
			ginkgo.By(fmt.Sprintf("creating deployment(%s/%s)", deployment.Namespace, deployment.Name), func() {
				_, err := kubeClient.AppsV1().Deployments(testNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By(fmt.Sprintf("check if deployment(%s/%s) only scheduled to tolerated cluster(%s)", deploymentNamespace, deploymentName, tolerationValue), func() {
				targetClusterNames, err := getTargetClusterNames(deployment)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(len(targetClusterNames) == 1).Should(gomega.BeTrue())
				gomega.Expect(targetClusterNames[0] == tolerationValue).Should(gomega.BeTrue())
			})

			ginkgo.By(fmt.Sprintf("removing deployment(%s/%s)", deploymentNamespace, deploymentName), func() {
				err := kubeClient.AppsV1().Deployments(testNamespace).Delete(context.TODO(), deploymentName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})

func constructAddedTaints(tolerationKey, clusterName string) []corev1.Taint {
	return []corev1.Taint{
		{
			Key:    tolerationKey,
			Value:  clusterName,
			Effect: corev1.TaintEffectNoSchedule,
		},
	}
}

func removeTargetFromSource(source, target []corev1.Taint) []corev1.Taint {
	var result []corev1.Taint
	for si := range source {
		deleted := false
		for tj := range target {
			if source[si].MatchTaint(&target[tj]) {
				deleted = true
				break
			}
		}
		if !deleted {
			result = append(result, source[si])
		}
	}

	return result
}
