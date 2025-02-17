/*
Copyright 2022 The Karmada Authors.

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
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[DependenciesDistributor] automatically propagate relevant resources testing", func() {
	ginkgo.Context("dependencies propagation with propagationPolicy testing", func() {
		var initClusterNames, updateClusterNames []string
		var policyName string
		var deploymentName string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			initClusterNames = []string{framework.ClusterNames()[0]}
			updateClusterNames = []string{framework.ClusterNames()[1]}

			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentName = policyName
		})

		ginkgo.JustBeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreatePropagationPolicy(karmadaClient, policy)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})
		})

		ginkgo.When("configmap propagate automatically", func() {
			var configMapName string
			var configMap *corev1.ConfigMap

			ginkgo.BeforeEach(func() {
				configMapName = configMapNamePrefix + rand.String(RandomStrLength)
				configMap = testhelper.NewConfigMap(testNamespace, configMapName, map[string]string{"user": "karmada"})

				volumes := []corev1.Volume{{
					Name: "vol-configmap",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							}}}}}
				deployment = testhelper.NewDeploymentWithVolumes(testNamespace, deploymentName, volumes)

				policy = testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			})

			ginkgo.BeforeEach(func() {
				framework.CreateConfigMap(kubeClient, configMap)
				ginkgo.DeferCleanup(func() {
					framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMapName)
				})
			})

			ginkgo.It("configmap automatically propagation testing", func() {
				ginkgo.By("check if the configmap is propagated automatically", func() {
					framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitConfigMapPresentOnClustersFitWith(initClusterNames, configMap.Namespace, configMapName,
						func(*corev1.ConfigMap) bool {
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
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitConfigMapPresentOnClustersFitWith(updateClusterNames, configMap.Namespace, configMapName,
						func(*corev1.ConfigMap) bool {
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
			})
		})

		ginkgo.When("secret propagate automatically", func() {
			var secretName string
			var secret *corev1.Secret

			ginkgo.BeforeEach(func() {
				secretName = secretNamePrefix + rand.String(RandomStrLength)
				secret = testhelper.NewSecret(testNamespace, secretName, map[string][]byte{"user": []byte("karmada")})

				volumes := []corev1.Volume{{
					Name: "vol-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secretName,
						}}}}
				deployment = testhelper.NewDeploymentWithVolumes(testNamespace, deploymentName, volumes)

				policy = testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			})

			ginkgo.BeforeEach(func() {
				framework.CreateSecret(kubeClient, secret)
				ginkgo.DeferCleanup(func() {
					framework.RemoveSecret(kubeClient, secret.Namespace, secretName)
				})
			})

			ginkgo.It("secret automatically propagation testing", func() {
				ginkgo.By("check if the secret is propagated automatically", func() {
					framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitSecretPresentOnClustersFitWith(initClusterNames, secret.Namespace, secretName,
						func(*corev1.Secret) bool {
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
										Name: "configMap-test",
									},
								},
							},
						},
					}

					framework.UpdateDeploymentVolumes(kubeClient, deployment, updateVolumes)
					framework.WaitSecretDisappearOnClusters(initClusterNames, secret.Namespace, secretName)
				})
			})
		})

		ginkgo.When("persistentVolumeClaim propagate automatically", func() {
			var pvcName string
			var pvc *corev1.PersistentVolumeClaim

			ginkgo.BeforeEach(func() {
				pvcName = pvcNamePrefix + rand.String(RandomStrLength)
				pvc = testhelper.NewPVC(testNamespace, pvcName, corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				}, corev1.ReadWriteOnce)

				volumes := []corev1.Volume{{
					Name: "vol-pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.GetName(),
							ReadOnly:  false,
						}}}}
				deployment = testhelper.NewDeploymentWithVolumes(testNamespace, deploymentName, volumes)

				policy = testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			})

			ginkgo.BeforeEach(func() {
				framework.CreatePVC(kubeClient, pvc)
				ginkgo.DeferCleanup(func() {
					framework.RemovePVC(kubeClient, pvc.GetNamespace(), pvc.GetName())
				})
			})

			ginkgo.It("persistentVolumeClaim automatically propagation testing", func() {
				ginkgo.By("check if the persistentVolumeClaim is propagated automatically", func() {
					framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitPVCPresentOnClustersFitWith(initClusterNames, pvc.GetNamespace(), pvc.GetName(),
						func(*corev1.PersistentVolumeClaim) bool {
							return true
						})
				})

				ginkgo.By("make the persistentVolumeClaim is not referenced by the deployment ", func() {
					updateVolumes := []corev1.Volume{
						{
							Name: "vol-configmap",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "configMap-test",
									},
								},
							},
						},
					}

					framework.UpdateDeploymentVolumes(kubeClient, deployment, updateVolumes)
					framework.WaitPVCDisappearOnClusters(initClusterNames, pvc.GetNamespace(), pvc.GetName())
				})
			})
		})

		ginkgo.When("serviceAccount propagate automatically", func() {
			var saName string
			var sa *corev1.ServiceAccount

			ginkgo.BeforeEach(func() {
				saName = saNamePrefix + rand.String(RandomStrLength)
				sa = testhelper.NewServiceaccount(testNamespace, saName)

				deployment = testhelper.NewDeploymentWithServiceAccount(testNamespace, deploymentName, saName)

				policy = testhelper.NewPropagationPolicy(testNamespace, policyName, []policyv1alpha1.ResourceSelector{
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
			})

			ginkgo.It("serviceAccount automatically propagation testing", func() {
				framework.CreateServiceAccount(kubeClient, sa)
				ginkgo.DeferCleanup(func() {
					framework.RemoveServiceAccount(kubeClient, sa.GetNamespace(), sa.GetName())
				})
				ginkgo.By("check if the serviceAccount is propagated automatically", func() {
					framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitServiceAccountPresentOnClustersFitWith(initClusterNames, sa.GetNamespace(), sa.GetName(),
						func(*corev1.ServiceAccount) bool {
							return true
						})
				})

				ginkgo.By("make the sa is not referenced by the deployment ", func() {
					framework.UpdateDeploymentServiceAccountName(kubeClient, deployment, "default")
					framework.WaitServiceAccountDisappearOnClusters(initClusterNames, sa.GetNamespace(), sa.GetName())
				})
			})
		})
	})

	ginkgo.Context("dependencies propagation with clusterPropagationPolicy testing", func() {
		var initClusterNames, updateClusterNames []string
		var policyName string
		var deploymentName string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.ClusterPropagationPolicy

		ginkgo.BeforeEach(func() {
			initClusterNames = []string{framework.ClusterNames()[0]}
			updateClusterNames = []string{framework.ClusterNames()[1]}

			policyName = deploymentNamePrefix + rand.String(RandomStrLength)
			deploymentName = policyName
		})

		ginkgo.JustBeforeEach(func() {
			framework.CreateDeployment(kubeClient, deployment)
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
			})
		})

		ginkgo.When("configmap propagate automatically", func() {
			var configMapName string
			var configMap *corev1.ConfigMap

			ginkgo.BeforeEach(func() {
				configMapName = configMapNamePrefix + rand.String(RandomStrLength)
				configMap = testhelper.NewConfigMap(testNamespace, configMapName, map[string]string{"user": "karmada"})

				volumes := []corev1.Volume{{
					Name: "vol-configmap",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							}}}}}
				deployment = testhelper.NewDeploymentWithVolumes(testNamespace, deploymentName, volumes)

				policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
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
			})

			ginkgo.BeforeEach(func() {
				framework.CreateConfigMap(kubeClient, configMap)
				ginkgo.DeferCleanup(func() {
					framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMapName)
				})
			})

			ginkgo.It("configmap automatically propagation testing", func() {
				ginkgo.By("check if the configmap is propagated automatically", func() {
					framework.WaitDeploymentPresentOnClustersFitWith(initClusterNames, deployment.Namespace, deployment.Name,
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitConfigMapPresentOnClustersFitWith(initClusterNames, configMap.Namespace, configMapName,
						func(*corev1.ConfigMap) bool {
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

					framework.PatchClusterPropagationPolicy(karmadaClient, policyName, patch, types.JSONPatchType)
					framework.WaitDeploymentPresentOnClustersFitWith(updateClusterNames, deployment.Namespace, deploymentName,
						func(*appsv1.Deployment) bool {
							return true
						})

					framework.WaitConfigMapPresentOnClustersFitWith(updateClusterNames, configMap.Namespace, configMapName,
						func(*corev1.ConfigMap) bool {
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
			})
		})
	})
})
