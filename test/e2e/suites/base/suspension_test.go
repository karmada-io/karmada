/*
Copyright 2024 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package base

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Suspension testing", func() {
	var deploymentName string
	var policyName string
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		deploymentName = fmt.Sprintf("deployment-%s", rand.String(RandomStrLength))
		policyName = fmt.Sprintf("policy-%s", rand.String(RandomStrLength))

		deployment = helper.NewDeployment(testNamespace, deploymentName)
		policy = helper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
		// Set initial suspension state to true
		dispatching := true
		policy.Spec.Suspension = &policyv1alpha1.Suspension{
			Dispatching: &dispatching,
		}
	})

	ginkgo.AfterEach(func() {
		framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
	})

	ginkgo.It("Test suspension dispatching with PropagationPolicy", func() {
		ginkgo.By("Create PropagationPolicy with suspension.dispatching=true", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
		})

		ginkgo.By("Create Deployment with octopus/user.submitted=false", func() {
			deployment.Annotations = map[string]string{
				"octopus/user.submitted": "false",
			}
			framework.CreateDeployment(kubeClient, deployment)
		})

		ginkgo.By("Wait for ResourceBinding with suspension.dispatching=true", func() {
			gomega.Eventually(func() bool {
				rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), names.GenerateBindingName(deployment.Kind, deployment.Name), metav1.GetOptions{})
				if err != nil {
					return false
				}
				return rb.Spec.Suspension != nil && rb.Spec.Suspension.Dispatching != nil && *rb.Spec.Suspension.Dispatching
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})

		ginkgo.By("Patch PropagationPolicy to set suspension.dispatching=false", func() {
			patch := []byte(`{"spec":{"suspension":{"dispatching":false}}}`)
			_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(testNamespace).Patch(context.TODO(), policyName, types.MergePatchType, patch, metav1.PatchOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Patch Deployment to set octopus/user.submitted=true", func() {
			patch := []byte(`{"metadata":{"annotations":{"octopus/user.submitted":"true"}}}`)
			_, err := kubeClient.AppsV1().Deployments(testNamespace).Patch(context.TODO(), deploymentName, types.MergePatchType, patch, metav1.PatchOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Wait for ResourceBinding with suspension.dispatching=false", func() {
			gomega.Eventually(func() bool {
				rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), names.GenerateBindingName(deployment.Kind, deployment.Name), metav1.GetOptions{})
				if err != nil {
					return false
				}
				return rb.Spec.Suspension != nil && rb.Spec.Suspension.Dispatching != nil && !*rb.Spec.Suspension.Dispatching
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})

		ginkgo.By("Wait for Deployment and Pod to be created in member clusters", func() {
			for _, cluster := range framework.ClusterNames() {
				clusterClient := framework.GetClusterClient(cluster)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				gomega.Eventually(func() bool {
					_, err := clusterClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return true
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())

				gomega.Eventually(func() bool {
					pods, err := clusterClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: fmt.Sprintf("app=%s", deploymentName),
					})
					if err != nil {
						return false
					}
					return len(pods.Items) > 0
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			}
		})
	})
})

var _ = ginkgo.Describe("Cluster Suspension testing", func() {
	var deploymentName string
	var policyName string
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.ClusterPropagationPolicy

	ginkgo.BeforeEach(func() {
		deploymentName = fmt.Sprintf("deployment-%s", rand.String(RandomStrLength))
		policyName = fmt.Sprintf("policy-%s", rand.String(RandomStrLength))

		deployment = helper.NewDeployment(testNamespace, deploymentName)
		policy = helper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
				Namespace:  testNamespace,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
		// Set initial suspension state to true
		dispatching := true
		policy.Spec.Suspension = &policyv1alpha1.Suspension{
			Dispatching: &dispatching,
		}
	})

	ginkgo.AfterEach(func() {
		framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
	})

	ginkgo.It("Test suspension dispatching with ClusterPropagationPolicy", func() {
		ginkgo.By("Create ClusterPropagationPolicy with suspension.dispatching=true", func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
		})

		ginkgo.By("Create Deployment with octopus/user.submitted=false", func() {
			deployment.Annotations = map[string]string{
				"octopus/user.submitted": "false",
			}
			framework.CreateDeployment(kubeClient, deployment)
		})

		ginkgo.By("Wait for ResourceBinding with suspension.dispatching=true", func() {
			gomega.Eventually(func() bool {
				rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), names.GenerateBindingName(deployment.Kind, deployment.Name), metav1.GetOptions{})
				if err != nil {
					return false
				}
				return rb.Spec.Suspension != nil && rb.Spec.Suspension.Dispatching != nil && *rb.Spec.Suspension.Dispatching
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})

		ginkgo.By("Patch ClusterPropagationPolicy to set suspension.dispatching=false", func() {
			patch := []byte(`{"spec":{"suspension":{"dispatching":false}}}`)
			_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Patch(context.TODO(), policyName, types.MergePatchType, patch, metav1.PatchOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Patch Deployment to set octopus/user.submitted=true", func() {
			patch := []byte(`{"metadata":{"annotations":{"octopus/user.submitted":"true"}}}`)
			_, err := kubeClient.AppsV1().Deployments(testNamespace).Patch(context.TODO(), deploymentName, types.MergePatchType, patch, metav1.PatchOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("Wait for ResourceBinding with suspension.dispatching=false", func() {
			gomega.Eventually(func() bool {
				rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), names.GenerateBindingName(deployment.Kind, deployment.Name), metav1.GetOptions{})
				if err != nil {
					return false
				}
				return rb.Spec.Suspension != nil && rb.Spec.Suspension.Dispatching != nil && !*rb.Spec.Suspension.Dispatching
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})

		ginkgo.By("Wait for Deployment and Pod to be created in member clusters", func() {
			for _, cluster := range framework.ClusterNames() {
				clusterClient := framework.GetClusterClient(cluster)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				gomega.Eventually(func() bool {
					_, err := clusterClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
					if err != nil {
						return false
					}
					return true
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())

				gomega.Eventually(func() bool {
					pods, err := clusterClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: fmt.Sprintf("app=%s", deploymentName),
					})
					if err != nil {
						return false
					}
					return len(pods.Items) > 0
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			}
		})
	})
})
