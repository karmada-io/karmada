/*
Copyright 2025 The Karmada Authors.

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
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[SchedulePriority] priority based resource scheduling", func() {
	ginkgo.Context("With KubePriorityClass source", func() {
		ginkgo.When("priorityclass exists", func() {
			var priorityClass *schedulingv1.PriorityClass
			var policy *policyv1alpha1.PropagationPolicy
			var deploymentName string

			ginkgo.JustBeforeEach(func() {
				framework.CreatePriorityClass(kubeClient, priorityClass)
				framework.CreatePropagationPolicy(karmadaClient, policy)
				deployment := testhelper.NewDeployment(testNamespace, deploymentName)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemovePriorityClass(kubeClient, priorityClass.Name)
				})
			})

			ginkgo.BeforeEach(func() {
				priorityClass = testhelper.NewPriorityClass(fmt.Sprintf("test-priority-%s", rand.String(RandomStrLength)), 1000)
				deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
				policy = testhelper.NewPropagationPolicy(testNamespace, deploymentName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deploymentName,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
				})
				policy.Spec.SchedulePriority = &policyv1alpha1.SchedulePriority{
					PriorityClassName:   priorityClass.Name,
					PriorityClassSource: policyv1alpha1.KubePriorityClass,
				}
			})

			ginkgo.It("ResourceBinding should inherit priority from propagationPolicy", func() {
				bindingName := names.GenerateBindingName("Deployment", deploymentName)
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, bindingName, func(binding *workv1alpha2.ResourceBinding) bool {
					return binding.Spec.SchedulePriority != nil && binding.Spec.SchedulePriority.Priority == priorityClass.Value
				})
			})
		})

		ginkgo.When("priorityclass does not exist", func() {
			var policy *policyv1alpha1.PropagationPolicy
			var deploymentName string

			ginkgo.JustBeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				deployment := testhelper.NewDeployment(testNamespace, deploymentName)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				})
			})

			ginkgo.BeforeEach(func() {
				deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
				policy = testhelper.NewPropagationPolicy(testNamespace, deploymentName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deploymentName,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
				})
				policy.Spec.SchedulePriority = &policyv1alpha1.SchedulePriority{
					PriorityClassName:   "non-existent-priority-class",
					PriorityClassSource: policyv1alpha1.KubePriorityClass,
				}
			})

			ginkgo.It("ResourceBinding should not be created", func() {
				bindingName := names.GenerateBindingName("Deployment", deploymentName)
				gomega.Consistently(func(g gomega.Gomega) {
					_, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), bindingName, metav1.GetOptions{})
					g.Expect(err).Should(gomega.HaveOccurred())
					g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
				}, pollTimeout, pollInterval).Should(gomega.Succeed())
			})
		})
	})

	// Future priority source tests can be added here as new contexts
	// ginkgo.Context("With PodPriorityClass source", func() {...})
	// ginkgo.Context("With FederatedPriorityClass source", func() {...})
})
