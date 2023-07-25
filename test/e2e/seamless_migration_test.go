package e2e

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

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Seamless migration testing", func() {
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
		var bindingName string

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
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateDeployment(member1Client, deployment)
			// Create Deployment in karmada control plane
			framework.CreateDeployment(kubeClient, deployment)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)

			ginkgo.DeferCleanup(func() {
				// Delete Deployment in karmada control plane
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				// Delete PropagationPolicy in karmada control plane
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)

				// Verify Deployment in member cluster will be deleted automatically after promotion since it has been deleted from Karmada
				klog.Infof("Waiting for Deployment deleted from cluster(%s)", member1)
				framework.WaitDeploymentDisappearOnCluster(member1, testNamespace, deployment.Name)
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

			// Step 2, Update PropagationPolicy in karmada control plane with conflictResolution=Overwrite
			ginkgo.By(fmt.Sprintf("Update PropagationPolicy %s in karmada control plane with conflictResolution=Overwrite", propagationPolicy.Name), func() {
				propagationPolicy.Spec.ConflictResolution = policyv1alpha1.ConflictOverwrite
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
		})
	})

	ginkgo.Context("Test migrate cluster resources: ClusterRole", func() {
		var clusterRoleName string
		var clusterRole *rbacv1.ClusterRole
		var cpp *policyv1alpha1.ClusterPropagationPolicy
		var bindingName string

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
			bindingName = names.GenerateBindingName(clusterRole.Kind, clusterRole.Name)
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateClusterRole(member1Client, clusterRole)
			// Create Deployment in karmada control plane
			framework.CreateClusterRole(kubeClient, clusterRole)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreateClusterPropagationPolicy(karmadaClient, cpp)

			ginkgo.DeferCleanup(func() {
				// Delete ClusterRole in karmada control plane
				framework.RemoveClusterRole(kubeClient, clusterRoleName)
				// Delete ClusterPropagationPolicy in karmada control plane
				framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)

				// Verify ClusterRole in member cluster will be deleted automatically after promotion since it has been deleted from Karmada
				klog.Infof("Waiting for ClusterRole deleted from cluster(%s)", member1)
				framework.WaitClusterRoleDisappearOnCluster(member1, clusterRoleName)
			})
		})

		ginkgo.It("Verify migrate a ClusterRole from member cluster", func() {
			ginkgo.By(fmt.Sprintf("Verify ClusterResourceBinding %s got Applied by overwriting conflict resource", bindingName), func() {
				klog.Infof("Waiting to verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName)
				gomega.Eventually(func() bool {
					framework.WaitClusterRolePresentOnClusterFitWith(member1, clusterRoleName, func(clusterRole *rbacv1.ClusterRole) bool {
						return true
					})
					_, e1 := kubeClient.RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
					binding, e2 := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), bindingName, metav1.GetOptions{})

					return e1 == nil && e2 == nil && len(binding.Status.AggregatedStatus) > 0 && binding.Status.AggregatedStatus[0].Applied
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("Test migrate namespaced resource: Service (NodePort)", func() {
		var serviceName string
		var service *corev1.Service
		var pp *policyv1alpha1.PropagationPolicy
		var bindingName string

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
			bindingName = names.GenerateBindingName(service.Kind, service.Name)
		})

		ginkgo.BeforeEach(func() {
			// Create Deployment in member1 cluster
			framework.CreateService(member1Client, service)
			// Create Deployment in karmada control plane
			framework.CreateService(kubeClient, service)
			// Create PropagationPolicy in karmada control plane without conflictResolution field
			framework.CreatePropagationPolicy(karmadaClient, pp)

			ginkgo.DeferCleanup(func() {
				// Delete Service in karmada control plane
				framework.RemoveService(kubeClient, testNamespace, serviceName)
				// Delete PropagationPolicy in karmada control plane
				framework.RemovePropagationPolicy(karmadaClient, testNamespace, pp.Name)

				// Verify Service in member cluster will be deleted automatically after promotion since it has been deleted from Karmada
				klog.Infof("Waiting for Service deleted from cluster(%s)", member1)
				framework.WaitServiceDisappearOnCluster(member1, testNamespace, serviceName)
			})
		})

		ginkgo.It("Verify migrate a Service from member cluster", func() {
			ginkgo.By(fmt.Sprintf("Verify PropagationPolicy %s got Applied by overwriting conflict resource", bindingName), func() {
				klog.Infof("Waiting to verify ResourceBinding %s got Applied by overwriting conflict resource", bindingName)
				gomega.Eventually(func() bool {
					framework.WaitServicePresentOnClusterFitWith(member1, testNamespace, serviceName, func(service *corev1.Service) bool {
						return true
					})
					_, e1 := kubeClient.CoreV1().Services(testNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
					binding, e2 := karmadaClient.WorkV1alpha2().ResourceBindings(testNamespace).Get(context.TODO(), bindingName, metav1.GetOptions{})

					return e1 == nil && e2 == nil && len(binding.Status.AggregatedStatus) > 0 && binding.Status.AggregatedStatus[0].Applied
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})
