package e2e

import (
	"fmt"
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = framework.SerialDescribe("unjoin pull mode cluster unknown status testing", ginkgo.Labels{NeedCreateCluster}, func() {
	ginkgo.Context("joining pull mode cluster and unjoining unknown status cluster", func() {
		var err error
		var homeDir string

		var clusterName string
		var kubeConfigPath string
		var clusterContext string
		var controlPlane string
		var memberEndPoint string
		var restConfig *rest.Config
		var kubeClient kubernetes.Interface

		var replicasNum int32
		var namespace = "karmada-system"
		var serviceAccountName = "karmada-agent-sa"
		var karmadaAgentName = "karmada-agent"
		var karmadaKubeconfigName = "karmada-kubeconfig"
		var karmadaContext = "karmada-context"

		// prepare for create cluster
		ginkgo.BeforeEach(func() {
			clusterName = "member-e2e-" + rand.String(3)
			homeDir = os.Getenv("HOME")
			kubeConfigPath = fmt.Sprintf("%s/.kube/%s.config", homeDir, clusterName)
			clusterContext = fmt.Sprintf("kind-%s", clusterName)
			controlPlane = fmt.Sprintf("%s-control-plane", clusterName)
			replicasNum = 1
		})

		// create member cluster
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("Create cluster: %s", clusterName), func() {
				err := createCluster(clusterName, kubeConfigPath, controlPlane, clusterContext)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			ginkgo.DeferCleanup(func() {
				ginkgo.By(fmt.Sprintf("Deleting clusters: %s", clusterName), func() {
					err := deleteCluster(clusterName, kubeConfigPath)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					_ = os.Remove(kubeConfigPath)
				})
			})
			restConfig, err = framework.LoadRESTClientConfig(kubeConfigPath, clusterContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			kubeClient, err = kubernetes.NewForConfig(restConfig)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			memberEndPoint = strings.Replace(restConfig.Host, "https://", "", -1)
		})

		// create namespace in member cluster
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("create namespace in member cluster: %s", clusterName), func() {
				ns := helper.NewNamespace(namespace)
				framework.CreateNamespace(kubeClient, ns)
			})
		})

		// create serviceaccount in member cluster
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("create serviceaccount in member cluster: %s", clusterName), func() {
				sa := helper.NewServiceaccount(namespace, serviceAccountName)
				framework.CreateServiceAccount(kubeClient, sa)
			})
		})

		// create clusterrole in member cluster
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("create clusterrole in member cluster: %s", clusterName), func() {
				crl := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: karmadaAgentName,
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"*"},
							Resources: []string{"*"},
							Verbs:     []string{"*"},
						},
						{
							NonResourceURLs: []string{"*"},
							Verbs:           []string{"get"},
						},
					},
				}
				framework.CreateClusterRole(kubeClient, crl)
			})
		})

		// create clusterrolebinding in member cluster
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("create clusterrolebinding in member cluster: %s", clusterName), func() {
				crb := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: karmadaAgentName,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     karmadaAgentName,
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      serviceAccountName,
							Namespace: namespace,
						},
					},
				}
				framework.CreateClusterRoleBinding(kubeClient, crb)
			})
		})

		// create karmada-agent in member cluster
		ginkgo.BeforeEach(func() {
			ginkgo.By(fmt.Sprintf("create karmada-agent in member cluster: %s", clusterName), func() {
				var karmadaAgentLabels = map[string]string{"app": karmadaAgentName}
				agent := &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      karmadaAgentName,
						Namespace: namespace,
						Labels:    karmadaAgentLabels,
					},
				}
				// podSpec
				podSpec := corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{
						{
							Name:  karmadaAgentName,
							Image: "docker.io/karmada/karmada-agent:latest",
							Command: []string{
								"/bin/karmada-agent",
								"--karmada-kubeconfig=/etc/kubeconfig/karmada-kubeconfig",
								fmt.Sprintf("--karmada-context=%s", karmadaContext),
								fmt.Sprintf("--cluster-name=%s", clusterName),
								fmt.Sprintf("--cluster-api-endpoint=%s", memberEndPoint),
								"--cluster-status-update-frequency=10s",
								"--bind-address=0.0.0.0",
								"--secure-port=10357",
								"--v=4",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kubeconfig",
									MountPath: "/etc/kubeconfig",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: karmadaKubeconfigName,
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
						},
					},
				}
				// PodTemplateSpec
				podTemplateSpec := corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      karmadaAgentName,
						Namespace: namespace,
						Labels:    karmadaAgentLabels,
					},
					Spec: podSpec,
				}
				// DeploymentSpec
				agent.Spec = appsv1.DeploymentSpec{
					Replicas: &replicasNum,
					Template: podTemplateSpec,
					Selector: &metav1.LabelSelector{
						MatchLabels: karmadaAgentLabels,
					},
				}
				// create karmada-agent deployment
				framework.CreateDeployment(kubeClient, agent)
				// wait for deployment present on member cluster
				framework.WaitDeploymentPresentOnClusterFitWith(clusterName, agent.Namespace, agent.Name, func(deployment *appsv1.Deployment) bool { return true })
			})
		})

		ginkgo.It("Test delete unknown status cluster", func() {
			ginkgo.By("Checking cluster status collection", func() {
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					cluster, err := framework.FetchCluster(karmadaClient, clusterName)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					if cluster.Status.KubernetesVersion == "" {
						return false, nil
					}
					if len(cluster.Status.APIEnablements) == 0 {
						return false, nil
					}
					if len(cluster.Status.Conditions) == 0 {
						return false, nil
					}
					if cluster.Status.NodeSummary == nil {
						return false, nil
					}
					if cluster.Status.ResourceSummary == nil || len(cluster.Status.ResourceSummary.AllocatableModelings) == 0 {
						return false, nil
					}
					return true, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			// delete karmada-agent deployment
			ginkgo.By(fmt.Sprintf("Delete agent deployment in cluster: %s", clusterName), func() {
				framework.RemoveDeployment(kubeClient, namespace, karmadaAgentName)
				framework.WaitClusterFitWith(controlPlaneClient, clusterName, func(cluster *clusterv1alpha1.Cluster) bool {
					return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)
				})
			})

			// delete cluster
			ginkgo.By(fmt.Sprintf("Unjoinning cluster: %s", clusterName), func() {
				err := deleteCluster(clusterName, kubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})
})
