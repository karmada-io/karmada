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
	_ "embed"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/yaml"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var (
	//go:embed manifest/flinkdeployments.flink.apache.org-v1.yaml
	flinkDeploymentCRDYAML string

	//go:embed manifest/flinkdeployment-cr.yaml
	flinkDeploymentCRYAML string
)

var _ = ginkgo.Describe("[ScheduleMultiTemplate] schedule multi template resource", func() {

	ginkgo.Context("FlinkDeployment scheduling", func() {
		var (
			flinkDeploymentCRD       apiextensionsv1.CustomResourceDefinition
			flinkDeploymentNamespace string
			flinkDeploymentName      string
			flinkDeploymentObj       *unstructured.Unstructured
			flinkDeploymentGVR       schema.GroupVersionResource
		)

		ginkgo.BeforeEach(func() {
			ginkgo.By("create FlinkDeployment CRD on karmada control plane", func() {
				err := yaml.Unmarshal([]byte(flinkDeploymentCRDYAML), &flinkDeploymentCRD)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.CreateCRD(dynamicClient, &flinkDeploymentCRD)
				ginkgo.DeferCleanup(func() {
					framework.RemoveCRD(dynamicClient, flinkDeploymentCRD.Name)
				})
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("propagate FlinkDeployment CRD to all clusters", func() {
				cpp := testhelper.NewClusterPropagationPolicy("flink-deployment-cpp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: flinkDeploymentCRD.APIVersion,
						Kind:       flinkDeploymentCRD.Kind,
						Name:       flinkDeploymentCRD.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
				})

				framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
				framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
					fmt.Sprintf("%s/%s", flinkDeploymentCRD.Spec.Group, "v1beta1"), flinkDeploymentCRD.Spec.Names.Kind)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
				})
			})
		})

		ginkgo.It("base case: propagate FlinkDeployment to one cluster", func() {
			flinkDeploymentNamespace = testNamespace
			flinkDeploymentName = fmt.Sprintf("flinkdeployment-%s", rand.String(RandomStrLength))

			ginkgo.By("create FlinkDeployment on karmada control plane", func() {
				flinkDeploymentObj = &unstructured.Unstructured{}
				err := yaml.Unmarshal([]byte(flinkDeploymentCRYAML), flinkDeploymentObj)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				flinkDeploymentObj.SetNamespace(flinkDeploymentNamespace)
				flinkDeploymentObj.SetName(flinkDeploymentName)

				flinkDeploymentGVR = schema.GroupVersionResource{
					Group:    flinkDeploymentObj.GroupVersionKind().Group,
					Version:  flinkDeploymentObj.GroupVersionKind().Version,
					Resource: "flinkdeployments",
				}

				_, err = dynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
					Create(context.Background(), flinkDeploymentObj, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				ginkgo.DeferCleanup(func() {
					err := dynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
						Delete(context.Background(), flinkDeploymentName, metav1.DeleteOptions{})
					if err != nil && !apierrors.IsNotFound(err) {
						// Ignore not found errors during cleanup, but other errors should be handled
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					}
				})
			})

			ginkgo.By("propagate FlinkDeployment resource to one cluster", func() {
				pp := testhelper.NewPropagationPolicy(flinkDeploymentNamespace, flinkDeploymentName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: flinkDeploymentObj.GetAPIVersion(),
						Kind:       flinkDeploymentObj.GetKind(),
						Name:       flinkDeploymentObj.GetName(),
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
					},
				})
				framework.CreatePropagationPolicy(karmadaClient, pp)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, pp.Namespace, pp.Name)
				})
			})

			bindingName := names.GenerateBindingName(flinkDeploymentObj.GetKind(), flinkDeploymentObj.GetName())

			// Helper function to find component by name
			findComponent := func(components []workv1alpha2.Component, name string) *workv1alpha2.Component {
				for i := range components {
					if components[i].Name == name {
						return &components[i]
					}
				}
				return nil
			}

			// Helper function to verify component resource requirements
			verifyComponentResources := func(component *workv1alpha2.Component, expectedReplicas int32, expectedCPU, expectedMemory string) {
				gomega.Expect(component).ShouldNot(gomega.BeNil())
				gomega.Expect(component.Replicas).Should(gomega.Equal(expectedReplicas))
				gomega.Expect(component.ReplicaRequirements).ShouldNot(gomega.BeNil())
				gomega.Expect(component.ReplicaRequirements.ResourceRequest).Should(gomega.Equal(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(expectedCPU),
					corev1.ResourceMemory: resource.MustParse(expectedMemory),
				}))
			}

			ginkgo.By("verify ResourceBinding components field", func() {
				var resourceBinding *workv1alpha2.ResourceBinding
				var err error

				// Wait for ResourceBinding to be created and components field to be populated
				gomega.Eventually(func(g gomega.Gomega) {
					resourceBinding, err = karmadaClient.WorkV1alpha2().ResourceBindings(flinkDeploymentNamespace).
						Get(context.Background(), bindingName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					g.Expect(resourceBinding).ShouldNot(gomega.BeNil())
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())

				// Ensure components field exists and has the expected number of components
				gomega.Expect(resourceBinding.Spec.Components).ShouldNot(gomega.BeNil())
				gomega.Expect(resourceBinding.Spec.Components).Should(gomega.HaveLen(2), "components should have exactly 2 items")
				// Find both components
				jobManagerComponent := findComponent(resourceBinding.Spec.Components, "jobmanager")
				taskManagerComponent := findComponent(resourceBinding.Spec.Components, "taskmanager")
				gomega.Expect(jobManagerComponent).ShouldNot(gomega.BeNil(), "jobmanager component should exist")
				gomega.Expect(taskManagerComponent).ShouldNot(gomega.BeNil(), "taskmanager component should exist")
				// Verify the detailed content of components (including ReplicaRequirements and resource values)
				verifyComponentResources(jobManagerComponent, 1, "1", "100m")
				verifyComponentResources(taskManagerComponent, 1, "1", "100m")
			})

			var targetClusterName string
			ginkgo.By("verify ResourceBinding scheduling result", func() {
				var resourceBinding *workv1alpha2.ResourceBinding
				var err error
				// Wait for ResourceBinding to be scheduled (check Scheduled condition)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					resourceBinding, err = karmadaClient.WorkV1alpha2().ResourceBindings(flinkDeploymentNamespace).
						Get(context.Background(), bindingName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(resourceBinding.Status.Conditions, workv1alpha2.Scheduled), nil
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Equal(true))

				// Verify scheduling result: exactly one cluster
				gomega.Expect(resourceBinding.Spec.Clusters).Should(gomega.HaveLen(1))
				targetClusterName = resourceBinding.Spec.Clusters[0].Name
				gomega.Expect(targetClusterName).ShouldNot(gomega.BeEmpty())
			})

			ginkgo.By("wait for FlinkDeployment to be created on the scheduled cluster", func() {
				// Verify FlinkDeployment is propagated to the scheduled cluster
				clusterDynamicClient := framework.GetClusterDynamicClient(targetClusterName)
				gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

				gomega.Eventually(func(g gomega.Gomega) {
					_, err := clusterDynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
						Get(context.Background(), flinkDeploymentName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred(), "FlinkDeployment should be present on cluster %s", targetClusterName)
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
			})
		})
	})
})
