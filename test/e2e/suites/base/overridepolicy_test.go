/*
Copyright 2021 The Karmada Authors.

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
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[OverridePolicy] apply overriders testing", func() {
	var propagationPolicyNamespace, propagationPolicyName string
	var overridePolicyNamespace, overridePolicyName string
	var propagationPolicy *policyv1alpha1.PropagationPolicy
	var overridePolicy *policyv1alpha1.OverridePolicy

	ginkgo.Context("[LabelsOverrider] apply labels overrider testing", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = deploymentName
			overridePolicyNamespace = testNamespace
			overridePolicyName = deploymentName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetLabels(map[string]string{
				"foo": "foo",
				"bar": "bar"},
			)

			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpReplace,
						Value: map[string]string{
							"foo":       "exist",
							"non-exist": "non-exist",
						},
					},
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"app": "nginx",
						},
					},
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value: map[string]string{
							"bar": "bar",
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment labelsOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct labels value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					_, labelBarExist := deployment.GetLabels()["bar"]
					_, labelNonExist := deployment.GetLabels()["non-exist"]
					return !labelBarExist && !labelNonExist &&
						deployment.GetLabels()["foo"] == "exist" &&
						deployment.GetLabels()["app"] == "nginx"
				})
		})
	})
	ginkgo.Context("[AnnotationsOverrider] apply annotations overrider testing", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = deploymentName
			overridePolicyNamespace = testNamespace
			overridePolicyName = deploymentName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			deployment.SetAnnotations(map[string]string{
				"foo": "foo",
				"bar": "bar"},
			)

			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				AnnotationsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
					{
						Operator: policyv1alpha1.OverriderOpReplace,
						Value: map[string]string{
							"foo":       "exist",
							"non-exist": "non-exist",
						},
					},
					{
						Operator: policyv1alpha1.OverriderOpAdd,
						Value: map[string]string{
							"app": "nginx",
						},
					},
					{
						Operator: policyv1alpha1.OverriderOpRemove,
						Value: map[string]string{
							"bar": "bar",
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment annotationsOverrider testing", func() {
			klog.Infof("check if deployment present on member clusters have correct annotations value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					_, labelBarExist := deployment.GetAnnotations()["bar"]
					_, labelNonExist := deployment.GetAnnotations()["non-exist"]
					return !labelBarExist && !labelNonExist &&
						deployment.GetAnnotations()["foo"] == "exist" &&
						deployment.GetAnnotations()["app"] == "nginx"
				})
		})
	})

	ginkgo.Context("Deployment override all images in container list", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = deploymentName
			overridePolicyNamespace = testNamespace
			overridePolicyName = deploymentName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				ImageOverrider: []policyv1alpha1.ImageOverrider{
					{
						Component: "Registry",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "fictional.registry.us",
					},
					{
						Component: "Repository",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "busybox",
					},
					{
						Component: "Tag",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "1.0",
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Image != "fictional.registry.us/busybox:1.0" {
							return false
						}
					}
					return true
				})
		})
	})

	ginkgo.Context("Pod override all images in container list", func() {
		var podNamespace, podName string
		var pod *corev1.Pod

		ginkgo.BeforeEach(func() {
			podNamespace = testNamespace
			podName = podNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = podName
			overridePolicyNamespace = testNamespace
			overridePolicyName = podName

			pod = helper.NewPod(podNamespace, podName)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: pod.APIVersion,
					Kind:       pod.Kind,
					Name:       pod.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				ImageOverrider: []policyv1alpha1.ImageOverrider{
					{
						Component: "Registry",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "fictional.registry.us",
					},
					{
						Component: "Repository",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "busybox",
					},
					{
						Component: "Tag",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "1.0",
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreatePod(kubeClient, pod)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
			})
		})

		ginkgo.It("pod imageOverride testing", func() {
			klog.Infof("check if pod present on member clusters have correct image value")
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name, func(pod *corev1.Pod) bool {
				for _, container := range pod.Spec.Containers {
					if container.Image != "fictional.registry.us/busybox:1.0" {
						return false
					}
				}
				return true
			})
		})
	})

	ginkgo.Context("Deployment override specific images in container list", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = deploymentName
			overridePolicyNamespace = testNamespace
			overridePolicyName = deploymentName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				ImageOverrider: []policyv1alpha1.ImageOverrider{
					{
						Predicate: &policyv1alpha1.ImagePredicate{
							Path: "/spec/template/spec/containers/0/image",
						},
						Component: "Registry",
						Operator:  policyv1alpha1.OverriderOpReplace,
						Value:     "fictional.registry.us",
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})
		})
	})

	ginkgo.Context("[FieldOverrider] apply field overrider testing to update JSON values in ConfigMap", func() {
		var configMapNamespace, configMapName string
		var configMap *corev1.ConfigMap

		ginkgo.BeforeEach(func() {
			configMapNamespace = testNamespace
			configMapName = configMapNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = configMapName
			overridePolicyNamespace = testNamespace
			overridePolicyName = configMapName

			configMapData := map[string]string{
				"deploy.json": fmt.Sprintf(`{
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {
					"name": "nginx-deploy",
					"namespace": "%s"
				},
				"spec": {
					"replicas": 3,
					"selector": {
						"matchLabels": {
							"app": "nginx"
						}
					},
					"template": {
						"metadata": {
							"labels": {
								"app": "nginx"
							}
						},
						"spec": {
							"containers": [
								{
									"name": "nginx",
									"image": "nginx:1.19.0"
								}
							]
						}
					}
				}
			}`, configMapNamespace),
			}

			configMap = helper.NewConfigMap(configMapNamespace, configMapName, configMapData)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: configMap.APIVersion,
					Kind:       configMap.Kind,
					Name:       configMap.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})

			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: configMap.APIVersion,
					Kind:       configMap.Kind,
					Name:       configMap.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				FieldOverrider: []policyv1alpha1.FieldOverrider{
					{
						FieldPath: "/data/deploy.json",
						JSON: []policyv1alpha1.JSONPatchOperation{
							{
								SubPath:  "/spec/replicas",
								Operator: policyv1alpha1.OverriderOpReplace,
								Value:    apiextensionsv1.JSON{Raw: []byte(`5`)},
							},
							{
								SubPath:  "/spec/template/spec/containers/-",
								Operator: policyv1alpha1.OverriderOpAdd,
								Value:    apiextensionsv1.JSON{Raw: []byte(`{"name": "nginx-helper", "image": "nginx:1.19.1"}`)},
							},
							{
								SubPath:  "/spec/template/spec/containers/0/image",
								Operator: policyv1alpha1.OverriderOpRemove,
							},
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateConfigMap(kubeClient, configMap)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMap.Name)
			})
		})

		ginkgo.It("should override JSON field in ConfigMap", func() {
			klog.Infof("check if configMap present on member clusters has the correct JSON field value.")
			framework.WaitConfigMapPresentOnClustersFitWith(framework.ClusterNames(), configMap.Namespace, configMap.Name,
				func(cm *corev1.ConfigMap) bool {
					return strings.Contains(cm.Data["deploy.json"], `"replicas":5`) &&
						strings.Contains(cm.Data["deploy.json"], `"name":"nginx-helper"`) &&
						!strings.Contains(cm.Data["deploy.json"], `"image":"nginx:1.19.0"`)
				})
		})
	})

	ginkgo.Context("[FieldOverrider] apply field overrider testing to update YAML values in ConfigMap", func() {
		var configMapNamespace, configMapName string
		var configMap *corev1.ConfigMap

		ginkgo.BeforeEach(func() {
			configMapNamespace = testNamespace
			configMapName = configMapNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = configMapName
			overridePolicyNamespace = testNamespace
			overridePolicyName = configMapName

			// Define the ConfigMap data
			configMapData := map[string]string{
				"nginx.yaml": `
server:
  listen: 80
  server_name: localhost
  location /:
    root: /usr/share/nginx/html
    index: 
      - index.html
      - index.htm
  error_page:
    - code: 500
    - code: 502
    - code: 503
    - code: 504
  location /50x.html:
    root: /usr/share/nginx/html
`,
			}
			configMap = helper.NewConfigMap(configMapNamespace, configMapName, configMapData)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: configMap.APIVersion,
					Kind:       configMap.Kind,
					Name:       configMap.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})

			overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: configMap.APIVersion,
					Kind:       configMap.Kind,
					Name:       configMap.Name,
				},
			}, policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			}, policyv1alpha1.Overriders{
				FieldOverrider: []policyv1alpha1.FieldOverrider{
					{
						FieldPath: "/data/nginx.yaml",
						YAML: []policyv1alpha1.YAMLPatchOperation{
							{
								SubPath:  "/server/location ~1/root",
								Operator: policyv1alpha1.OverriderOpReplace,
								Value:    apiextensionsv1.JSON{Raw: []byte(`"/var/www/html"`)},
							},
							{
								SubPath:  "/server/error_page/-",
								Operator: policyv1alpha1.OverriderOpAdd,
								Value:    apiextensionsv1.JSON{Raw: []byte(`{"code": 400}`)},
							},
							{
								SubPath:  "/server/location ~1/index",
								Operator: policyv1alpha1.OverriderOpRemove,
							},
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateConfigMap(kubeClient, configMap)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMap.Name)
			})
		})

		ginkgo.It("should override YAML field in ConfigMap", func() {
			klog.Infof("check if configMap present on member clusters has the correct YAML field value.")
			framework.WaitConfigMapPresentOnClustersFitWith(framework.ClusterNames(), configMap.Namespace, configMap.Name,
				func(cm *corev1.ConfigMap) bool {
					return strings.Contains(cm.Data["nginx.yaml"], "root: /var/www/html") &&
						strings.Contains(cm.Data["nginx.yaml"], "code: 400") &&
						!strings.Contains(cm.Data["nginx.yaml"], "- index.html")
				})
		})
	})

})

var _ = framework.SerialDescribe("OverridePolicy with nil resourceSelector testing", func() {
	var deploymentNamespace, deploymentName string
	var propagationPolicyNamespace, propagationPolicyName string
	var overridePolicyNamespace, overridePolicyName string
	var deployment *appsv1.Deployment
	var propagationPolicy *policyv1alpha1.PropagationPolicy
	var overridePolicy *policyv1alpha1.OverridePolicy

	ginkgo.BeforeEach(func() {
		deploymentNamespace = testNamespace
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace = testNamespace
		propagationPolicyName = deploymentName
		overridePolicyNamespace = testNamespace
		overridePolicyName = deploymentName

		deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
		overridePolicy = helper.NewOverridePolicy(overridePolicyNamespace, overridePolicyName, nil, policyv1alpha1.ClusterAffinity{
			ClusterNames: framework.ClusterNames(),
		}, policyv1alpha1.Overriders{
			ImageOverrider: []policyv1alpha1.ImageOverrider{
				{
					Predicate: &policyv1alpha1.ImagePredicate{
						Path: "/spec/template/spec/containers/0/image",
					},
					Component: "Registry",
					Operator:  policyv1alpha1.OverriderOpReplace,
					Value:     "fictional.registry.us",
				},
			},
		})
	})

	ginkgo.Context("Deployment override testing", func() {
		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})
		})
	})
})

var _ = ginkgo.Describe("[OverrideRules] apply overriders testing", func() {
	var propagationPolicyNamespace, propagationPolicyName string
	var overridePolicyNamespace, overridePolicyName string
	var propagationPolicy *policyv1alpha1.PropagationPolicy
	var overridePolicy *policyv1alpha1.OverridePolicy

	ginkgo.Context("Deployment override all images in container list", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = deploymentName
			overridePolicyNamespace = testNamespace
			overridePolicyName = deploymentName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
					Overriders: policyv1alpha1.Overriders{
						ImageOverrider: []policyv1alpha1.ImageOverrider{
							{
								Component: "Registry",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "fictional.registry.us",
							},
							{
								Component: "Repository",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "busybox",
							},
							{
								Component: "Tag",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "1.0",
							},
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Image != "fictional.registry.us/busybox:1.0" {
							return false
						}
					}
					return true
				})
		})
	})

	ginkgo.Context("Pod override all images in container list", func() {
		var podNamespace, podName string
		var pod *corev1.Pod

		ginkgo.BeforeEach(func() {
			podNamespace = testNamespace
			podName = podNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = podName
			overridePolicyNamespace = testNamespace
			overridePolicyName = podName

			pod = helper.NewPod(podNamespace, podName)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: pod.APIVersion,
					Kind:       pod.Kind,
					Name:       pod.Name,
				},
			}, []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
					Overriders: policyv1alpha1.Overriders{
						ImageOverrider: []policyv1alpha1.ImageOverrider{
							{
								Component: "Registry",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "fictional.registry.us",
							},
							{
								Component: "Repository",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "busybox",
							},
							{
								Component: "Tag",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "1.0",
							},
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreatePod(kubeClient, pod)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemovePod(kubeClient, pod.Namespace, pod.Name)
			})
		})

		ginkgo.It("pod imageOverride testing", func() {
			klog.Infof("check if pod present on member clusters have correct image value")
			framework.WaitPodPresentOnClustersFitWith(framework.ClusterNames(), pod.Namespace, pod.Name, func(pod *corev1.Pod) bool {
				for _, container := range pod.Spec.Containers {
					if container.Image != "fictional.registry.us/busybox:1.0" {
						return false
					}
				}
				return true
			})
		})
	})

	ginkgo.Context("Deployment override specific images in container list", func() {
		var deploymentNamespace, deploymentName string
		var deployment *appsv1.Deployment

		ginkgo.BeforeEach(func() {
			deploymentNamespace = testNamespace
			deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
			propagationPolicyNamespace = testNamespace
			propagationPolicyName = deploymentName
			overridePolicyNamespace = testNamespace
			overridePolicyName = deploymentName

			deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
			propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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
			overridePolicy = helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
					Overriders: policyv1alpha1.Overriders{
						ImageOverrider: []policyv1alpha1.ImageOverrider{
							{
								Predicate: &policyv1alpha1.ImagePredicate{
									Path: "/spec/template/spec/containers/0/image",
								},
								Component: "Registry",
								Operator:  policyv1alpha1.OverriderOpReplace,
								Value:     "fictional.registry.us",
							},
						},
					},
				},
			})
		})

		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})
		})
	})
})

var _ = framework.SerialDescribe("OverrideRules with nil resourceSelector testing", func() {
	var deploymentNamespace, deploymentName string
	var propagationPolicyNamespace, propagationPolicyName string
	var overridePolicyNamespace, overridePolicyName string
	var deployment *appsv1.Deployment
	var propagationPolicy *policyv1alpha1.PropagationPolicy
	var overridePolicy *policyv1alpha1.OverridePolicy

	ginkgo.BeforeEach(func() {
		deploymentNamespace = testNamespace
		deploymentName = deploymentNamePrefix + rand.String(RandomStrLength)
		propagationPolicyNamespace = testNamespace
		propagationPolicyName = deploymentName
		overridePolicyNamespace = testNamespace
		overridePolicyName = deploymentName

		deployment = helper.NewDeployment(deploymentNamespace, deploymentName)
		propagationPolicy = helper.NewPropagationPolicy(propagationPolicyNamespace, propagationPolicyName, []policyv1alpha1.ResourceSelector{
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

		overridePolicy = helper.NewOverridePolicyByOverrideRules(overridePolicyNamespace, overridePolicyName, nil, []policyv1alpha1.RuleWithCluster{
			{
				TargetCluster: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				Overriders: policyv1alpha1.Overriders{
					ImageOverrider: []policyv1alpha1.ImageOverrider{
						{
							Predicate: &policyv1alpha1.ImagePredicate{
								Path: "/spec/template/spec/containers/0/image",
							},
							Component: "Registry",
							Operator:  policyv1alpha1.OverriderOpReplace,
							Value:     "fictional.registry.us",
						},
					},
				},
			},
		})
	})

	ginkgo.Context("Deployment override testing", func() {
		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateOverridePolicy(karmadaClient, overridePolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			klog.Infof("check if deployment present on member clusters have correct image value")
			framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
				})
		})
	})

	ginkgo.Context("Deployment override testing when creating overridePolicy after develop and propagationPolicy have been created", func() {
		ginkgo.BeforeEach(func() {
			framework.CreatePropagationPolicy(karmadaClient, propagationPolicy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, propagationPolicy.Namespace, propagationPolicy.Name)
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			})
		})

		ginkgo.It("deployment imageOverride testing", func() {
			ginkgo.By("Check if deployment have presented on member clusters", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(*appsv1.Deployment) bool {
						return true
					})
			})

			framework.CreateOverridePolicy(karmadaClient, overridePolicy)

			ginkgo.By("Check if deployment presented on member clusters have correct image value", func() {
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name,
					func(deployment *appsv1.Deployment) bool {
						return deployment.Spec.Template.Spec.Containers[0].Image == "fictional.registry.us/nginx:1.19.0"
					})
			})

			framework.RemoveOverridePolicy(karmadaClient, overridePolicy.Namespace, overridePolicy.Name)
		})
	})
})
