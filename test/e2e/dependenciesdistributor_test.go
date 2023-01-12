package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[DependenciesDistributor] automatically propagate relevant resources testing", func() {
	ginkgo.Context("dependencies propagation testing", func() {
		var initClusterNames, updateClusterNames []string
		var policyName string
		var deploymentName string
		var deployment *appsv1.Deployment
		var policy *policyv1alpha1.PropagationPolicy

		ginkgo.BeforeEach(func() {
			initClusterNames = []string{"member1"}
			updateClusterNames = []string{"member2"}

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
				pvc = testhelper.NewPVC(testNamespace, pvcName, corev1.ResourceRequirements{
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
						func(deployment *appsv1.Deployment) bool {
							return true
						})

					framework.WaitPVCPresentOnClustersFitWith(initClusterNames, pvc.GetNamespace(), pvc.GetName(),
						func(pvc *corev1.PersistentVolumeClaim) bool {
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
						func(deployment *appsv1.Deployment) bool {
							return true
						})

					framework.WaitServiceAccountPresentOnClustersFitWith(initClusterNames, sa.GetNamespace(), sa.GetName(),
						func(sa *corev1.ServiceAccount) bool {
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

	ginkgo.Context("across namespace dependencies propagation testing", func() {
		var crdGroup string
		var randStr string
		var crdSpecNames apiextensionsv1.CustomResourceDefinitionNames
		var crd *apiextensionsv1.CustomResourceDefinition
		var crdPolicy *policyv1alpha1.ClusterPropagationPolicy
		var crNamespace, crName string
		var crGVR schema.GroupVersionResource
		var crAPIVersion string
		var cr *unstructured.Unstructured
		var crPolicy *policyv1alpha1.PropagationPolicy
		var initClusterNames, updateClusterNames []string

		ginkgo.BeforeEach(func() {
			initClusterNames = []string{"member1"}
			updateClusterNames = []string{"member2"}
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
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, crdPolicy)
			framework.CreateCRD(dynamicClient, crd)
			framework.WaitCRDPresentOnClusters(karmadaClient, framework.ClusterNames(),
				fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1"), crd.Spec.Names.Kind)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, crdPolicy.Name)
				framework.RemoveCRD(dynamicClient, crd.Name)
				framework.WaitCRDDisappearedOnClusters(framework.ClusterNames(), crd.Name)
			})
		})

		ginkgo.JustBeforeEach(func() {
			ginkgo.By(fmt.Sprintf("create cr(%s/%s)", crNamespace, crName), func() {
				_, err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Create(context.TODO(), cr, metav1.CreateOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
			framework.CreatePropagationPolicy(karmadaClient, crPolicy)
			ginkgo.DeferCleanup(func() {
				ginkgo.By(fmt.Sprintf("remove cr(%s/%s)", crNamespace, crName), func() {
					err := dynamicClient.Resource(crGVR).Namespace(crNamespace).Delete(context.TODO(), crName, metav1.DeleteOptions{})
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
				framework.RemovePropagationPolicy(karmadaClient, crPolicy.Namespace, crPolicy.Name)
			})
		})

		ginkgo.When("across namespace configmap propagate automatically", func() {
			var configMapName string
			var configMap *corev1.ConfigMap
			var customizationConfigMap *configv1alpha1.ResourceInterpreterCustomization
			ginkgo.BeforeEach(func() {
				configMapName = configMapNamePrefix + rand.String(RandomStrLength)
				configMap = testhelper.NewConfigMap("default", configMapName, map[string]string{"user": "karmada"})
				crNamespace = testNamespace
				crName = crdNamePrefix + rand.String(RandomStrLength)
				crGVR = schema.GroupVersionResource{Group: crd.Spec.Group, Version: "v1alpha1", Resource: crd.Spec.Names.Plural}
				crAPIVersion = fmt.Sprintf("%s/%s", crd.Spec.Group, "v1alpha1")
				cr = testhelper.NewCustomResourceWithConfigMap(crAPIVersion, crd.Spec.Names.Kind, crNamespace, crName, configMapName)
				customizationConfigMap = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: cr.GetAPIVersion(),
						Kind:       cr.GetKind(),
					},
					configv1alpha1.CustomizationRules{
						DependencyInterpretation: &configv1alpha1.DependencyInterpretation{
							LuaScript: `
function GetDependencies(desiredObj)
    dependencies = {}
	if desiredObj.spec.resource ~= nil and desiredObj.spec.resource.kind == 'ConfigMap' then
		dependObj = {}
		dependObj.apiVersion = 'v1'
		dependObj.kind = 'ConfigMap'
		dependObj.name = desiredObj.spec.resource.name
		dependObj.namespace = desiredObj.spec.resource.namespace
		dependencies[1] = dependObj
	end
	return dependencies
end `,
						},
					})
				crPolicy = testhelper.NewPropagationPolicy(crNamespace, crName, []policyv1alpha1.ResourceSelector{
					{
						APIVersion: crAPIVersion,
						Kind:       crd.Spec.Names.Kind,
						Name:       crName,
					},
				}, policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: initClusterNames,
					},
				})
				crPolicy.Spec.PropagateDeps = true
			})

			ginkgo.BeforeEach(func() {
				framework.CreateConfigMap(kubeClient, configMap)
				framework.CreateResourceInterpreterCustomization(karmadaClient, customizationConfigMap)
				// Wait for resource interpreter informer synced.
				time.Sleep(time.Second)
				ginkgo.DeferCleanup(func() {
					framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMapName)
					framework.DeleteResourceInterpreterCustomization(karmadaClient, customizationConfigMap.Name)
				})
			})

			ginkgo.It("across namespace configmap automatically propagation testing", func() {
				ginkgo.By("check if cr present on member clusters", func() {
					for _, cluster := range initClusterNames {
						clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
						gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())
						klog.Infof("Waiting for cr(%s/%s) present on cluster(%s)", crNamespace, crName, cluster)
						err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
							_, err = clusterDynamicClient.Resource(crGVR).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
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

				ginkgo.By("check if the configmap is propagated automatically", func() {
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

					framework.PatchPropagationPolicy(karmadaClient, crPolicy.Namespace, crPolicy.Name, patch, types.JSONPatchType)
					ginkgo.By("check if cr present on member clusters", func() {
						for _, cluster := range updateClusterNames {
							clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
							gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())
							klog.Infof("Waiting for cr(%s/%s) present on cluster(%s)", crNamespace, crName, cluster)
							err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
								_, err = clusterDynamicClient.Resource(crGVR).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
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
			})
		})
	})
})
