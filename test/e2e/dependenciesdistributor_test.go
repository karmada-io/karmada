package e2e

import (
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[DependenciesDistributor] automatically propagate relevant resources testing", func() {
	ginkgo.Context("dependencies propagation testing", func() {
		initClusterNames := []string{"member1"}
		updateClusterNames := []string{"member2"}

		secretName := secretNamePrefix + rand.String(RandomStrLength)
		configMapName := configMapNamePrefix + rand.String(RandomStrLength)
		secret := testhelper.NewSecret(testNamespace, secretName, map[string][]byte{"user": []byte("karmada")})
		configMap := testhelper.NewConfigMap(testNamespace, configMapName, map[string]string{"user": "karmada"})

		ginkgo.It("configmap automatically propagation testing", func() {
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentName := policyName

			deployment := testhelper.NewDeploymentReferencesConfigMap(testNamespace, deploymentName, configMapName)
			policy := testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: initClusterNames,
				},
			})

			policy.Spec.PropagateDeps = true

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreateConfigMap(kubeClient, configMap)

			ginkgo.By("check if the configmap is propagated automatically", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return true
					})

				framework.WaitConfigMapPresentOnClustersFitWith(initClusterNames, configMap.Namespace, configMapName,
					func(configmap *corev1.ConfigMap) bool {
						return true
					})
			})

			ginkgo.By("updating propagation policy's clusterNames", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/spec/placement/clusterAffinity/clusterNames",
						"value": updateClusterNames,
					},
				}

				framework.PatchPropagationPolicy(karmadaClient, policy.Namespace, policyName, patch, types.JSONPatchType)
				framework.WaitDeploymentPresentOnClustersFitWith(updateClusterNames, deployment.Namespace, deploymentName,
					func(deployment *appsv1.Deployment) bool {
						return true
					})

				framework.WaitConfigMapPresentOnClustersFitWith(updateClusterNames, configMap.Namespace, configMapName,
					func(configmap *corev1.ConfigMap) bool {
						return true
					})
			})

			ginkgo.By("updating configmap's data", func() {
				patch := []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/data/user",
						"value": "karmada-e2e",
					},
				}

				framework.UpdateConfigMapWithPatch(kubeClient, configMap.Namespace, configMapName, patch, types.JSONPatchType)
				framework.WaitConfigMapPresentOnClustersFitWith(updateClusterNames, configMap.Namespace, configMapName,
					func(configmap *corev1.ConfigMap) bool {
						for key, value := range configmap.Data {
							if key == "user" && value == "karmada-e2e" {
								return true
							}
						}
						return false
					})
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMapName)
		})

		ginkgo.It("secret automatically propagation testing", func() {
			policyName := deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentName := policyName

			deployment := testhelper.NewDeploymentReferencesSecret(testNamespace, deploymentName, secretName)
			policy := testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: initClusterNames,
				},
			})

			policy.Spec.PropagateDeps = true

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreateSecret(kubeClient, secret)

			ginkgo.By("check if the secret is propagated automatically", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return true
					})

				framework.WaitSecretPresentOnClustersFitWith(initClusterNames, secret.Namespace, secretName,
					func(secret *corev1.Secret) bool {
						return true
					})
			})

			ginkgo.By("make the secret is not referenced by the deployment ", func() {
				updateVolumes := []corev1.Volume{
					{
						Name: "vol-configmap",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMapName,
								},
							},
						},
					},
				}

				framework.UpdateDeploymentVolumes(kubeClient, deployment, updateVolumes)
				framework.WaitSecretDisappearOnClusters(initClusterNames, secret.Namespace, secretName)
			})

			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.RemoveSecret(kubeClient, secret.Namespace, secretName)
		})
	})
})
