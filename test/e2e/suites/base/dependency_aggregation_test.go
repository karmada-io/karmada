// filepath: /Users/tangkexin/Desktop/karmada/test/e2e/suites/base/dependency_aggregation_test.go
package base

import (
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("[DependenciesDistributor][Aggregation] multi-parent dependency tracking", func() {
	var clusters []string

	ginkgo.BeforeEach(func() {
		clusters = []string{framework.ClusterNames()[0]}
	})

	ginkgo.It("tracks multiple parents in RequiredBy and updates on reference changes", func() {
		// shared ConfigMap
		cmName := configMapNamePrefix + rand.String(RandomStrLength)
		cm := testhelper.NewConfigMap(testNamespace, cmName, map[string]string{"k": "v"})
		framework.CreateConfigMap(kubeClient, cm)
		ginkgo.DeferCleanup(func() { framework.RemoveConfigMap(kubeClient, testNamespace, cmName) })

		// two deployments referencing same ConfigMap
		vol := corev1.Volume{
			Name:         "vol-configmap",
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cmName}}},
		}
		depAName := deploymentNamePrefix + rand.String(RandomStrLength)
		depBName := deploymentNamePrefix + rand.String(RandomStrLength)
		depA := testhelper.NewDeploymentWithVolumes(testNamespace, depAName, []corev1.Volume{vol})
		depB := testhelper.NewDeploymentWithVolumes(testNamespace, depBName, []corev1.Volume{vol})
		framework.CreateDeployment(kubeClient, depA)
		framework.CreateDeployment(kubeClient, depB)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, depA.Namespace, depA.Name)
			framework.RemoveDeployment(kubeClient, depB.Namespace, depB.Name)
		})

		// create PropagationPolicies with PropagateDeps enabled
		ppA := testhelper.NewPropagationPolicy(testNamespace, depAName, []policyv1alpha1.ResourceSelector{{
			APIVersion: depA.APIVersion, Kind: depA.Kind, Name: depA.Name,
		}}, policyv1alpha1.Placement{ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: clusters}})
		ppA.Spec.PropagateDeps = true
		ppB := testhelper.NewPropagationPolicy(testNamespace, depBName, []policyv1alpha1.ResourceSelector{{
			APIVersion: depB.APIVersion, Kind: depB.Kind, Name: depB.Name,
		}}, policyv1alpha1.Placement{ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: clusters}})
		ppB.Spec.PropagateDeps = true
		framework.CreatePropagationPolicy(karmadaClient, ppA)
		framework.CreatePropagationPolicy(karmadaClient, ppB)
		ginkgo.DeferCleanup(func() {
			framework.RemovePropagationPolicy(karmadaClient, ppA.Namespace, ppA.Name)
			framework.RemovePropagationPolicy(karmadaClient, ppB.Namespace, ppB.Name)
		})

		// Ensure deployments are scheduled
		framework.WaitDeploymentPresentOnClustersFitWith(clusters, depA.Namespace, depA.Name, func(*appsv1.Deployment) bool { return true })
		framework.WaitDeploymentPresentOnClustersFitWith(clusters, depB.Namespace, depB.Name, func(*appsv1.Deployment) bool { return true })

		attachedRBName := names.GenerateBindingName("ConfigMap", cmName)

		ginkgo.By("RequiredBy contains both parents", func() {
			framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedRBName, func(rb *workv1alpha2.ResourceBinding) bool {
				return len(rb.Spec.RequiredBy) == 2
			})
		})

		ginkgo.By("when one parent stops referencing, RequiredBy shrinks", func() {
			framework.UpdateDeploymentVolumes(kubeClient, depB, []corev1.Volume{{
				Name:         "dummy",
				VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "dummy-cm"}}},
			}})
			framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, attachedRBName, func(rb *workv1alpha2.ResourceBinding) bool {
				return len(rb.Spec.RequiredBy) == 1 && rb.Spec.RequiredBy[0].Name == names.GenerateBindingName(depA.Kind, depA.Name)
			})
		})
	})
})
