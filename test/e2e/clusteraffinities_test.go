package e2e

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	controllercluster "github.com/karmada-io/karmada/pkg/controllers/cluster"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[ClusterAffinities] propagation testing", func() {
	ginkgo.When("[PropagationPolicy] replica scheduling type is Duplicated", func() {
		ginkgo.Context("schedule with multi clusterAffinity", func() {
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy
			var member1LabelKey, member2LabelKey string

			ginkgo.BeforeEach(func() {
				member1LabelKey = "member1-" + rand.String(RandomStrLength)
				member2LabelKey = "member2-" + rand.String(RandomStrLength)

				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member1LabelKey: "ok"}}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member2LabelKey: "ok"}}},
						},
						{
							AffinityName:    "group3",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"no-exist-cluster": "ok"}}},
						},
						{
							AffinityName:    "group4",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						},
					}})
			})

			ginkgo.BeforeEach(func() {
				framework.UpdateClusterLabels(karmadaClient, "member1", map[string]string{member1LabelKey: "ok"})
				framework.UpdateClusterLabels(karmadaClient, "member2", map[string]string{member2LabelKey: "ok"})
				ginkgo.DeferCleanup(func() {
					framework.DeleteClusterLabels(karmadaClient, "member1", map[string]string{member1LabelKey: ""})
					framework.DeleteClusterLabels(karmadaClient, "member2", map[string]string{member2LabelKey: ""})
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("propagate deployment and then update the cluster label", func() {
				// 1. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member1", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })

				// 2. update member1 cluster label to make it's unmatched with the policy
				framework.UpdateClusterLabels(karmadaClient, "member1", map[string]string{member1LabelKey: "not-ok"})
				framework.WaitDeploymentDisappearOnCluster("member1", deployment.Namespace, deployment.Name)

				// 3. wait for deployment present on member2 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member2", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })

				// 4. update member2 cluster label to make it's unmatched with the policy
				framework.UpdateClusterLabels(karmadaClient, "member2", map[string]string{member2LabelKey: "not-ok"})
				framework.WaitDeploymentDisappearOnCluster("member2", deployment.Namespace, deployment.Name)

				// 5. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member1", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinity to clusterAffinities", func() {
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy

			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("propagate deployment and then update the cluster label", func() {
				// 1. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member1", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = nil
				policy.Spec.Placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{
					AffinityName:    "group1",
					ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}},
				}}
				framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)

				// 3. wait for deployment present on member2 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member2", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinities to clusterAffinity", func() {
			var deployment *appsv1.Deployment
			var policy *policyv1alpha1.PropagationPolicy

			ginkgo.BeforeEach(func() {
				deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
				policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: deployment.APIVersion,
						Kind:       deployment.Kind,
						Name:       deployment.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
					}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePropagationPolicy(karmadaClient, policy)
				framework.CreateDeployment(kubeClient, deployment)
				ginkgo.DeferCleanup(func() {
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				})
			})

			ginkgo.It("propagate deployment and then update the cluster label", func() {
				// 1. wait for deployment present on member1 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member1", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}
				policy.Spec.Placement.ClusterAffinities = nil
				framework.UpdatePropagationPolicyWithSpec(karmadaClient, policy.Namespace, policy.Name, policy.Spec)

				// 3. wait for deployment present on member2 cluster
				framework.WaitDeploymentPresentOnClusterFitWith("member2", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })
			})
		})
	})

	ginkgo.When("[ClusterPropagationPolicy] replica scheduling type is Duplicated", func() {
		ginkgo.Context("schedule with multi clusterAffinity", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole
			var member1LabelKey, member2LabelKey string

			ginkgo.BeforeEach(func() {
				member1LabelKey = "member1-" + rand.String(RandomStrLength)
				member2LabelKey = "member2-" + rand.String(RandomStrLength)

				clusterRole = testhelper.NewClusterRole(clusterRoleNamePrefix+rand.String(RandomStrLength), nil)
				policy = testhelper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member1LabelKey: "ok"}}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{member2LabelKey: "ok"}}},
						},
						{
							AffinityName:    "group3",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"no-exist-cluster": "ok"}}},
						},
						{
							AffinityName:    "group4",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
						},
					}})
			})

			ginkgo.BeforeEach(func() {
				framework.UpdateClusterLabels(karmadaClient, "member1", map[string]string{member1LabelKey: "ok"})
				framework.UpdateClusterLabels(karmadaClient, "member2", map[string]string{member2LabelKey: "ok"})
				ginkgo.DeferCleanup(func() {
					framework.DeleteClusterLabels(karmadaClient, "member1", map[string]string{member1LabelKey: ""})
					framework.DeleteClusterLabels(karmadaClient, "member2", map[string]string{member2LabelKey: ""})
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})
			})

			ginkgo.It("propagate clusterRole and then update the cluster label", func() {
				// 1. wait for clusterRole present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member1", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })

				// 2. update member1 cluster label to make it's unmatched with the policy
				framework.UpdateClusterLabels(karmadaClient, "member1", map[string]string{member1LabelKey: "not-ok"})
				framework.WaitClusterRoleDisappearOnCluster("member1", clusterRole.Name)

				// 3. wait for clusterRole present on member2 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member2", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })

				// 4. update member2 cluster label to make it's unmatched with the policy
				framework.UpdateClusterLabels(karmadaClient, "member2", map[string]string{member2LabelKey: "not-ok"})
				framework.WaitClusterRoleDisappearOnCluster("member2", clusterRole.Name)

				// 5. wait for deployment present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member1", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinity to clusterAffinities", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole

			ginkgo.BeforeEach(func() {
				clusterRole = testhelper.NewClusterRole(clusterRoleNamePrefix+rand.String(RandomStrLength), nil)
				policy = testhelper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})
			})

			ginkgo.It("propagate clusterRole and then update the cluster label", func() {
				// 1. wait for clusterRole present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member1", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = nil
				policy.Spec.Placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{
					AffinityName:    "group1",
					ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}},
				}}
				framework.UpdateClusterPropagationPolicyWithSpec(karmadaClient, policy.Name, policy.Spec)

				// 3. wait for clusterRole present on member2 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member2", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })
			})
		})

		ginkgo.Context("schedule change from clusterAffinities to clusterAffinity", func() {
			var policy *policyv1alpha1.ClusterPropagationPolicy
			var clusterRole *rbacv1.ClusterRole

			ginkgo.BeforeEach(func() {
				clusterRole = testhelper.NewClusterRole(clusterRoleNamePrefix+rand.String(RandomStrLength), nil)
				policy = testhelper.NewClusterPropagationPolicy(clusterRole.Name, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: clusterRole.APIVersion,
						Kind:       clusterRole.Kind,
						Name:       clusterRole.Name,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
					}},
				})
			})

			ginkgo.BeforeEach(func() {
				framework.CreateClusterPropagationPolicy(karmadaClient, policy)
				framework.CreateClusterRole(kubeClient, clusterRole)
				ginkgo.DeferCleanup(func() {
					framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
					framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				})
			})

			ginkgo.It("propagate clusterRole and then update the cluster label", func() {
				// 1. wait for clusterRole present on member1 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member1", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })

				// 2. update policy placement with clusterAffinities
				policy.Spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}}
				policy.Spec.Placement.ClusterAffinities = nil
				framework.UpdateClusterPropagationPolicyWithSpec(karmadaClient, policy.Name, policy.Spec)

				// 3. wait for clusterRole present on member2 cluster
				framework.WaitClusterRolePresentOnClusterFitWith("member2", clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool { return true })
			})
		})
	})

	framework.SerialWhen("[Failover] member cluster become unReachable", func() {
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))

			policy = testhelper.NewPropagationPolicy(deployment.Namespace, deployment.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterTolerations: []corev1.Toleration{
					*helper.NewNotReadyToleration(2),
					*helper.NewUnreachableToleration(2),
				},
				ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
					{
						AffinityName:    "group1",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}},
					},
					{
						AffinityName:    "group2",
						ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member2"}},
					}}})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.It("deployment failover testing", func() {
			// 1. set cluster member1 condition status to false
			ginkgo.By("set cluster member1 condition status to false", func() {
				err := disableCluster(controlPlaneClient, "member1")
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				framework.WaitClusterFitWith(controlPlaneClient, "member1", func(cluster *clusterv1alpha1.Cluster) bool {
					return helper.TaintExists(cluster.Spec.Taints, controllercluster.NotReadyTaintTemplate)
				})
			})

			// 2. wait for deployment present on member2 cluster
			framework.WaitDeploymentPresentOnClusterFitWith("member2", deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool { return true })

			// 3. recover not ready cluster
			ginkgo.By("recover not ready cluster", func() {
				originalAPIEndpoint := getClusterAPIEndpoint("member1")
				err := recoverCluster(controlPlaneClient, "member1", originalAPIEndpoint)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentCluster, err := util.GetCluster(controlPlaneClient, "member1")
					if err != nil {
						return false, err
					}
					if !helper.TaintExists(currentCluster.Spec.Taints, controllercluster.NotReadyTaintTemplate) {
						fmt.Printf("cluster %s recovered\n", "member1")
						return true, nil
					}
					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})

	})
})
