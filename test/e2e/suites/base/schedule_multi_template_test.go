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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var (
	//go:embed manifest/flinkdeployments.flink.apache.org-v1.yaml
	flinkDeploymentCRDYAML string

	//go:embed manifest/flinkdeployment-cr.yaml
	flinkDeploymentCRYAML string

	//go:embed manifest/batch.volcano.sh_jobs.yaml
	volcanoJobCRDYAML string

	//go:embed manifest/volcanojob-cr.yaml
	volcanoJobCRYAML string
)

var _ = framework.SerialDescribe("[ScheduleMultiTemplate] schedule multi template resource", func() {
	// Helper function to find component by name
	findComponent := func(components []workv1alpha2.Component, name string) *workv1alpha2.Component {
		for i := range components {
			if components[i].Name == name {
				return &components[i]
			}
		}
		return nil
	}
	findTargetComponent := func(components []workv1alpha2.TargetComponent, name string) *workv1alpha2.TargetComponent {
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
				framework.WaitCRDEstablished(dynamicClient, flinkDeploymentCRD.Name)
				ginkgo.DeferCleanup(func() {
					framework.RemoveCRD(dynamicClient, flinkDeploymentCRD.Name)
					framework.WaitCRDDisappeared(dynamicClient, flinkDeploymentCRD.Name)
					framework.WaitCRDDisappearedOnClusters(framework.ClusterNames(), flinkDeploymentCRD.Name)
					framework.WaitCRDDisappearedFromClusterStatus(karmadaClient, framework.ClusterNames(),
						fmt.Sprintf("%s/%s", flinkDeploymentCRD.Spec.Group, "v1beta1"), flinkDeploymentCRD.Spec.Names.Kind)
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

		ginkgo.It("reschedules FlinkDeployment components without migrating or propagating rejected updates", func(ctx context.Context) {
			flinkDeploymentNamespace = fmt.Sprintf("karmadatest-flink-scale-%s", rand.String(RandomStrLength))
			flinkDeploymentName = fmt.Sprintf("flinkdeployment-%s", rand.String(RandomStrLength))

			ginkgo.By("create an isolated namespace on the control plane and member clusters", func() {
				err := setupTestNamespace(flinkDeploymentNamespace, kubeClient)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				ginkgo.DeferCleanup(func() {
					err := cleanupTestNamespace(flinkDeploymentNamespace, kubeClient)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
				framework.WaitNamespacePresentOnClusters(framework.ClusterNames(), flinkDeploymentNamespace)
			})

			ginkgo.By("limit every candidate cluster to 250m CPU in the isolated namespace", func() {
				const quotaName = "flink-component-scale"
				expectedCPU := resource.MustParse("250m")
				for _, clusterName := range framework.ClusterNames() {
					clusterClient := framework.GetClusterClient(clusterName)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
					quota := &corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{
							Name:      quotaName,
							Namespace: flinkDeploymentNamespace,
						},
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								corev1.ResourceCPU: expectedCPU,
							},
						},
					}
					framework.CreateResourceQuota(clusterClient, quota)

					currentClusterName := clusterName
					ginkgo.DeferCleanup(func() {
						framework.RemoveResourceQuota(framework.GetClusterClient(currentClusterName), flinkDeploymentNamespace, quotaName)
					})

					gomega.Eventually(func(g gomega.Gomega) {
						currentQuota, err := clusterClient.CoreV1().ResourceQuotas(flinkDeploymentNamespace).
							Get(ctx, quotaName, metav1.GetOptions{})
						g.Expect(err).ShouldNot(gomega.HaveOccurred())
						actualCPU, found := currentQuota.Status.Hard[corev1.ResourceCPU]
						g.Expect(found).Should(gomega.BeTrue())
						g.Expect(actualCPU.Cmp(expectedCPU)).Should(gomega.Equal(0))
					}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
				}
			})

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
					Create(ctx, flinkDeploymentObj, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				ginkgo.DeferCleanup(func() {
					err := dynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
						Delete(context.Background(), flinkDeploymentName, metav1.DeleteOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
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
			workName := names.GenerateWorkName(flinkDeploymentObj.GetKind(), flinkDeploymentName, flinkDeploymentNamespace)

			updateFlinkDeployment := func(mutate func(*unstructured.Unstructured) error) {
				gomega.Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
					current, err := dynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
						Get(ctx, flinkDeploymentName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if err = mutate(current); err != nil {
						return err
					}
					_, err = dynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
						Update(ctx, current, metav1.UpdateOptions{})
					return err
				})).Should(gomega.Succeed())
			}

			assertFlinkState := func(g gomega.Gomega, object *unstructured.Unstructured, expectedTaskManagers int64, expectedTaskManagerCPU string) {
				jobManagers, found, err := unstructured.NestedInt64(object.Object, "spec", "jobManager", "replicas")
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(found).Should(gomega.BeTrue())
				g.Expect(jobManagers).Should(gomega.Equal(int64(1)))

				taskManagers, found, err := unstructured.NestedInt64(object.Object, "spec", "taskManager", "replicas")
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(found).Should(gomega.BeTrue())
				g.Expect(taskManagers).Should(gomega.Equal(expectedTaskManagers))

				cpuValue, found, err := unstructured.NestedFieldNoCopy(object.Object, "spec", "taskManager", "resource", "cpu")
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(found).Should(gomega.BeTrue())
				actualCPU, err := resource.ParseQuantity(fmt.Sprint(cpuValue))
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(actualCPU.Cmp(resource.MustParse(expectedTaskManagerCPU))).Should(gomega.Equal(0))
			}

			var targetClusterName string
			assertBindingState := func(g gomega.Gomega, expectedDesiredTaskManagers, expectedAcceptedTaskManagers int32,
				expectedDesiredTaskManagerCPU string, expectedStatus metav1.ConditionStatus, expectedReason, expectedMessage string) *workv1alpha2.ResourceBinding {
				resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(flinkDeploymentNamespace).
					Get(ctx, bindingName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(resourceBinding.Spec.Components).Should(gomega.HaveLen(2))

				jobManagerComponent := findComponent(resourceBinding.Spec.Components, "jobmanager")
				taskManagerComponent := findComponent(resourceBinding.Spec.Components, "taskmanager")
				g.Expect(jobManagerComponent).ShouldNot(gomega.BeNil())
				g.Expect(jobManagerComponent.Replicas).Should(gomega.Equal(int32(1)))
				g.Expect(jobManagerComponent.ReplicaRequirements).ShouldNot(gomega.BeNil())
				g.Expect(jobManagerComponent.ReplicaRequirements.ResourceRequest).Should(gomega.Equal(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("100m"),
				}))
				g.Expect(taskManagerComponent).ShouldNot(gomega.BeNil())
				g.Expect(taskManagerComponent.Replicas).Should(gomega.Equal(expectedDesiredTaskManagers))
				g.Expect(taskManagerComponent.ReplicaRequirements).ShouldNot(gomega.BeNil())
				g.Expect(taskManagerComponent.ReplicaRequirements.ResourceRequest).Should(gomega.Equal(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(expectedDesiredTaskManagerCPU),
					corev1.ResourceMemory: resource.MustParse("100m"),
				}))

				g.Expect(resourceBinding.Spec.Clusters).Should(gomega.HaveLen(1))
				if targetClusterName != "" {
					g.Expect(resourceBinding.Spec.Clusters[0].Name).Should(gomega.Equal(targetClusterName))
				}
				g.Expect(resourceBinding.Spec.Clusters[0].Components).Should(gomega.HaveLen(2))
				acceptedJobManager := findTargetComponent(resourceBinding.Spec.Clusters[0].Components, "jobmanager")
				acceptedTaskManager := findTargetComponent(resourceBinding.Spec.Clusters[0].Components, "taskmanager")
				g.Expect(acceptedJobManager).ShouldNot(gomega.BeNil())
				g.Expect(acceptedJobManager.Replicas).Should(gomega.Equal(int32(1)))
				g.Expect(acceptedTaskManager).ShouldNot(gomega.BeNil())
				g.Expect(acceptedTaskManager.Replicas).Should(gomega.Equal(expectedAcceptedTaskManagers))

				condition := meta.FindStatusCondition(resourceBinding.Status.Conditions, workv1alpha2.Scheduled)
				g.Expect(condition).ShouldNot(gomega.BeNil())
				g.Expect(condition.Status).Should(gomega.Equal(expectedStatus))
				if expectedReason != "" {
					g.Expect(condition.Reason).Should(gomega.Equal(expectedReason))
				}
				if expectedMessage != "" {
					g.Expect(condition.Message).Should(gomega.ContainSubstring(expectedMessage))
				}
				if expectedStatus == metav1.ConditionTrue {
					g.Expect(resourceBinding.Status.SchedulerObservedGeneration).Should(gomega.Equal(resourceBinding.Generation))
				} else {
					g.Expect(resourceBinding.Status.SchedulerObservedGeneration).Should(gomega.BeNumerically("<", resourceBinding.Generation))
				}
				return resourceBinding
			}

			waitForBindingState := func(expectedDesiredTaskManagers, expectedAcceptedTaskManagers int32,
				expectedDesiredTaskManagerCPU string, expectedStatus metav1.ConditionStatus, expectedReason, expectedMessage string) *workv1alpha2.ResourceBinding {
				var resourceBinding *workv1alpha2.ResourceBinding
				gomega.Eventually(func(g gomega.Gomega) {
					resourceBinding = assertBindingState(g, expectedDesiredTaskManagers, expectedAcceptedTaskManagers,
						expectedDesiredTaskManagerCPU, expectedStatus, expectedReason, expectedMessage)
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
				return resourceBinding
			}

			type deliveredRevision struct {
				memberUID                string
				memberGeneration         int64
				workUID                  string
				workGeneration           int64
				acceptedRequirementsHash string
			}

			assertDeliveredState := func(g gomega.Gomega, expectedTaskManagers int64, expectedTaskManagerCPU string) deliveredRevision {
				clusterDynamicClient := framework.GetClusterDynamicClient(targetClusterName)
				g.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())
				memberFlinkDeployment, err := clusterDynamicClient.Resource(flinkDeploymentGVR).Namespace(flinkDeploymentNamespace).
					Get(ctx, flinkDeploymentName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				assertFlinkState(g, memberFlinkDeployment, expectedTaskManagers, expectedTaskManagerCPU)

				workNamespace := names.GenerateExecutionSpaceName(targetClusterName)
				work, err := karmadaClient.WorkV1alpha1().Works(workNamespace).Get(ctx, workName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(work.Spec.Workload.Manifests).Should(gomega.HaveLen(1))
				workload := &unstructured.Unstructured{}
				g.Expect(workload.UnmarshalJSON(work.Spec.Workload.Manifests[0].Raw)).Should(gomega.Succeed())
				assertFlinkState(g, workload, expectedTaskManagers, expectedTaskManagerCPU)

				resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(flinkDeploymentNamespace).
					Get(ctx, bindingName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				acceptedRequirementsHash := resourceBinding.Annotations[util.AcceptedComponentRequirementsHashAnnotation]
				g.Expect(acceptedRequirementsHash).ShouldNot(gomega.BeEmpty())

				return deliveredRevision{
					memberUID:                string(memberFlinkDeployment.GetUID()),
					memberGeneration:         memberFlinkDeployment.GetGeneration(),
					workUID:                  string(work.UID),
					workGeneration:           work.Generation,
					acceptedRequirementsHash: acceptedRequirementsHash,
				}
			}

			waitForDeliveredState := func(expectedTaskManagers int64, expectedTaskManagerCPU string) {
				gomega.Eventually(func(g gomega.Gomega) {
					assertDeliveredState(g, expectedTaskManagers, expectedTaskManagerCPU)
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
			}
			captureDeliveredRevision := func(expectedTaskManagers int64, expectedTaskManagerCPU string) deliveredRevision {
				var revision deliveredRevision
				gomega.Eventually(func(g gomega.Gomega) {
					revision = assertDeliveredState(g, expectedTaskManagers, expectedTaskManagerCPU)
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
				return revision
			}

			assertRejectedStateConsistently := func(expectedDesiredTaskManagers, expectedAcceptedTaskManagers int32,
				expectedDesiredTaskManagerCPU, expectedDeliveredTaskManagerCPU, expectedReason, expectedMessage string,
				acceptedRevision deliveredRevision) {
				gomega.Consistently(func(g gomega.Gomega) {
					resourceBinding := assertBindingState(g, expectedDesiredTaskManagers, expectedAcceptedTaskManagers,
						expectedDesiredTaskManagerCPU, metav1.ConditionFalse, expectedReason, expectedMessage)
					g.Expect(resourceBinding.Annotations[util.AcceptedComponentRequirementsHashAnnotation]).Should(
						gomega.Equal(acceptedRevision.acceptedRequirementsHash))
					currentRevision := assertDeliveredState(g, int64(expectedAcceptedTaskManagers), expectedDeliveredTaskManagerCPU)
					g.Expect(currentRevision.memberUID).Should(gomega.Equal(acceptedRevision.memberUID))
					g.Expect(currentRevision.memberGeneration).Should(gomega.Equal(acceptedRevision.memberGeneration))
					g.Expect(currentRevision.workUID).Should(gomega.Equal(acceptedRevision.workUID))
					g.Expect(currentRevision.workGeneration).Should(gomega.Equal(acceptedRevision.workGeneration))
				}, 3*framework.PollInterval, framework.PollInterval).Should(gomega.Succeed())
			}

			ginkgo.By("verify the initial component result is delivered to exactly one cluster", func() {
				resourceBinding := waitForBindingState(1, 1, "100m", metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				targetClusterName = resourceBinding.Spec.Clusters[0].Name
				gomega.Expect(targetClusterName).ShouldNot(gomega.BeEmpty())
				waitForDeliveredState(1, "100m")
			})

			ginkgo.By("scale taskmanager up on the accepted cluster", func() {
				updateFlinkDeployment(func(object *unstructured.Unstructured) error {
					return unstructured.SetNestedField(object.Object, int64(2), "spec", "taskManager", "replicas")
				})
				waitForBindingState(2, 2, "100m", metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				waitForDeliveredState(2, "100m")
			})

			ginkgo.By("scale taskmanager down on the accepted cluster", func() {
				updateFlinkDeployment(func(object *unstructured.Unstructured) error {
					return unstructured.SetNestedField(object.Object, int64(1), "spec", "taskManager", "replicas")
				})
				waitForBindingState(1, 1, "100m", metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				waitForDeliveredState(1, "100m")
			})

			ginkgo.By("retain the accepted result when taskmanager scale-up exceeds available CPU", func() {
				acceptedRevision := captureDeliveredRevision(1, "100m")
				updateFlinkDeployment(func(object *unstructured.Unstructured) error {
					return unstructured.SetNestedField(object.Object, int64(4), "spec", "taskManager", "replicas")
				})
				waitForBindingState(4, 1, "100m", metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "")
				assertRejectedStateConsistently(4, 1, "100m", "100m", workv1alpha2.BindingReasonUnschedulable, "", acceptedRevision)
			})

			ginkgo.By("restore the source replicas to the accepted result", func() {
				updateFlinkDeployment(func(object *unstructured.Unstructured) error {
					return unstructured.SetNestedField(object.Object, int64(1), "spec", "taskManager", "replicas")
				})
				waitForBindingState(1, 1, "100m", metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				waitForDeliveredState(1, "100m")
			})

			ginkgo.By("reject resource requirement changes without updating the accepted Work", func() {
				acceptedRevision := captureDeliveredRevision(1, "100m")
				updateFlinkDeployment(func(object *unstructured.Unstructured) error {
					return unstructured.SetNestedField(object.Object, float64(0.2), "spec", "taskManager", "resource", "cpu")
				})
				waitForBindingState(1, 1, "200m", metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "")
				assertRejectedStateConsistently(1, 1, "200m", "100m", workv1alpha2.BindingReasonUnschedulable, "", acceptedRevision)
			})

			ginkgo.By("restore the accepted resource requirements after rejection", func() {
				updateFlinkDeployment(func(object *unstructured.Unstructured) error {
					return unstructured.SetNestedField(object.Object, float64(0.1), "spec", "taskManager", "resource", "cpu")
				})
				waitForBindingState(1, 1, "100m", metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				waitForDeliveredState(1, "100m")
			})
		})
	})

	ginkgo.Context("VolcanoJob scheduling", func() {
		var (
			volcanoJobCRD       apiextensionsv1.CustomResourceDefinition
			volcanoJobNamespace string
			volcanoJobName      string
			volcanoJobObj       *unstructured.Unstructured
			volcanoJobGVR       schema.GroupVersionResource
		)

		ginkgo.BeforeEach(func() {
			ginkgo.By("create VolcanoJob CRD on karmada control plane", func() {
				err := yaml.Unmarshal([]byte(volcanoJobCRDYAML), &volcanoJobCRD)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.CreateCRD(dynamicClient, &volcanoJobCRD)
				ginkgo.DeferCleanup(func() {
					framework.RemoveCRD(dynamicClient, volcanoJobCRD.Name)
				})
			})
		})

		ginkgo.BeforeEach(func() {
			ginkgo.By("propagate VolcanoJob CRD to all clusters", func() {
				cpp := testhelper.NewClusterPropagationPolicy("volcano-job-cpp", []policyv1alpha1.ResourceSelector{
					{
						APIVersion: volcanoJobCRD.APIVersion,
						Kind:       volcanoJobCRD.Kind,
						Name:       volcanoJobCRD.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
				})

				framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
				framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
					fmt.Sprintf("%s/%s", volcanoJobCRD.Spec.Group, "v1alpha1"), volcanoJobCRD.Spec.Names.Kind)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
				})
			})
		})

		ginkgo.It("base case: propagate VolcanoJob to one cluster", func() {
			volcanoJobNamespace = testNamespace
			volcanoJobName = fmt.Sprintf("volcanojob-%s", rand.String(RandomStrLength))

			ginkgo.By("create VolcanoJob on karmada control plane", func() {
				volcanoJobObj = &unstructured.Unstructured{}
				err := yaml.Unmarshal([]byte(volcanoJobCRYAML), volcanoJobObj)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				volcanoJobObj.SetNamespace(volcanoJobNamespace)
				volcanoJobObj.SetName(volcanoJobName)

				volcanoJobGVR = schema.GroupVersionResource{
					Group:    volcanoJobObj.GroupVersionKind().Group,
					Version:  volcanoJobObj.GroupVersionKind().Version,
					Resource: "jobs",
				}

				_, err = dynamicClient.Resource(volcanoJobGVR).Namespace(volcanoJobNamespace).
					Create(context.Background(), volcanoJobObj, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				ginkgo.DeferCleanup(func() {
					err := dynamicClient.Resource(volcanoJobGVR).Namespace(volcanoJobNamespace).
						Delete(context.Background(), volcanoJobName, metav1.DeleteOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
			})

			ginkgo.By("propagate VolcanoJob resource to one cluster", func() {
				pp := testhelper.NewPropagationPolicy(volcanoJobNamespace, volcanoJobName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: volcanoJobObj.GetAPIVersion(),
						Kind:       volcanoJobObj.GetKind(),
						Name:       volcanoJobObj.GetName(),
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

			bindingName := names.GenerateBindingName(volcanoJobObj.GetKind(), volcanoJobObj.GetName())

			ginkgo.By("verify ResourceBinding components field", func() {
				var resourceBinding *workv1alpha2.ResourceBinding
				var err error

				// Wait for ResourceBinding to be created and components field to be populated
				gomega.Eventually(func(g gomega.Gomega) {
					resourceBinding, err = karmadaClient.WorkV1alpha2().ResourceBindings(volcanoJobNamespace).
						Get(context.Background(), bindingName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					g.Expect(resourceBinding).ShouldNot(gomega.BeNil())
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())

				// Ensure components field exists and has the expected number of components
				gomega.Expect(resourceBinding.Spec.Components).ShouldNot(gomega.BeNil())
				gomega.Expect(resourceBinding.Spec.Components).Should(gomega.HaveLen(2), "components should have exactly 2 items")
				// Find both components
				jobNginx1Component := findComponent(resourceBinding.Spec.Components, "job-nginx1")
				jobNginx2Component := findComponent(resourceBinding.Spec.Components, "job-nginx2")
				gomega.Expect(jobNginx1Component).ShouldNot(gomega.BeNil(), "job-nginx1 component should exist")
				gomega.Expect(jobNginx2Component).ShouldNot(gomega.BeNil(), "job-nginx2 component should exist")
				// Verify the detailed content of components (including ReplicaRequirements and resource values)
				verifyComponentResources(jobNginx1Component, 1, "200m", "100Mi")
				verifyComponentResources(jobNginx2Component, 2, "100m", "100Mi")
			})

			var targetClusterName string
			ginkgo.By("verify ResourceBinding scheduling result", func() {
				var resourceBinding *workv1alpha2.ResourceBinding
				var err error
				// Wait for ResourceBinding to be scheduled (check Scheduled condition)
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					resourceBinding, err = karmadaClient.WorkV1alpha2().ResourceBindings(volcanoJobNamespace).
						Get(context.Background(), bindingName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					return meta.IsStatusConditionTrue(resourceBinding.Status.Conditions, workv1alpha2.Scheduled), nil
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Equal(true))

				// Verify scheduling result: exactly one cluster
				gomega.Expect(resourceBinding.Spec.Clusters).Should(gomega.HaveLen(1))
				targetClusterName = resourceBinding.Spec.Clusters[0].Name
				gomega.Expect(targetClusterName).ShouldNot(gomega.BeEmpty())
			})

			ginkgo.By("wait for VolcanoJob to be created on the scheduled cluster", func() {
				// Verify VolcanoJob is propagated to the scheduled cluster
				clusterDynamicClient := framework.GetClusterDynamicClient(targetClusterName)
				gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

				gomega.Eventually(func(g gomega.Gomega) {
					_, err := clusterDynamicClient.Resource(volcanoJobGVR).Namespace(volcanoJobNamespace).
						Get(context.Background(), volcanoJobName, metav1.GetOptions{})
					g.Expect(err).ShouldNot(gomega.HaveOccurred(), "VolcanoJob should be present on cluster %s", targetClusterName)
				}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
			})
		})
	})

})
