/*
Copyright 2020 The Karmada Authors.

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
	"encoding/json"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

// BasicPropagation focus on basic propagation functionality testing.
var _ = ginkgo.Describe("[BasicCase] PropagationPolicy testing", func() {
	var policyNamespace, policyName string
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		ginkgo.DeferCleanup(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("Deployment propagation testing", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment propagation testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool {
					return true
				})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == updateDeploymentReplicas
				})
		})

		ginkgo.It("adds dispatching event with a dispatching message", func() {
			workName := names.GenerateWorkName(deployment.Kind, deployment.Name, deployment.Namespace)
			esName := names.GenerateExecutionSpaceName(framework.ClusterNames()[0])
			framework.WaitEventFitWith(kubeClient, esName, workName, func(event corev1.Event) bool {
				return event.Reason == events.EventReasonWorkDispatching &&
					event.Message == execution.WorkDispatchingConditionMessage
			})
		})
	})

	ginkgo.Context("Service propagation testing", func() {
		var serviceNamespace, serviceName string
		var service *corev1.Service

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = serviceNamePrefix + rand.String(RandomStrLength)
			serviceNamespace = policyNamespace
			serviceName = policyName

			service = testhelper.NewService(serviceNamespace, serviceName, corev1.ServiceTypeClusterIP)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
					Name:       service.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateService(kubeClient, service)
			ginkgo.DeferCleanup(func() {
				framework.RemoveService(kubeClient, service.Namespace, service.Name)
				framework.WaitServiceDisappearOnClusters(framework.ClusterNames(), service.Namespace, service.Name)
			})
		})

		ginkgo.It("service propagation testing", func() {
			framework.WaitServicePresentOnClustersFitWith(framework.ClusterNames(), service.Namespace, service.Name,
				func(*corev1.Service) bool {
					return true
				})

			patch := []map[string]interface{}{{"op": "replace", "path": "/spec/ports/0/port", "value": updateServicePort}}
			framework.UpdateServiceWithPatch(kubeClient, service.Namespace, service.Name, patch, types.JSONPatchType)

			framework.WaitServicePresentOnClustersFitWith(framework.ClusterNames(), service.Namespace, service.Name,
				func(service *corev1.Service) bool {
					return service.Spec.Ports[0].Port == updateServicePort
				})
		})
	})

	ginkgo.Context("Pod propagation testing", func() {
		var podNamespace, podName string
		var pod *corev1.Pod
		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = podNamePrefix + rand.String(RandomStrLength)
			podNamespace = policyNamespace
			podName = policyName

			pod = testhelper.NewPod(podNamespace, podName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: pod.APIVersion,
					Kind:       pod.Kind,
					Name:       pod.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePod(kubeClient, pod)
			ginkgo.DeferCleanup(func() {
				framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
				framework.WaitPodDisappearOnClusters(framework.ClusterNames(), pod.Namespace, pod.Name)
			})
		})

		ginkgo.It("pod propagation testing", func() {
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name,
				func(*corev1.Pod) bool {
					return true
				})

			patch := []map[string]interface{}{{"op": "replace", "path": "/spec/containers/0/image", "value": updatePodImage}}
			framework.UpdatePodWithPatch(kubeClient, pod.Namespace, pod.Name, patch, types.JSONPatchType)

			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name,
				func(pod *corev1.Pod) bool {
					return pod.Spec.Containers[0].Image == updatePodImage
				})
		})
	})

	ginkgo.Context("NamespaceScoped CustomResource propagation testing", func() {
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition
		var crdPolicy *policyv1alpha1.ClusterPropagationPolicy
		var crNamespace, crName string
		var crGVR schema.GroupVersionResource
		var crAPIVersion string
		var cr *unstructured.Unstructured

		ginkgo.BeforeEach(func() {
			crdGroup = fmt.Sprintf("example-%s.karmada.io", rand.String(RandomStrLength))
			randStr = rand.String(RandomStrLength)
			crdSpecNames = apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     fmt.Sprintf("Foo%s", randStr),
				ListKind: fmt.Sprintf("Foo%sList", randStr),
				Plural:   fmt.Sprintf("foo%ss", randStr),
				Singular: fmt.Sprintf("foo%s", randStr),
			}
			crd = testhelper.NewCustomResourceDefinition(crdGroup, crdSpecNames, apiextensionsv1.NamespaceScoped)
			crdPolicy = testhelper.NewClusterPropagationPolicy(crd.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: crd.APIVersion,
					Kind:       crd.Kind,
					Name:       crd.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})

			crNamespace = testNamespace
			crName = crdNamePrefix + rand.String(RandomStrLength)
			crGVR = schema.GroupVersionResource{Group: crd.Spec.Group, Version: "v1alpha1", Resource: crd.Spec.Names.Plural}

			crAPIVersion = fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1")
			cr = testhelper.NewCustomResource(crAPIVersion, crd.Spec.Names.Kind, crNamespace, crName)
			policy = testhelper.NewPropagationPolicy(crNamespace, crName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: crAPIVersion,
					Kind:       crd.Spec.Names.Kind,
					Name:       crName,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
				fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1"), crd.Spec.Names.Kind)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
				framework.RemoveCRD(dynamicClient, crd.Name)
			})
		})

		ginkgo.It("namespaceScoped cr propagation testing", func() {
			ginkgo.By(fmt.Sprintf("creating cr(%s/%s)", crNamespace, crName), func() {
				_, err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Create(context.TODO(), cr, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if cr present on member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for cr(%s/%s) present on cluster(%s)", crNamespace, crName, cluster.Name)
					err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crGVR).Namespace(crNamespace).Get(ctx, crName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return false, nil
							}
							return false, err
						}
						return true, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By("updating cr", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/resource/namespace",
						"value": updateCRnamespace,
					},
				}
				bytes, err := json.Marshal(patch)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				_, err = dynamicClient.Resource(crGVR).Namespace(crNamespace).Patch(context.TODO(), crName, types.JSONPatchType, bytes, metav1.PatchOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if update has been synced to member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for cr(%s/%s) synced on cluster(%s)", crNamespace, crName, cluster.Name)
					err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
						cr, err := clusterDynamicClient.Resource(crGVR).Namespace(crNamespace).Get(ctx, crName, metav1.GetOptions{})
						if err != nil {
							return false, err
						}

						namespace, found, err := unstructured.NestedString(cr.Object, "spec", "resource", "namespace")
						if err != nil || !found {
							return false, err
						}
						if namespace == updateCRnamespace {
							return true, nil
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})

			ginkgo.By(fmt.Sprintf("removing cr(%s/%s)", crNamespace, crName), func() {
				err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Delete(context.TODO(), crName, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if cr has been deleted from member clusters", func() {
				for _, cluster := range framework.Clusters() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster.Name)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					klog.Infof("Waiting for cr(%s/%s) disappear on cluster(%s)", crNamespace, crName, cluster.Name)
					err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
						_, err = clusterDynamicClient.Resource(crGVR).Namespace(crNamespace).Get(ctx, crName, metav1.GetOptions{})
						if err != nil {
							if apierrors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						}

						return false, nil
					})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
	})

	ginkgo.Context("Job propagation testing", func() {
		var jobNamespace, jobName string
		var job *batchv1.Job

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = jobNamePrefix + rand.String(RandomStrLength)
			jobNamespace = testNamespace
			jobName = policyName

			job = testhelper.NewJob(jobNamespace, jobName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: job.APIVersion,
					Kind:       job.Kind,
					Name:       job.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateJob(kubeClient, job)
			ginkgo.DeferCleanup(func() {
				framework.RemoveJob(kubeClient, job.Namespace, job.Name)
				framework.WaitJobDisappearOnClusters(framework.ClusterNames(), job.Namespace, job.Name)
			})
		})

		ginkgo.It("job propagation testing", func() {
			framework.WaitJobPresentOnClustersFitWith(framework.ClusterNames(), job.Namespace, job.Name,
				func(*batchv1.Job) bool {
					return true
				})

			patch := []map[string]interface{}{{"op": "replace", "path": "/spec/backoffLimit", "value": ptr.To[int32](updateBackoffLimit)}}
			bytes, err := json.Marshal(patch)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.UpdateJobWithPatchBytes(kubeClient, job.Namespace, job.Name, bytes, types.JSONPatchType)

			framework.WaitJobPresentOnClustersFitWith(framework.ClusterNames(), job.Namespace, job.Name,
				func(job *batchv1.Job) bool {
					return *job.Spec.BackoffLimit == updateBackoffLimit
				})
		})
	})

	ginkgo.Context("Role propagation testing", func() {
		var (
			roleNamespace, roleName string
			role                    *rbacv1.Role
		)

		ginkgo.BeforeEach(func() {
			roleNamespace = testNamespace
			policyName = roleNamePrefix + rand.String(RandomStrLength)
			roleName = fmt.Sprintf("system:test:%s", policyName)

			role = testhelper.NewRole(roleNamespace, roleName, nil)
			policy = testhelper.NewPropagationPolicy(roleNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: role.APIVersion,
					Kind:       role.Kind,
					Name:       role.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateRole(kubeClient, role)
			ginkgo.DeferCleanup(func() {
				framework.RemoveRole(kubeClient, role.Namespace, role.Name)
				framework.WaitRoleDisappearOnClusters(framework.ClusterNames(), role.Namespace, role.Name)
			})
		})

		ginkgo.It("role propagation testing", func() {
			framework.WaitRolePresentOnClustersFitWith(framework.ClusterNames(), role.Namespace, role.Name,
				func(*rbacv1.Role) bool {
					return true
				})
		})
	})

	ginkgo.Context("RoleBinding propagation testing", func() {
		var (
			roleBindingNamespace, roleBindingName string
			roleBinding                           *rbacv1.RoleBinding
		)

		ginkgo.BeforeEach(func() {
			roleBindingNamespace = testNamespace
			policyName = roleBindingNamePrefix + rand.String(RandomStrLength)
			roleBindingName = fmt.Sprintf("system:test.%s", policyName)

			roleBinding = testhelper.NewRoleBinding(roleBindingNamespace, roleBindingName, roleBindingName, nil)
			policy = testhelper.NewPropagationPolicy(roleBindingNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: roleBinding.APIVersion,
					Kind:       roleBinding.Kind,
					Name:       roleBinding.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateRoleBinding(kubeClient, roleBinding)
			ginkgo.DeferCleanup(func() {
				framework.RemoveRoleBinding(kubeClient, roleBinding.Namespace, roleBinding.Name)
				framework.WaitRoleBindingDisappearOnClusters(framework.ClusterNames(), roleBinding.Namespace, roleBinding.Name)
			})
		})

		ginkgo.It("roleBinding propagation testing", func() {
			framework.WaitRoleBindingPresentOnClustersFitWith(framework.ClusterNames(), roleBinding.Namespace, roleBinding.Name,
				func(*rbacv1.RoleBinding) bool {
					return true
				})
		})
	})
})

var _ = ginkgo.Describe("[CornerCase] PropagationPolicy testing", func() {
	var policyNamespace, policyName string
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		ginkgo.DeferCleanup(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("Propagate Deployment with long pp name (exceed 63 character)", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			policyName = deploymentNamePrefix + "-longname-longname-longname-longname-longname-longname-" + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = policyName

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment propagation testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool {
					return true
				})

			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})
})

// ImplicitPriority more than one PP matches the object, we should choose the most suitable one.
// Set it to run sequentially to avoid affecting other test cases.
var _ = framework.SerialDescribe("[ImplicitPriority] PropagationPolicy testing", func() {
	ginkgo.Context("priorityMatchName propagation testing", func() {
		var policyNamespace, priorityMatchName, priorityMatchLabelSelector, priorityMatchAll string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policyMatchName, policyMatchLabelSelector, policyPriorityMatchAll *policyv1alpha1.PropagationPolicy
		var implicitPriorityLabelKey = "priority"
		var implicitPriorityLabelValue = "implicit-priority"

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			priorityMatchName = deploymentNamePrefix + rand.String(RandomStrLength)
			priorityMatchLabelSelector = deploymentNamePrefix + rand.String(RandomStrLength)
			priorityMatchAll = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{implicitPriorityLabelKey: implicitPriorityLabelValue})
			policyMatchName = testhelper.NewPropagationPolicy(policyNamespace, priorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
			policyMatchLabelSelector = testhelper.NewPropagationPolicy(policyNamespace, priorityMatchLabelSelector, []policyv1alpha1.ResourceSelector{
				{
					APIVersion:    deployment.APIVersion,
					Kind:          deployment.Kind,
					LabelSelector: metav1.SetAsLabelSelector(labels.Set{implicitPriorityLabelKey: implicitPriorityLabelValue}),
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
			policyPriorityMatchAll = testhelper.NewPropagationPolicy(policyNamespace, priorityMatchAll, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policyMatchName)
			framework.CreatePropagationPolicy(karmadaClient, policyMatchLabelSelector)
			framework.CreatePropagationPolicy(karmadaClient, policyPriorityMatchAll)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("priorityMatchName/priorityMatchLabel/priorityMatchAll testing", func() {
			ginkgo.By("check whether the deployment uses the highest priority propagationPolicy (priorityMatchName)", func() {
				defer framework.RemovePropagationPolicy(karmadaClient, policyMatchLabelSelector.Namespace, policyMatchName.Name)
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation] == priorityMatchName
					})
			})
			ginkgo.By("check whether the deployment uses the highest priority propagationPolicy (priorityMatchLabel)", func() {
				defer framework.RemovePropagationPolicy(karmadaClient, policyMatchLabelSelector.Namespace, policyMatchLabelSelector.Name)
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation] == priorityMatchLabelSelector
					})
			})
			ginkgo.By("check whether the deployment uses the highest priority propagationPolicy (priorityMatchAll)", func() {
				defer framework.RemovePropagationPolicy(karmadaClient, policyMatchLabelSelector.Namespace, policyPriorityMatchAll.Name)
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation] == priorityMatchAll
					})
			})
		})
	})
})

// ExplicitPriority more than one PP matches the object, we should select the one with the highest explicit priority, if the
// explicit priority is same, select the one with the highest implicit priority.
var _ = ginkgo.Describe("[ExplicitPriority] PropagationPolicy testing", func() {
	ginkgo.Context("high explicit/low priority/implicit priority PropagationPolicy propagation testing", func() {
		var policyNamespace, higherPriorityLabelSelector, lowerPriorityMatchName, implicitPriorityMatchName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policyHigherPriorityLabelSelector, policyLowerPriorityMatchMatchName, policyImplicitPriorityMatchMatchName *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			higherPriorityLabelSelector = deploymentNamePrefix + "higherprioritylabelselector" + rand.String(RandomStrLength)
			lowerPriorityMatchName = deploymentNamePrefix + "lowerprioritymatchame" + rand.String(RandomStrLength)
			implicitPriorityMatchName = deploymentNamePrefix + "implicitprioritymatchname" + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			var priorityLabelKey = "priority"
			var priorityLabelValue = "priority" + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{priorityLabelKey: priorityLabelValue})
			policyHigherPriorityLabelSelector = testhelper.NewExplicitPriorityPropagationPolicy(policyNamespace, higherPriorityLabelSelector, []policyv1alpha1.ResourceSelector{
				{
					APIVersion:    deployment.APIVersion,
					Kind:          deployment.Kind,
					LabelSelector: metav1.SetAsLabelSelector(labels.Set{priorityLabelKey: priorityLabelValue}),
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 2)
			policyLowerPriorityMatchMatchName = testhelper.NewExplicitPriorityPropagationPolicy(policyNamespace, lowerPriorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 1)
			policyImplicitPriorityMatchMatchName = testhelper.NewPropagationPolicy(policyNamespace, implicitPriorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policyHigherPriorityLabelSelector)
			framework.CreatePropagationPolicy(karmadaClient, policyLowerPriorityMatchMatchName)
			framework.CreatePropagationPolicy(karmadaClient, policyImplicitPriorityMatchMatchName)

			// Wait till the informer's cache is synced in karmada-controller.
			// Note: We tested and find that it takes about 1s before cache synced.
			time.Sleep(time.Second * 5)

			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policyHigherPriorityLabelSelector.Namespace, policyHigherPriorityLabelSelector.Name)
				framework.RemovePropagationPolicy(karmadaClient, policyLowerPriorityMatchMatchName.Namespace, policyLowerPriorityMatchMatchName.Name)
			})
		})

		ginkgo.It("high explicit/low priority/implicit priority PropagationPolicy propagation testing", func() {
			ginkgo.By("check whether the deployment uses the highest explicit priority PropagationPolicy", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						klog.Infof("Match PropagationPolicy:%s/%s", deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNamespaceAnnotation],
							deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation])
						return deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation] == higherPriorityLabelSelector
					})
			})
		})
	})

	ginkgo.Context("same explicit priority PropagationPolicy propagation testing", func() {
		var policyNamespace, explicitPriorityLabelSelector, explicitPriorityMatchName string
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment
		var policyExplicitPriorityLabelSelector, policyExplicitPriorityMatchName *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			policyNamespace = testNamespace
			explicitPriorityLabelSelector = deploymentNamePrefix + "explicitprioritylabelselector" + rand.String(RandomStrLength)
			explicitPriorityMatchName = deploymentNamePrefix + "explicitprioritymatchname" + rand.String(RandomStrLength)
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			var priorityLabelKey = "priority"
			var priorityLabelValue = "priority" + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{priorityLabelKey: priorityLabelValue})
			policyExplicitPriorityLabelSelector = testhelper.NewExplicitPriorityPropagationPolicy(policyNamespace, explicitPriorityLabelSelector, []policyv1alpha1.ResourceSelector{
				{
					APIVersion:    deployment.APIVersion,
					Kind:          deployment.Kind,
					LabelSelector: metav1.SetAsLabelSelector(labels.Set{priorityLabelKey: priorityLabelValue}),
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 1)
			policyExplicitPriorityMatchName = testhelper.NewExplicitPriorityPropagationPolicy(policyNamespace, explicitPriorityMatchName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			}, 1)
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policyExplicitPriorityLabelSelector)
			framework.CreatePropagationPolicy(karmadaClient, policyExplicitPriorityMatchName)

			// Wait till the informer's cache is synced in karmada-controller.
			// Note: We tested and find that it takes about 1s before cache synced.
			time.Sleep(time.Second * 5)

			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.WaitDeploymentDisappearOnClusters(framework.ClusterNames(), deployment.Namespace, deployment.Name)
			})
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policyExplicitPriorityLabelSelector.Namespace, policyExplicitPriorityLabelSelector.Name)
				framework.RemovePropagationPolicy(karmadaClient, policyExplicitPriorityMatchName.Namespace, policyExplicitPriorityMatchName.Name)
			})
		})

		ginkgo.It("same explicit priority PropagationPolicy propagation testing", func() {
			ginkgo.By("check whether the deployment uses the PropagationPolicy with name matched", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						klog.Infof("Match PropagationPolicy:%s/%s", deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNamespaceAnnotation],
							deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation])
						return deployment.GetAnnotations()[policyv1alpha1.PropagationPolicyNameAnnotation] == explicitPriorityMatchName
					})
			})
		})
	})
})

// AdvancedPropagation focus on some advanced propagation testing.
var _ = ginkgo.Describe("[AdvancedCase] PropagationPolicy testing", func() {
	ginkgo.Context("Edit PropagationPolicy ResourceSelectors", func() {
		var policy *policyv1alpha1.PropagationPolicy
		var deployment01, deployment02 *appsv1.Deployment
		var targetMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			policyNamespace := testNamespace
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment01 = testhelper.NewDeployment(testNamespace, policyName+"01")
			deployment02 = testhelper.NewDeployment(testNamespace, policyName+"02")

			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment01.APIVersion,
					Kind:       deployment01.Kind,
					Name:       deployment01.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment01)
			framework.CreateDeployment(kubeClient, deployment02)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemoveDeployment(kubeClient, deployment01.Namespace, deployment01.Name)
				framework.RemoveDeployment(kubeClient, deployment02.Namespace, deployment02.Name)
			})

			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment01.Namespace, deployment01.Name,
				func(*appsv1.Deployment) bool { return true })
		})

		ginkgo.It("add resourceSelectors item", func() {
			policy.Spec.ResourceSelectors = []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment01.APIVersion,
					Kind:       deployment01.Kind,
					Name:       deployment01.Name,
				},
				{
					APIVersion: deployment02.APIVersion,
					Kind:       deployment02.Kind,
					Name:       deployment02.Name,
				},
			}
			framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)

			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment02.Namespace, deployment02.Name,
				func(*appsv1.Deployment) bool { return true })
		})

		ginkgo.It("update resourceSelectors item", func() {
			policySpec := policyv1alpha1.PropagationSpec{
				ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment02.APIVersion,
						Kind:       deployment02.Kind,
						Name:       deployment02.Name,
					},
				},
			}
			framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policySpec)

			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment02.Namespace, deployment02.Name,
				func(*appsv1.Deployment) bool { return true })
			framework.WaitDeploymentGetByClientFitWith(kubeClient, deployment01.Namespace, deployment01.Name,
				func(deployment *appsv1.Deployment) bool {
					if deployment.Annotations == nil {
						return true
					}

					_, namespaceExist := deployment.Annotations[policyv1alpha1.PropagationPolicyNamespaceAnnotation]
					_, nameExist := deployment.Annotations[policyv1alpha1.PropagationPolicyNameAnnotation]
					if namespaceExist || nameExist {
						return false
					}
					return true
				})
		})
	})

	ginkgo.Context("Edit PropagationPolicy fields other than resourceSelectors", func() {
		var policy *policyv1alpha1.PropagationPolicy
		var deployment *appsv1.Deployment
		var targetMember, updatedMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			updatedMember = framework.ClusterNames()[1]
			policyNamespace := testNamespace
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(testNamespace, policyName+"01")

			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})

			gomega.Eventually(func() bool {
				observedPolicy, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policy.Namespace).Get(context.TODO(), policy.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(labels.Set{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.PropagationPolicyPermanentIDLabel],
					}).String(),
				})
				if err != nil {
					return false
				}
				return len(bindings.Items) != 0
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.It("update policy propagateDeps", func() {
			patch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/propagateDeps",
					"value": true,
				},
			}
			framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policy.Name, patch, types.JSONPatchType)
			gomega.Eventually(func() bool {
				observedPolicy, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policy.Namespace).Get(context.TODO(), policy.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(labels.Set{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.PropagationPolicyPermanentIDLabel],
					}).String(),
				})
				if err != nil {
					return false
				}
				return bindings.Items[0].Spec.PropagateDeps == true
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.It("update policy placement", func() {
			updatedPlacement := policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{updatedMember},
				}}
			patch := []map[string]interface{}{
				{
					"op":    "replace",
					"path":  "/spec/placement",
					"value": updatedPlacement,
				},
			}
			framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policy.Name, patch, types.JSONPatchType)
			framework.WaitDeploymentDisappearOnCluster(targetMember, deployment.Namespace, deployment.Name)
			framework.WaitDeploymentPresentOnClusterFitWith(updatedMember, deployment.Namespace, deployment.Name,
				func(*appsv1.Deployment) bool { return true })
		})
	})

	ginkgo.Context("Delete the propagationPolicy", func() {
		var policy *policyv1alpha1.PropagationPolicy
		var deployment *appsv1.Deployment
		var targetMember string

		ginkgo.BeforeEach(func() {
			targetMember = framework.ClusterNames()[0]
			policyNamespace := testNamespace
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)

			deployment = testhelper.NewDeployment(testNamespace, policyName+"01")

			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetMember},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				// PropagationPolicy will be removed in subsequent test cases
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})

			gomega.Eventually(func() bool {
				observedPolicy, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(policy.Namespace).Get(context.TODO(), policy.Name, metav1.GetOptions{})
				if err != nil {
					return false
				}
				bindings, err := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(labels.Set{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: observedPolicy.Labels[policyv1alpha1.PropagationPolicyPermanentIDLabel],
					}).String(),
				})
				if err != nil {
					return false
				}
				return len(bindings.Items) != 0
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.It("delete the propagationPolicy and check whether labels and annotations are deleted correctly", func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.WaitDeploymentFitWith(kubeClient, deployment.Namespace, deployment.Name, func(dep *appsv1.Deployment) bool {
				if dep.Labels != nil && dep.Labels[policyv1alpha1.PropagationPolicyPermanentIDLabel] != "" {
					return false
				}
				if dep.Annotations != nil && dep.Annotations[policyv1alpha1.PropagationPolicyNamespaceAnnotation] != "" && dep.Annotations[policyv1alpha1.PropagationPolicyNameAnnotation] != "" {
					return false
				}
				return true
			})

			resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
			framework.WaitResourceBindingFitWith(karmadaClient, deployment.Namespace, resourceBindingName, func(resourceBinding *workv1alpha2.ResourceBinding) bool {
				if resourceBinding.Labels != nil && resourceBinding.Labels[policyv1alpha1.PropagationPolicyPermanentIDLabel] != "" {
					return false
				}
				if resourceBinding.Annotations != nil && resourceBinding.Annotations[policyv1alpha1.PropagationPolicyNamespaceAnnotation] != "" && resourceBinding.Annotations[policyv1alpha1.PropagationPolicyNameAnnotation] != "" {
					return false
				}
				return true
			})
		})
	})

	ginkgo.Context("Unbind the old PropagationPolicy and create a new one", func() {
		var policy01, policy02 *policyv1alpha1.PropagationPolicy
		var configmap *corev1.ConfigMap
		var member1, member2 string

		ginkgo.BeforeEach(func() {
			member1 = framework.ClusterNames()[0]
			member2 = framework.ClusterNames()[1]
			policyNamespace := testNamespace
			policyName := configMapNamePrefix + rand.String(RandomStrLength)

			configmap = testhelper.NewConfigMap(testNamespace, policyName, map[string]string{"a": "b"})
			configmap.ObjectMeta.Labels = map[string]string{"a": "b"}

			policy01 = testhelper.NewPropagationPolicy(policyNamespace, policyName+"01", []policyv1alpha1.ResourceSelector{
				{
					APIVersion: configmap.APIVersion,
					Kind:       configmap.Kind,
					Name:       configmap.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{member1},
				},
			})
			policy02 = testhelper.NewPropagationPolicy(policyNamespace, policyName+"02", []policyv1alpha1.ResourceSelector{
				{
					APIVersion: configmap.APIVersion,
					Kind:       configmap.Kind,
					Name:       configmap.Name,
				}}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{member2},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreateConfigMap(kubeClient, configmap)
			ginkgo.DeferCleanup(func() {
				framework.RemoveConfigMap(kubeClient, configmap.Namespace, configmap.Name)
				framework.WaitConfigMapDisappearOnClusters(framework.ClusterNames(), configmap.Namespace, configmap.Name)
			})
		})

		ginkgo.It("modify the old propagationPolicy to unbind and create a new one", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy01)
			framework.WaitConfigMapPresentOnClusterFitWith(member1, configmap.Namespace, configmap.Name,
				func(*corev1.ConfigMap) bool { return true })
			framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy01.Namespace, policy01.Name, policyv1alpha1.PropagationSpec{
				ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: configmap.APIVersion,
						Kind:       configmap.Kind,
						Name:       configmap.Name + "fake",
					},
				},
				Placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{member1},
					},
				},
			})

			framework.CreatePropagationPolicy(karmadaClient, policy02)
			framework.WaitConfigMapDisappearOnCluster(member1, configmap.Namespace, configmap.Name)
			framework.WaitConfigMapPresentOnClusterFitWith(member2, configmap.Namespace, configmap.Name,
				func(*corev1.ConfigMap) bool { return true })
			framework.RemovePropagationPolicy(karmadaClient, policy01.Namespace, policy01.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy02.Namespace, policy02.Name)
		})

		ginkgo.It("delete the old propagationPolicy to unbind and create a new one", func() {
			framework.CreatePropagationPolicy(karmadaClient, policy01)
			framework.WaitConfigMapPresentOnClusterFitWith(member1, configmap.Namespace, configmap.Name,
				func(*corev1.ConfigMap) bool { return true })
			framework.RemovePropagationPolicy(karmadaClient, policy01.Namespace, policy01.Name)

			framework.CreatePropagationPolicy(karmadaClient, policy02)
			framework.WaitConfigMapDisappearOnCluster(member1, configmap.Namespace, configmap.Name)
			framework.WaitConfigMapPresentOnClusterFitWith(member2, configmap.Namespace, configmap.Name,
				func(*corev1.ConfigMap) bool { return true })
			framework.RemovePropagationPolicy(karmadaClient, policy02.Namespace, policy02.Name)
		})
	})
})

var _ = ginkgo.Describe("[Suspension] PropagationPolicy testing", func() {
	var policy *policyv1alpha1.PropagationPolicy
	var deployment *appsv1.Deployment
	var targetMember string

	ginkgo.BeforeEach(func() {
		targetMember = framework.ClusterNames()[0]
		policyNamespace := testNamespace
		policyName := deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = testhelper.NewDeployment(testNamespace, policyName+"01")
		policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			}}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetMember},
			},
		})
	})

	ginkgo.BeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
			func(*appsv1.Deployment) bool {
				return true
			})
		ginkgo.DeferCleanup(func() {
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		})
	})

	ginkgo.It("suspend the PP dispatching", func() {
		ginkgo.By("update the pp suspension dispatching to true", func() {
			policy.Spec.Suspension = &policyv1alpha1.Suspension{
				Dispatching: ptr.To(true),
			}
			framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)
		})

		ginkgo.By("check RB suspension spec", func() {
			framework.WaitResourceBindingFitWith(karmadaClient, deployment.Namespace, names.GenerateBindingName(deployment.Kind, deployment.Name),
				func(binding *workv1alpha2.ResourceBinding) bool {
					return binding.Spec.Suspension != nil && ptr.Deref(binding.Spec.Suspension.Dispatching, false)
				})
		})

		ginkgo.By("check Work suspension spec", func() {
			workName := names.GenerateWorkName(deployment.Kind, deployment.Name, deployment.Namespace)
			esName := names.GenerateExecutionSpaceName(targetMember)
			gomega.Eventually(func() bool {
				work, err := karmadaClient.WorkV1alpha1().Works(esName).Get(context.TODO(), workName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return work != nil && util.IsWorkSuspendDispatching(work)
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.By("check Work Dispatching status condition", func() {
			workName := names.GenerateWorkName(deployment.Kind, deployment.Name, deployment.Namespace)
			esName := names.GenerateExecutionSpaceName(targetMember)
			gomega.Eventually(func() bool {
				work, err := karmadaClient.WorkV1alpha1().Works(esName).Get(context.TODO(), workName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return work != nil && meta.IsStatusConditionPresentAndEqual(work.Status.Conditions, workv1alpha1.WorkDispatching, metav1.ConditionFalse)
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.By("check dispatching event", func() {
			workName := names.GenerateWorkName(deployment.Kind, deployment.Name, deployment.Namespace)
			esName := names.GenerateExecutionSpaceName(targetMember)
			framework.WaitEventFitWith(kubeClient, esName, workName,
				func(event corev1.Event) bool {
					return event.Reason == events.EventReasonWorkDispatching &&
						event.Message == execution.WorkSuspendDispatchingConditionMessage
				})
		})
	})

	ginkgo.It("suspension resume", func() {
		ginkgo.By("update deployment replicas", func() {
			framework.UpdateDeploymentReplicas(kubeClient, deployment, updateDeploymentReplicas)
		})

		ginkgo.By("resume the propagationPolicy", func() {
			policy.Spec.Suspension = &policyv1alpha1.Suspension{}
			framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)
		})

		ginkgo.By("check deployment replicas", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetMember, deployment.Namespace, deployment.Name,
				func(d *appsv1.Deployment) bool {
					return *d.Spec.Replicas == updateDeploymentReplicas
				})
		})
	})
})
