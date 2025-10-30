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
	"context"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util/names"
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

	ginkgo.Context("dependency policy conflict detected and resolved testing", func() {
		var initClusterNames []string
		var deploymentAName, deploymentBName string
		var policyAName, policyBName string
		var deploymentA, deploymentB *appsv1.Deployment
		var policyA, policyB *policyv1alpha1.PropagationPolicy
		var configMapName string
		var configMap *corev1.ConfigMap
		var attachedBindingName, independentBindingAName, independentBindingBName string

		// Wait until the independent ResourceBinding reflects desired CR/Preserve values
		waitRBUpdated := func(rbName string, wantCR policyv1alpha1.ConflictResolution, wantPreserve *bool) {
			framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, rbName, func(rb *workv1alpha2.ResourceBinding) bool {
				if wantCR != "" && rb.Spec.ConflictResolution != wantCR {
					return false
				}
				if wantPreserve != nil {
					return rb.Spec.PreserveResourcesOnDeletion != nil && *rb.Spec.PreserveResourcesOnDeletion == *wantPreserve
				}
				return true
			})
		}

		ginkgo.BeforeEach(func() {
			initClusterNames = []string{framework.ClusterNames()[0]}
			configMapName = configMapNamePrefix + rand.String(RandomStrLength)
			configMap = testhelper.NewConfigMap(testNamespace, configMapName, map[string]string{"user": "karmada"})

			deploymentAName = deploymentNamePrefix + "-a-" + rand.String(RandomStrLength)
			deploymentBName = deploymentNamePrefix + "-b-" + rand.String(RandomStrLength)
			policyAName = deploymentAName
			policyBName = deploymentBName

			volumes := func(name string) []corev1.Volume {
				return []corev1.Volume{{
					Name: "vol-configmap",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: name},
						},
					},
				}}
			}

			deploymentA = testhelper.NewDeploymentWithVolumes(testNamespace, deploymentAName, volumes(configMapName))
			deploymentB = testhelper.NewDeploymentWithVolumes(testNamespace, deploymentBName, volumes(configMapName))

			policyA = testhelper.NewPropagationPolicy(testNamespace, policyAName, []policyv1alpha1.ResourceSelector{{
				APIVersion: deploymentA.APIVersion,
				Kind:       deploymentA.Kind,
				Name:       deploymentA.Name,
			}}, policyv1alpha1.Placement{ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: initClusterNames}})
			policyA.Spec.PropagateDeps = true

			policyB = testhelper.NewPropagationPolicy(testNamespace, policyBName, []policyv1alpha1.ResourceSelector{{
				APIVersion: deploymentB.APIVersion,
				Kind:       deploymentB.Kind,
				Name:       deploymentB.Name,
			}}, policyv1alpha1.Placement{ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: initClusterNames}})
			policyB.Spec.PropagateDeps = true

			attachedBindingName = names.GenerateBindingName("ConfigMap", configMapName)
			independentBindingAName = names.GenerateBindingName("Deployment", deploymentAName)
			independentBindingBName = names.GenerateBindingName("Deployment", deploymentBName)
		})

		ginkgo.JustBeforeEach(func() {
			// Create shared dependent resource and both independent resources and policies
			framework.CreateConfigMap(kubeClient, configMap)
			framework.CreateDeployment(kubeClient, deploymentA)
			framework.CreateDeployment(kubeClient, deploymentB)
			framework.CreatePropagationPolicy(karmadaClient, policyA)
			framework.CreatePropagationPolicy(karmadaClient, policyB)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicyIfExist(karmadaClient, policyA.Namespace, policyA.Name)
				framework.RemovePropagationPolicyIfExist(karmadaClient, policyB.Namespace, policyB.Name)
				// Delete deployments but ignore NotFound if already removed by the test
				if err := kubeClient.AppsV1().Deployments(deploymentA.Namespace).Delete(context.TODO(), deploymentA.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
				if err := kubeClient.AppsV1().Deployments(deploymentB.Namespace).Delete(context.TODO(), deploymentB.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
				framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMap.Name)
			})

			// Ensure attached and independent bindings exist
			framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, independentBindingAName, func(*workv1alpha2.ResourceBinding) bool { return true })
			framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, independentBindingBName, func(*workv1alpha2.ResourceBinding) bool { return true })
			framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedBindingName, func(*workv1alpha2.ResourceBinding) bool { return true })
		})

		ginkgo.It("conflict on both fields", func() {
			ginkgo.By("set policy A Overwrite+true and policy B Abort+false", func() {
				framework.PatchPropagationPolicy(karmadaClient, policyA.Namespace, policyA.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": true},
				}, types.JSONPatchType)
				framework.PatchPropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictAbort)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": false},
				}, types.JSONPatchType)

				waitRBUpdated(independentBindingAName, policyv1alpha1.ConflictOverwrite, func() *bool { b := true; return &b }())
				waitRBUpdated(independentBindingBName, policyv1alpha1.ConflictAbort, func() *bool { b := false; return &b }())
			})

			ginkgo.By("event reports both conflict hints", func() {
				framework.WaitEventFitWith(kubeClient, testNamespace, attachedBindingName, func(e corev1.Event) bool {
					return e.Reason == events.EventReasonDependencyPolicyConflict &&
						strings.Contains(e.Message, "ConflictResolution conflicted (Overwrite vs Abort)") &&
						strings.Contains(e.Message, "PreserveResourcesOnDeletion conflicted (true vs false)")
				})
			})

			ginkgo.By("resolved policy becomes Overwrite+true", func() {
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedBindingName, func(rb *workv1alpha2.ResourceBinding) bool {
					if rb.Spec.ConflictResolution != policyv1alpha1.ConflictOverwrite {
						return false
					}
					return rb.Spec.PreserveResourcesOnDeletion != nil && *rb.Spec.PreserveResourcesOnDeletion
				})
			})
		})

		ginkgo.It("conflict on conflictResolution only", func() {
			ginkgo.By("set policy A Overwrite+false and policy B Abort+false", func() {
				framework.PatchPropagationPolicy(karmadaClient, policyA.Namespace, policyA.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": false},
				}, types.JSONPatchType)
				framework.PatchPropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictAbort)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": false},
				}, types.JSONPatchType)

				waitRBUpdated(independentBindingAName, policyv1alpha1.ConflictOverwrite, func() *bool { b := false; return &b }())
				waitRBUpdated(independentBindingBName, policyv1alpha1.ConflictAbort, func() *bool { b := false; return &b }())
			})

			ginkgo.By("event only reports CR conflict", func() {
				framework.WaitEventFitWith(kubeClient, testNamespace, attachedBindingName, func(e corev1.Event) bool {
					return e.Reason == events.EventReasonDependencyPolicyConflict &&
						strings.Contains(e.Message, "ConflictResolution conflicted (Overwrite vs Abort)") &&
						!strings.Contains(e.Message, "PreserveResourcesOnDeletion conflicted (true vs false)")
				})
			})

			ginkgo.By("resolved policy becomes Overwrite+false", func() {
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedBindingName, func(rb *workv1alpha2.ResourceBinding) bool {
					if rb.Spec.ConflictResolution != policyv1alpha1.ConflictOverwrite {
						return false
					}
					return rb.Spec.PreserveResourcesOnDeletion != nil && !*rb.Spec.PreserveResourcesOnDeletion
				})
			})
		})

		ginkgo.It("conflict on preserve flag only", func() {
			ginkgo.By("set policy A Overwrite+true and policy B Overwrite+false", func() {
				framework.PatchPropagationPolicy(karmadaClient, policyA.Namespace, policyA.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": true},
				}, types.JSONPatchType)
				framework.PatchPropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": false},
				}, types.JSONPatchType)

				waitRBUpdated(independentBindingAName, policyv1alpha1.ConflictOverwrite, func() *bool { b := true; return &b }())
				waitRBUpdated(independentBindingBName, policyv1alpha1.ConflictOverwrite, func() *bool { b := false; return &b }())
			})

			ginkgo.By("event only reports preserve conflict", func() {
				framework.WaitEventFitWith(kubeClient, testNamespace, attachedBindingName, func(e corev1.Event) bool {
					return e.Reason == events.EventReasonDependencyPolicyConflict &&
						strings.Contains(e.Message, "PreserveResourcesOnDeletion conflicted (true vs false)") &&
						!strings.Contains(e.Message, "ConflictResolution conflicted (Overwrite vs Abort)")
				})
			})

			ginkgo.By("resolved policy becomes Overwrite+true", func() {
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedBindingName, func(rb *workv1alpha2.ResourceBinding) bool {
					if rb.Spec.ConflictResolution != policyv1alpha1.ConflictOverwrite {
						return false
					}
					return rb.Spec.PreserveResourcesOnDeletion != nil && *rb.Spec.PreserveResourcesOnDeletion
				})
			})
		})

		ginkgo.It("conflict resolved by updating policy", func() {
			ginkgo.By("start with policy A Overwrite+true and policy B Abort+false", func() {
				framework.PatchPropagationPolicy(karmadaClient, policyA.Namespace, policyA.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": true},
				}, types.JSONPatchType)
				framework.PatchPropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictAbort)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": false},
				}, types.JSONPatchType)

				waitRBUpdated(independentBindingAName, policyv1alpha1.ConflictOverwrite, func() *bool { b := true; return &b }())
				waitRBUpdated(independentBindingBName, policyv1alpha1.ConflictAbort, func() *bool { b := false; return &b }())
			})

			ginkgo.By("conflict event fires before alignment", func() {
				framework.WaitEventFitWith(kubeClient, testNamespace, attachedBindingName, func(e corev1.Event) bool {
					return e.Reason == events.EventReasonDependencyPolicyConflict &&
						strings.Contains(e.Message, "ConflictResolution conflicted (Overwrite vs Abort)") &&
						strings.Contains(e.Message, "PreserveResourcesOnDeletion conflicted (true vs false)")
				})
			})

			ginkgo.By("align policy B with policy A", func() {
				framework.PatchPropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": true},
				}, types.JSONPatchType)
				waitRBUpdated(independentBindingBName, policyv1alpha1.ConflictOverwrite, func() *bool { b := true; return &b }())
			})

			ginkgo.By("conflict resolved, converges to Overwrite+true", func() {
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedBindingName, func(rb *workv1alpha2.ResourceBinding) bool {
					return rb.Spec.ConflictResolution == policyv1alpha1.ConflictOverwrite && rb.Spec.PreserveResourcesOnDeletion != nil && *rb.Spec.PreserveResourcesOnDeletion
				})
			})
		})

		ginkgo.It("conflict resolved after deleting policy B", func() {
			ginkgo.By("start with policy A Overwrite+true and policy B Abort+false", func() {
				framework.PatchPropagationPolicy(karmadaClient, policyA.Namespace, policyA.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictOverwrite)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": true},
				}, types.JSONPatchType)
				framework.PatchPropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name, []map[string]interface{}{
					{"op": "replace", "path": "/spec/conflictResolution", "value": string(policyv1alpha1.ConflictAbort)},
					{"op": "replace", "path": "/spec/preserveResourcesOnDeletion", "value": false},
				}, types.JSONPatchType)

				waitRBUpdated(independentBindingAName, policyv1alpha1.ConflictOverwrite, func() *bool { b := true; return &b }())
				waitRBUpdated(independentBindingBName, policyv1alpha1.ConflictAbort, func() *bool { b := false; return &b }())
			})

			ginkgo.By("delete policy B to drop its binding", func() {
				framework.RemovePropagationPolicy(karmadaClient, policyB.Namespace, policyB.Name)
			})

			ginkgo.By("conflict resolved, follows policy A after deletion", func() {
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedBindingName, func(rb *workv1alpha2.ResourceBinding) bool {
					return rb.Spec.ConflictResolution == policyv1alpha1.ConflictOverwrite && rb.Spec.PreserveResourcesOnDeletion != nil && *rb.Spec.PreserveResourcesOnDeletion
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
