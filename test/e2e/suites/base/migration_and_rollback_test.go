/*
Copyright 2023 The Karmada Authors.

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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	pkgutil "github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Seamless migration and rollback testing", func() {
	var member1 string
	var member1Client kubernetes.Interface

	ginkgo.BeforeEach(func() {
		member1 = framework.ClusterNames()[0]
		member1Client = framework.GetClusterClient(member1)
	})

	// referring to related manual steps: https://github.com/karmada-io/karmada/pull/3821#issuecomment-1649238940
	ginkgo.Context("Test migrate namespaced resource: Deployment", func() {
		var deployment *appsv1.Deployment
		var propagationPolicy *policyv1alpha1.PropagationPolicy
		var bindingName, workName, workNamespace string

		ginkgo.BeforeEach(func() {
			deployment = helper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			propagationPolicy = helper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
			})
			bindingName = names.GenerateBindingName(deployment.Kind, deployment.Name)
			workName = names.GenerateWorkName(deployment.Kind, deployment.Name, deployment.Namespace)
			workNamespace = names.GenerateExecutionSpaceName(member1)
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateDeployment(member1Client, deployment)
			// Create Deployment in karmada control plane
			framework.CreateDeployment(kubeClient, deployment)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)

			ginkgo.DeferCleanup(func() {
				// Delete Deployment in member cluster
				framework.RemoveDeployment(member1Client, deployment.Namespace, deployment.Name)
				// Delete PropagationPolicy in karmada control plane
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			})
		})

		ginkgo.It("Verify migrate a Deployment from member cluster", func() {
			// Step 1, Verify ResourceBinding got unHealthy for resource already exist
			ginkgo.By(fmt.Sprintf("Verify ResourceBinding %s got unApplied for resource already exist", bindingName), func() {
				klog.Infof("Waiting to verify ResourceBinding %s got unApplied for resource already exist", bindingName)
				gomega.Eventually(func() bool {
					binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(deployment.Namespace).Get(context.TODO(), bindingName, metav1.GetOptions{})
					if err != nil && apierrors.IsNotFound(err) {
						return false
					}
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					items := binding.Status.AggregatedStatus
					return len(items) > 0 && items[0].Applied == false && items[0].Health != workv1alpha2.ResourceHealthy
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			// Step 2, Update PropagationPolicy in karmada control plane with conflictResolution=Overwrite and preserveResourcesOnDeletion=true
			ginkgo.By(fmt.Sprintf("Update PropagationPolicy %s in karmada control plane with conflictResolution=Overwrite", propagationPolicy.Name), func() {
				propagationPolicy.Spec.ConflictResolution = policyv1alpha1.ConflictOverwrite
				propagationPolicy.Spec.PreserveResourcesOnDeletion = ptr.To[bool](true)
				framework.UpdatePropagationPolicyWithSpec(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name, propagationPolicy.Spec)
			})

			// Step 3, Verify Deployment Replicas all ready and ResourceBinding got Healthy for overwriting conflict resource
			ginkgo.By(fmt.Sprintf("Verify Deployment Replicas all ready and ResourceBinding %s got Healthy for overwriting conflict resource", bindingName), func() {
				klog.Infof("Waiting for Deployment ready on Karmada control plane")
				framework.WaitDeploymentStatus(kubeClient, deployment, *deployment.Spec.Replicas)

				binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(deployment.Namespace).Get(context.TODO(), bindingName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				items := binding.Status.AggregatedStatus
				gomega.Expect(len(items)).ShouldNot(gomega.Equal(0))
				gomega.Expect(items[0].Applied).Should(gomega.BeTrue())
				gomega.Expect(items[0].Health).Should(gomega.Equal(workv1alpha2.ResourceHealthy))
			})

			// Step 4, Delete resource template and check whether member cluster resource is preserved
			ginkgo.By("Delete resource template and check whether member cluster resource is preserved", func() {
				// Delete Deployment in karmada control plane
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)

				// Wait for work deleted
				framework.WaitForWorkToDisappear(karmadaClient, workNamespace, workName)

				// Check member cluster resource is preserved
				framework.WaitDeploymentPresentOnClusterFitWith(member1, deployment.Namespace, deployment.Name, isResourceNotManagedByKarmada)

			})
		})
	})

	ginkgo.Context("Test migrate cluster resources: ClusterRole", func() {
		var clusterRoleName string
		var clusterRole *rbacv1.ClusterRole
		var cpp *policyv1alpha1.ClusterPropagationPolicy
		var bindingName, workName, workNamespace string

		ginkgo.BeforeEach(func() {
			clusterRoleName = clusterRoleNamePrefix + rand.String(RandomStrLength)
			clusterRole = helper.NewClusterRole(clusterRoleName, []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"cluster.karmada.io"},
					Verbs:         []string{"*"},
					Resources:     []string{"clusters/proxy"},
					ResourceNames: []string{member1},
				},
			})
			cpp = helper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: clusterRole.APIVersion,
					Kind:       clusterRole.Kind,
					Name:       clusterRole.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
			})
			cpp.Spec.ConflictResolution = policyv1alpha1.ConflictOverwrite
			cpp.Spec.PreserveResourcesOnDeletion = ptr.To[bool](true)
			bindingName = names.GenerateBindingName(clusterRole.Kind, clusterRole.Name)
			workName = names.GenerateWorkName(clusterRole.Kind, clusterRole.Name, clusterRole.Namespace)
			workNamespace = names.GenerateExecutionSpaceName(member1)
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateClusterRole(member1Client, clusterRole)
			// Create Deployment in karmada control plane
			framework.CreateClusterRole(kubeClient, clusterRole)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreateClusterPropagationPolicy(karmadaClient, cpp)

			ginkgo.DeferCleanup(func() {
				// Delete ClusterRole in member cluster
				framework.RemoveClusterRole(member1Client, clusterRoleName)
				// Delete ClusterPropagationPolicy in karmada control plane
				framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
			})
		})

		ginkgo.It("Verify migrate a ClusterRole from member cluster", func() {
			ginkgo.By(fmt.Sprintf("Verify ClusterResourceBinding %s got Applied by overwriting conflict resource", bindingName), func() {
				klog.Infof("Waiting to verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName)
				gomega.Eventually(func() bool {
					framework.WaitClusterRolePresentOnClusterFitWith(member1, clusterRoleName, func(*rbacv1.ClusterRole) bool {
						return true
					})
					_, e1 := kubeClient.RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
					binding, e2 := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), bindingName, metav1.GetOptions{})

					return e1 == nil && e2 == nil && len(binding.Status.AggregatedStatus) > 0 && binding.Status.AggregatedStatus[0].Applied
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("Delete resource template and check whether member cluster resource is preserved", func() {
				// Delete ClusterRole in karmada control plane
				framework.RemoveClusterRole(kubeClient, clusterRole.Name)

				// Wait for work deleted
				framework.WaitForWorkToDisappear(karmadaClient, workNamespace, workName)

				// Check member cluster resource is preserved
				framework.WaitClusterRolePresentOnClusterFitWith(member1, clusterRole.Name, isResourceNotManagedByKarmada)

			})
		})
	})

	ginkgo.Context("Test migrate namespaced resource: Service (NodePort)", func() {
		var serviceName string
		var service *corev1.Service
		var pp *policyv1alpha1.PropagationPolicy
		var bindingName, workName, workNamespace string

		ginkgo.BeforeEach(func() {
			serviceName = serviceNamePrefix + rand.String(RandomStrLength)
			service = helper.NewService(testNamespace, serviceName, corev1.ServiceTypeNodePort)
			pp = helper.NewPropagationPolicy(testNamespace, service.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
					Name:       service.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
			})
			pp.Spec.ConflictResolution = policyv1alpha1.ConflictOverwrite
			pp.Spec.PreserveResourcesOnDeletion = ptr.To[bool](true)
			bindingName = names.GenerateBindingName(service.Kind, service.Name)
			workName = names.GenerateWorkName(service.Kind, service.Name, service.Namespace)
			workNamespace = names.GenerateExecutionSpaceName(member1)
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateService(member1Client, service)
			// Create Deployment in karmada control plane
			framework.CreateService(kubeClient, service)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreatePropagationPolicy(karmadaClient, pp)

			ginkgo.DeferCleanup(func() {
				// Delete Service in member cluster
				framework.RemoveService(member1Client, testNamespace, serviceName)
				// Delete PropagationPolicy in karmada control plane
				framework.RemovePropagationPolicy(karmadaClient, testNamespace, pp.Name)
			})
		})

		ginkgo.It("Verify migrate a Service from member cluster", func() {
			ginkgo.By(fmt.Sprintf("Verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName), func() {
				klog.Infof("Waiting to verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName)
				gomega.Eventually(func() bool {
					framework.WaitServicePresentOnClusterFitWith(member1, testNamespace, serviceName, func(*corev1.Service) bool {
						return true
					})
					_, e1 := kubeClient.CoreV1().Services(testNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
					binding, e2 := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), bindingName, metav1.GetOptions{})

					return e1 == nil && e2 == nil && len(binding.Status.AggregatedStatus) > 0 && binding.Status.AggregatedStatus[0].Applied
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("Delete resource template and check whether member cluster resource is preserved", func() {
				// Delete Service in karmada control plane
				framework.RemoveService(kubeClient, service.Namespace, service.Name)

				// Wait for work deleted
				framework.WaitForWorkToDisappear(karmadaClient, workNamespace, workName)

				// Check member cluster resource is preserved
				framework.WaitServicePresentOnClusterFitWith(member1, service.Namespace, service.Name, isResourceNotManagedByKarmada)
			})
		})
	})

	ginkgo.Context("Test migrate dependent resource", func() {
		var secret *corev1.Secret
		var volume []corev1.Volume
		var deployment *appsv1.Deployment
		var propagationPolicy *policyv1alpha1.PropagationPolicy
		var bindingName, workName, workNamespace string

		ginkgo.BeforeEach(func() {
			secret = helper.NewSecret(testNamespace, secretNamePrefix+rand.String(RandomStrLength), map[string][]byte{"test": []byte("test")})
			volume = []corev1.Volume{{
				Name: secret.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			}}
			deployment = helper.NewDeploymentWithVolumes(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength), volume)
			propagationPolicy = helper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{member1}},
			})
			propagationPolicy.Spec.PropagateDeps = true
			propagationPolicy.Spec.ConflictResolution = policyv1alpha1.ConflictOverwrite
			propagationPolicy.Spec.PreserveResourcesOnDeletion = ptr.To[bool](true)
			bindingName = names.GenerateBindingName(secret.Kind, secret.Name)
			workName = names.GenerateWorkName(secret.Kind, secret.Name, secret.Namespace)
			workNamespace = names.GenerateExecutionSpaceName(member1)
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateDeployment(member1Client, deployment)
			// Create Secret in member1 cluster
			framework.CreateSecret(member1Client, secret)
			// Create Deployment in karmada control plane
			framework.CreateDeployment(kubeClient, deployment)
			// Create Secret in karmada control plane
			framework.CreateSecret(kubeClient, secret)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)

			ginkgo.DeferCleanup(func() {
				// Delete Deployment in control plane and member cluster
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemoveDeployment(member1Client, deployment.Namespace, deployment.Name)
				// Delete Secret in member cluster
				framework.RemoveSecret(member1Client, secret.Namespace, secret.Name)
				// Delete PropagationPolicy in karmada control plane
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
			})
		})

		ginkgo.It("Verify migrate a dependent secret from member cluster", func() {

			ginkgo.By(fmt.Sprintf("Verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName), func() {
				klog.Infof("Waiting to verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName)
				gomega.Eventually(func() bool {
					framework.WaitSecretPresentOnClusterFitWith(member1, testNamespace, secret.Name, func(*corev1.Secret) bool {
						return true
					})
					_, e1 := kubeClient.CoreV1().Secrets(testNamespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
					binding, e2 := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), bindingName, metav1.GetOptions{})

					return e1 == nil && e2 == nil && len(binding.Status.AggregatedStatus) > 0 && binding.Status.AggregatedStatus[0].Applied
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			ginkgo.By("Delete dependent secret template and check whether member cluster secret is preserved", func() {
				// Delete dependent secret in karmada control plane
				framework.RemoveSecret(kubeClient, secret.Namespace, secret.Name)

				// Wait for work deleted
				framework.WaitForWorkToDisappear(karmadaClient, workNamespace, workName)

				// Check member cluster secret is preserved
				framework.WaitSecretPresentOnClusterFitWith(member1, secret.Namespace, secret.Name, isResourceNotManagedByKarmada)
			})
		})
	})
})

// isResourceNotManagedByKarmada checks if resource is missing all karmada managed labels/annotations
// which indicates that it's not managed by Karmada.
func isResourceNotManagedByKarmada[T metav1.Object](obj T) bool {
	for _, key := range pkgutil.ManagedResourceLabels {
		if _, exist := obj.GetLabels()[key]; exist {
			return false
		}
	}

	for _, key := range pkgutil.ManagedResourceAnnotations {
		if _, exist := obj.GetAnnotations()[key]; exist {
			return false
		}
	}

	return true
}
