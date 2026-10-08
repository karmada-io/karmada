/*
Copyright 2026 The Karmada Authors.

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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Quota plugin Testing", func() {
	var resourceQuota corev1.ResourceQuota
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	var rqNamespace, rqName string
	var deployNamespace, deployName string
	var policyNamespace, policyName string
	var targetCluster string

	var err error

	ginkgo.BeforeEach(func() {
		targetCluster = framework.ClusterNames()[0]

		ginkgo.By("set up namespace", func() {
			// To avoid conflicts with other test cases, use random strings to generate unique namespaces instead of using testNamespace.
			deployNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			err = setupTestNamespace(deployNamespace, kubeClient)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				framework.RemoveNamespace(kubeClient, deployNamespace)
			})
		})

		deployName = deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = helper.NewDeployment(deployNamespace, deployName)

		ginkgo.By("create resourceQuota", func() {
			rqNamespace = deployNamespace
			rqName = resourceQuotaPrefix + rand.String(RandomStrLength)
			resourceQuota = corev1.ResourceQuota{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ResourceQuota",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rqName,
					Namespace: rqNamespace,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"requests.cpu": resource.MustParse("0.03"), // Equals to the resource requested by the deployment created by helper.NewDeployment: 3 replicas × 10 milli CPU
					},
				},
			}
			framework.CreateResourceQuota(kubeClient, &resourceQuota)
			ginkgo.DeferCleanup(func() {
				framework.RemoveResourceQuota(kubeClient, rqNamespace, rqName)
			})
		})

		ginkgo.By("create propagation policy", func() {
			policyNamespace = deployNamespace
			policyName = ppNamePrefix + rand.String(RandomStrLength)
			policy = helper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
				{
					APIVersion: resourceQuota.APIVersion,
					Kind:       resourceQuota.Kind,
					Name:       resourceQuota.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetCluster},
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
					},
				},
			})
			framework.CreatePropagationPolicy(karmadaClient, policy)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, policyNamespace, policyName)
			})
		})

		// To ensure that the resource quota is created on the target cluster before creating the deployment.
		framework.WaitResourceQuotaPresentOnCluster(targetCluster, rqNamespace, rqName)
	})

	ginkgo.It("Deployment should be successfully propagated to target cluster within resource quota limits", func() {
		ginkgo.By("Creating deployment within resource quota limits", func() {
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployNamespace, deployName)
			})
		})

		ginkgo.By("first verifying resource binding scheduling", func() {
			deployBindingName := names.GenerateBindingName(util.DeploymentKind, deployName)
			framework.AssertBindingScheduledClusters(karmadaClient, deployNamespace, deployBindingName, [][]string{{targetCluster}})
			framework.WaitResourceBindingFitWith(karmadaClient, deployNamespace, deployBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				if binding.Spec.ReplicaRequirements == nil {
					return false
				}
				return binding.Spec.ReplicaRequirements.Namespace == deployNamespace
			})
		})

		ginkgo.By("Verifying deployment propagation to target cluster", func() {
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deployNamespace, deployName, func(deploy *appsv1.Deployment) bool {
				return framework.CheckDeploymentReadyStatus(deploy, *deployment.Spec.Replicas)
			})
		})
	})

	ginkgo.It("Deployment should not be propagated to target cluster exceeding resource quota limits", func() {
		ginkgo.By("Creating deployment exceeding resource quota limits", func() {
			deployment.Spec.Replicas = ptr.To[int32](5) // This will request 0.05 CPU which exceeds the quota of 0.03 CPU
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployNamespace, deployName)
			})
		})

		ginkgo.By("first verifying resource binding scheduling", func() {
			deployBindingName := names.GenerateBindingName(util.DeploymentKind, deployName)
			framework.WaitResourceBindingFitWith(karmadaClient, deployNamespace, deployBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
				return binding.Spec.Clusters == nil && cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == workv1alpha2.BindingReasonUnschedulable
			})
			framework.WaitResourceBindingFitWith(karmadaClient, deployNamespace, deployBindingName, func(binding *workv1alpha2.ResourceBinding) bool {
				if binding.Spec.ReplicaRequirements == nil {
					return false
				}
				return binding.Spec.ReplicaRequirements.Namespace == deployNamespace
			})
		})

		ginkgo.By("Verifying deployment is not propagated to target cluster", func() {
			framework.WaitDeploymentDisappearOnCluster(targetCluster, deployNamespace, deployName)
		})
	})
})

// flinkDeploymentGVR is the GroupVersionResource for FlinkDeployment.
var flinkDeploymentGVR = schema.GroupVersionResource{
	Group:    "flink.apache.org",
	Version:  "v1beta1",
	Resource: "flinkdeployments",
}

var _ = framework.SerialDescribe("[EstimatorAssumption] ResourceQuota plugin assumption testing", func() {
	const targetCluster = "member1"

	var flinkCRD apiextensionsv1.CustomResourceDefinition
	var quotaNamespace, rqName string

	ginkgo.BeforeEach(func() {
		// Use a dedicated namespace so the ResourceQuota only constrains this test's workloads.
		quotaNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
		err := setupTestNamespace(quotaNamespace, kubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		ginkgo.DeferCleanup(func() {
			framework.RemoveNamespace(kubeClient, quotaNamespace)
		})
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("creating FlinkDeployment CRD on karmada control plane", func() {
			err := yaml.Unmarshal([]byte(flinkDeploymentCRDYAML), &flinkCRD)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.CreateCRD(dynamicClient, &flinkCRD)
			framework.WaitCRDEstablished(dynamicClient, flinkCRD.Name)
			ginkgo.DeferCleanup(func() {
				framework.RemoveCRD(dynamicClient, flinkCRD.Name)
				framework.WaitCRDDisappeared(dynamicClient, flinkCRD.Name)
				framework.WaitCRDDisappearedOnClusters([]string{targetCluster}, flinkCRD.Name)
				framework.WaitCRDDisappearedFromClusterStatus(karmadaClient, []string{targetCluster},
					fmt.Sprintf("%s/%s", flinkCRD.Spec.Group, "v1beta1"), flinkCRD.Spec.Names.Kind)
			})
		})

		ginkgo.By("propagating FlinkDeployment CRD to member1", func() {
			cpp := helper.NewClusterPropagationPolicy(cppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: flinkCRD.APIVersion,
					Kind:       flinkCRD.Kind,
					Name:       flinkCRD.Name,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
				})
			framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
			framework.WaitCRDPresentOnClusters(karmadaClient, []string{targetCluster},
				fmt.Sprintf("%s/%s", flinkCRD.Spec.Group, "v1beta1"), flinkCRD.Spec.Names.Kind)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
			})
		})
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("creating ResourceQuota with cpu=1 and propagating to member1", func() {
			rqName = resourceQuotaPrefix + rand.String(RandomStrLength)
			rq := &corev1.ResourceQuota{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ResourceQuota",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rqName,
					Namespace: quotaNamespace,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			}
			framework.CreateResourceQuota(kubeClient, rq)
			ginkgo.DeferCleanup(func() {
				framework.RemoveResourceQuota(kubeClient, quotaNamespace, rqName)
			})

			pp := helper.NewPropagationPolicy(quotaNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: rq.TypeMeta.APIVersion,
					Kind:       rq.TypeMeta.Kind,
					Name:       rqName,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
				})
			framework.CreatePropagationPolicy(karmadaClient, pp)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, quotaNamespace, pp.Name)
			})

			// Ensure the quota exists on member1 before creating any FlinkDeployments.
			framework.WaitResourceQuotaPresentOnCluster(targetCluster, quotaNamespace, rqName)
		})
	})

	ginkgo.It("FlinkDeployment should be unschedulable when assumed workloads exhaust ResourceQuota", func(ctx context.Context) {
		// Each FlinkDeployment uses jobManager(50m) + taskManager(100m) = 150m CPU (from manifest).
		// With ResourceQuota of 1000m and no real pods running (ResourceQuota.Status.Used stays at 0),
		// the resourcequota estimator deducts in-flight assumed workloads:
		//   - FlinkDeployments 1-6: 6 × 150m = 900m assumed → each is schedulable.
		//   - FlinkDeployment 7:  900m + 150m = 1050m > 1000m → estimator returns 0 → unschedulable.
		const schedulableCount = 6

		// createFlinkDeployment creates a FlinkDeployment in quotaNamespace with a PropagationPolicy
		// targeting member1 using the cpu values already set in flinkDeploymentCRYAML,
		// and returns the corresponding ResourceBinding name.
		createFlinkDeployment := func() string {
			flinkName := fmt.Sprintf("flinkdeployment-%s", rand.String(RandomStrLength))

			flinkObj := &unstructured.Unstructured{}
			err := yaml.Unmarshal([]byte(flinkDeploymentCRYAML), flinkObj)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			flinkObj.SetNamespace(quotaNamespace)
			flinkObj.SetName(flinkName)
			_, err = dynamicClient.Resource(flinkDeploymentGVR).Namespace(quotaNamespace).
				Create(ctx, flinkObj, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				_ = dynamicClient.Resource(flinkDeploymentGVR).Namespace(quotaNamespace).
					Delete(ctx, flinkName, metav1.DeleteOptions{})
			})

			pp := helper.NewPropagationPolicy(quotaNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "flink.apache.org/v1beta1",
					Kind:       "FlinkDeployment",
					Name:       flinkName,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
					},
				})
			framework.CreatePropagationPolicy(karmadaClient, pp)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, quotaNamespace, pp.Name)
			})

			return names.GenerateBindingName("FlinkDeployment", flinkName)
		}

		ginkgo.By(fmt.Sprintf("creating %d FlinkDeployments that fit within the ResourceQuota", schedulableCount), func() {
			for range schedulableCount {
				bindingName := createFlinkDeployment()
				// Wait for successful scheduling before creating the next one so that each
				// workload is recorded as an assumed workload before the quota is re-evaluated.
				framework.WaitResourceBindingFitWith(karmadaClient, quotaNamespace, bindingName,
					func(binding *workv1alpha2.ResourceBinding) bool {
						cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
						return cond != nil && cond.Status == metav1.ConditionTrue
					})
			}
		})

		ginkgo.By("verifying the 7th FlinkDeployment is unschedulable because ResourceQuota is exhausted by assumed workloads", func() {
			bindingName := createFlinkDeployment()
			framework.WaitResourceBindingFitWith(karmadaClient, quotaNamespace, bindingName,
				func(binding *workv1alpha2.ResourceBinding) bool {
					cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
					return cond != nil && cond.Status == metav1.ConditionFalse &&
						cond.Reason == workv1alpha2.BindingReasonSchedulerError &&
						strings.Contains(cond.Message, "no enough resource")
				})
		})

		// At this point, 6 FlinkDeployments are in the assumption cache (6 × 150m = 900m assumed).
		// A single-template Deployment requesting 200m CPU would push the total to 1100m,
		// exceeding the 1000m ResourceQuota. This step verifies that the assumption cache also
		// protects against over-scheduling of single-template workloads.
		ginkgo.By("verifying a single-template Deployment requesting 200m CPU is also unschedulable due to assumed workloads", func() {
			assertSingleTemplateDeploymentUnschedulable(quotaNamespace, targetCluster, 200, nil)
		})
	})
})

var _ = framework.SerialDescribe("[EstimatorAssumption] NodeResource plugin assumption testing", ginkgo.Labels{NeedCreateCluster}, func() {
	var (
		targetCluster       string
		memberKubeClient    kubernetes.Interface
		memberDynamicClient dynamic.Interface
		flinkCRD            apiextensionsv1.CustomResourceDefinition
	)

	ginkgo.BeforeEach(func() {
		targetCluster = "member-e2e-" + rand.String(RandomStrLength)
		memberKubeConfigPath := filepath.Join(os.Getenv("HOME"), ".kube", targetCluster+".config")
		clusterContext := "kind-" + targetCluster
		controlPlane := targetCluster + "-control-plane"

		defaultConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		defaultConfigFlags.Context = &karmadaContext
		factory := cmdutil.NewFactory(defaultConfigFlags)

		ginkgo.DeferCleanup(func() {
			ginkgo.By(fmt.Sprintf("deleting dedicated member cluster: %s", targetCluster), func() {
				err := deleteCluster(targetCluster, memberKubeConfigPath)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = os.Remove(memberKubeConfigPath)
				if !os.IsNotExist(err) {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			})
		})
		ginkgo.By(fmt.Sprintf("creating dedicated member cluster: %s", targetCluster), func() {
			err := createCluster(targetCluster, memberKubeConfigPath, controlPlane, clusterContext)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.By("adding the estimator cluster name as a kubeconfig context", func() {
			err := addKubeconfigContextAlias(memberKubeConfigPath, clusterContext, targetCluster)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.DeferCleanup(func() {
			removeSchedulerEstimator(hostKubeClient, targetCluster)
		})
		ginkgo.By(fmt.Sprintf("deploying scheduler-estimator for cluster: %s", targetCluster), func() {
			err := deploySchedulerEstimator(kubeconfig, hostContext, memberKubeConfigPath, targetCluster)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			estimatorName := names.GenerateEstimatorDeploymentName(targetCluster)
			framework.WaitDeploymentGetByClientFitWith(hostKubeClient, names.NamespaceKarmadaSystem, estimatorName, func(deployment *appsv1.Deployment) bool {
				return framework.CheckDeploymentReadyStatus(deployment, 2)
			})
		})

		ginkgo.DeferCleanup(func() {
			ginkgo.By(fmt.Sprintf("unjoining dedicated member cluster: %s", targetCluster), func() {
				var cleanupErrors []error
				_, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), targetCluster, metav1.GetOptions{})
				switch {
				case err == nil:
					opts := unjoin.CommandUnjoinOption{
						DryRun:            false,
						ClusterNamespace:  options.DefaultKarmadaClusterNamespace,
						ClusterName:       targetCluster,
						ClusterContext:    targetCluster,
						ClusterKubeConfig: memberKubeConfigPath,
						Wait:              5 * options.DefaultKarmadactlCommandDuration,
					}
					if err = opts.Run(factory); err != nil {
						cleanupErrors = append(cleanupErrors, err)
					}
				case !apierrors.IsNotFound(err):
					cleanupErrors = append(cleanupErrors, err)
				}

				for _, secretName := range []string{targetCluster, names.GenerateImpersonationSecretName(targetCluster)} {
					if err = util.DeleteSecret(kubeClient, options.DefaultKarmadaClusterNamespace, secretName); err != nil {
						cleanupErrors = append(cleanupErrors, err)
					}
				}
				gomega.Expect(errors.Join(cleanupErrors...)).ShouldNot(gomega.HaveOccurred())
			})
		})
		ginkgo.By(fmt.Sprintf("joining dedicated member cluster: %s", targetCluster), func() {
			opts := join.CommandJoinOption{
				DryRun:            false,
				ClusterNamespace:  options.DefaultKarmadaClusterNamespace,
				ClusterName:       targetCluster,
				ClusterContext:    targetCluster,
				ClusterKubeConfig: memberKubeConfigPath,
			}
			err := opts.Run(factory)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		framework.WaitClusterFitWith(controlPlaneClient, targetCluster, func(cluster *clusterv1alpha1.Cluster) bool {
			return meta.IsStatusConditionPresentAndEqual(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
		})
		waitSchedulerEstimatorConnection(hostKubeClient, targetCluster)

		memberClusterClient, err := util.NewClusterClientSet(targetCluster, controlPlaneClient, nil)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(memberClusterClient.KubeClient).ShouldNot(gomega.BeNil())
		memberKubeClient = memberClusterClient.KubeClient
		framework.WaitNamespacePresentOnClusterByClient(memberKubeClient, testNamespace)

		dynamicClusterClient, err := util.NewClusterDynamicClientSet(targetCluster, controlPlaneClient, nil)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(dynamicClusterClient.DynamicClientSet).ShouldNot(gomega.BeNil())
		memberDynamicClient = dynamicClusterClient.DynamicClientSet
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("creating FlinkDeployment CRD on karmada control plane", func() {
			err := yaml.Unmarshal([]byte(flinkDeploymentCRDYAML), &flinkCRD)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			framework.CreateCRD(dynamicClient, &flinkCRD)
			framework.WaitCRDEstablished(dynamicClient, flinkCRD.Name)
			ginkgo.DeferCleanup(func() {
				framework.RemoveCRD(dynamicClient, flinkCRD.Name)
				framework.WaitCRDDisappeared(dynamicClient, flinkCRD.Name)
				waitCRDDisappearedOnCluster(memberDynamicClient, targetCluster, flinkCRD.Name)
				framework.WaitCRDDisappearedFromClusterStatus(karmadaClient, []string{targetCluster},
					fmt.Sprintf("%s/%s", flinkCRD.Spec.Group, "v1beta1"), flinkCRD.Spec.Names.Kind)
			})
		})
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By(fmt.Sprintf("propagating FlinkDeployment CRD to dedicated cluster: %s", targetCluster), func() {
			cpp := helper.NewClusterPropagationPolicy(cppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: flinkCRD.APIVersion,
					Kind:       flinkCRD.Kind,
					Name:       flinkCRD.Name,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
				})
			framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
			framework.WaitCRDPresentOnClusters(karmadaClient, []string{targetCluster},
				fmt.Sprintf("%s/%s", flinkCRD.Spec.Group, "v1beta1"), flinkCRD.Spec.Names.Kind)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, cpp.Name)
			})
		})
	})

	ginkgo.It("FlinkDeployment should be unschedulable when assumed workloads exhaust cluster resources", func(ctx context.Context) {
		targetNodeName, targetNodeHostname, availableMilliCPU := mostAvailableSchedulableNodeCPU(ctx, memberKubeClient, targetCluster)
		targetNodeSelector := map[string]string{corev1.LabelHostname: targetNodeHostname}
		const (
			// A FlinkDeployment reserves CPU for one JobManager and one TaskManager.
			flinkComponentsPerDeployment int64 = 2
			// Six scheduled deployments are enough to verify that assumed CPU accumulates while keeping the test bounded.
			targetSchedulableFlinkDeployments int64 = 6
			// Each component requests at least 50m CPU, even on a small node.
			minimumComponentMilliCPU int64 = 50
		)
		// Divide the node's available CPU across the target deployments and their two components.
		componentMilliCPU := max(minimumComponentMilliCPU,
			availableMilliCPU/(flinkComponentsPerDeployment*targetSchedulableFlinkDeployments))
		flinkDeploymentMilliCPU := flinkComponentsPerDeployment * componentMilliCPU
		gomega.Expect(availableMilliCPU).Should(gomega.BeNumerically(">", flinkDeploymentMilliCPU),
			"expected enough available CPU on node %q to schedule one FlinkDeployment", targetNodeName)
		componentCPU := float64(componentMilliCPU) / 1000

		// Create one more deployment than the node can fit, the final one must be unschedulable.
		maxFlinkCount := int(availableMilliCPU/flinkDeploymentMilliCPU) + 1

		ginkgo.By(fmt.Sprintf("targeting node %q with %dm available CPU, %dm per Flink component, and up to %d FlinkDeployments",
			targetNodeName, availableMilliCPU, componentMilliCPU, maxFlinkCount))

		// createFlinkDeployment creates a FlinkDeployment with the calculated CPU request,
		// pins it to the selected node, and returns its ResourceBinding name.
		createFlinkDeployment := func() string {
			flinkName := fmt.Sprintf("flinkdeployment-%s", rand.String(RandomStrLength))

			flinkObj := &unstructured.Unstructured{}
			err := yaml.Unmarshal([]byte(flinkDeploymentCRYAML), flinkObj)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			flinkObj.SetNamespace(testNamespace)
			flinkObj.SetName(flinkName)
			err = unstructured.SetNestedField(flinkObj.Object, componentCPU, "spec", "jobManager", "resource", "cpu")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			err = unstructured.SetNestedField(flinkObj.Object, componentCPU, "spec", "taskManager", "resource", "cpu")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			err = unstructured.SetNestedStringMap(flinkObj.Object, targetNodeSelector, "spec", "podTemplate", "spec", "nodeSelector")
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			_, err = dynamicClient.Resource(flinkDeploymentGVR).Namespace(testNamespace).
				Create(ctx, flinkObj, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				_ = dynamicClient.Resource(flinkDeploymentGVR).Namespace(testNamespace).
					Delete(ctx, flinkName, metav1.DeleteOptions{})
			})

			pp := helper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "flink.apache.org/v1beta1",
					Kind:       "FlinkDeployment",
					Name:       flinkName,
				}},
				policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{targetCluster},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
					},
				})
			framework.CreatePropagationPolicy(karmadaClient, pp)
			ginkgo.DeferCleanup(func() {
				framework.RemovePropagationPolicy(karmadaClient, testNamespace, pp.Name)
			})

			return names.GenerateBindingName("FlinkDeployment", flinkName)
		}

		ginkgo.By(fmt.Sprintf("creating FlinkDeployments one by one (up to %d) until assumption exhausts cluster resources", maxFlinkCount), func() {
			assumptionExhausted := false
			scheduledCount := 0
			for index := range maxFlinkCount {
				bindingName := createFlinkDeployment()
				// Wait for a definitive scheduling result before creating the next one,
				// ensuring the assumption is recorded before the next workload is evaluated.
				framework.WaitResourceBindingFitWith(karmadaClient, testNamespace, bindingName,
					func(binding *workv1alpha2.ResourceBinding) bool {
						cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
						if cond == nil {
							return false
						}
						if cond.Status == metav1.ConditionFalse && cond.Reason == workv1alpha2.BindingReasonSchedulerError &&
							strings.Contains(cond.Message, "no enough resource") {
							assumptionExhausted = true
							return true
						}
						if cond.Status == metav1.ConditionTrue {
							scheduledCount++
							return true
						}
						return false
					})
				if index == 0 {
					waitForSuccessfulMaxAvailableComponentSetsRequest(ctx, hostKubeClient, targetCluster)
				}
				if assumptionExhausted {
					break
				}
			}
			gomega.Expect(scheduledCount).Should(gomega.BeNumerically(">", 0),
				"expected the dedicated scheduler-estimator to schedule at least one FlinkDeployment before assumptions exhaust node resources")
			gomega.Expect(assumptionExhausted).Should(gomega.BeTrue(),
				"expected assumption to exhaust cluster resources within %d FlinkDeployments", maxFlinkCount)
		})

		// At this point the assumption cache has less than one FlinkDeployment's total CPU request
		// remaining on the dedicated cluster. A single-template Deployment with that total request should also fail
		// to schedule, verifying that the assumption cache protects against over-scheduling of
		// single-template workloads as well.
		ginkgo.By("verifying a single-template Deployment is also unschedulable due to assumed workloads", func() {
			assertSingleTemplateDeploymentUnschedulable(testNamespace, targetCluster, flinkDeploymentMilliCPU, targetNodeSelector)
		})
	})
})

// mostAvailableSchedulableNodeCPU finds the Ready, schedulable node with the most
// CPU remaining after subtracting requests from non-terminal pods. It returns the
// node's name, hostname, and remaining CPU in millicores.
func mostAvailableSchedulableNodeCPU(ctx context.Context, clusterClient kubernetes.Interface, cluster string) (string, string, int64) {
	nodeList, err := clusterClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	podList, err := clusterClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	requestedCPUByNode := make(map[string]int64)
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Spec.NodeName == "" || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		requestedCPUByNode[pod.Spec.NodeName] += util.EmptyResource().AddPodRequest(&pod.Spec).MilliCPU
	}

	var selectedNodeName, selectedHostname string
	var maxAvailableMilliCPU int64
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if !nodeSchedulableByDefault(node) {
			continue
		}
		hostname := node.Labels[corev1.LabelHostname]
		if hostname == "" {
			continue
		}
		availableMilliCPU := node.Status.Allocatable.Cpu().MilliValue() - requestedCPUByNode[node.Name]
		if availableMilliCPU > maxAvailableMilliCPU {
			selectedNodeName = node.Name
			selectedHostname = hostname
			maxAvailableMilliCPU = availableMilliCPU
		}
	}

	gomega.Expect(selectedNodeName).ShouldNot(gomega.BeEmpty(), "expected at least one schedulable node in cluster %q", cluster)
	gomega.Expect(maxAvailableMilliCPU).Should(gomega.BeNumerically(">", 0), "expected positive available CPU on node %q", selectedNodeName)
	return selectedNodeName, selectedHostname, maxAvailableMilliCPU
}

func nodeSchedulableByDefault(node *corev1.Node) bool {
	if node.Spec.Unschedulable {
		return false
	}
	ready := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			ready = cond.Status == corev1.ConditionTrue
			break
		}
	}
	if !ready {
		return false
	}
	for _, taint := range node.Spec.Taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			return false
		}
	}
	return true
}

func waitSchedulerEstimatorConnection(client kubernetes.Interface, clusterName string) {
	expectedLog := fmt.Sprintf("of cluster(%s) has been established.", clusterName)
	ginkgo.By(fmt.Sprintf("waiting for scheduler to connect to the estimator for cluster: %s", clusterName), func() {
		gomega.Eventually(func() (bool, error) {
			pods, err := podsForDeployment(context.TODO(), client, names.NamespaceKarmadaSystem, names.KarmadaSchedulerComponentName)
			if err != nil {
				return false, err
			}
			var logErrors []error
			for i := range pods.Items {
				logs, err := podLogs(context.TODO(), client, names.NamespaceKarmadaSystem, pods.Items[i].Name)
				if err != nil {
					logErrors = append(logErrors, err)
					continue
				}
				if strings.Contains(logs, expectedLog) {
					return true, nil
				}
			}
			if len(logErrors) == len(pods.Items) {
				return false, errors.Join(logErrors...)
			}
			return false, nil
		}, pollTimeout, pollInterval).Should(gomega.BeTrue(),
			"expected scheduler to establish the estimator connection for cluster %q", clusterName)
	})
}

func waitForSuccessfulMaxAvailableComponentSetsRequest(ctx context.Context, client kubernetes.Interface, clusterName string) {
	ginkgo.By(fmt.Sprintf("verifying scheduler used the estimator for cluster: %s", clusterName), func() {
		gomega.Eventually(func() (bool, error) {
			estimatorName := names.GenerateEstimatorDeploymentName(clusterName)
			pods, err := podsForDeployment(ctx, client, names.NamespaceKarmadaSystem, estimatorName)
			if err != nil {
				return false, err
			}

			var metricsErrors []error
			for i := range pods.Items {
				output, err := framework.GetMetricsFromPod(ctx, client, pods.Items[i].Name, names.NamespaceKarmadaSystem, 8080)
				if err != nil {
					metricsErrors = append(metricsErrors, err)
					continue
				}
				podMetrics := testutil.Metrics{}
				if err = testutil.ParseMetrics(output, &podMetrics); err != nil {
					metricsErrors = append(metricsErrors, err)
					continue
				}
				for _, sample := range podMetrics["karmada_scheduler_estimator_estimating_request_total"] {
					if sample.Metric[testutil.LabelName("result")] == testutil.LabelValue("success") &&
						sample.Metric[testutil.LabelName("type")] == testutil.LabelValue("MaxAvailableComponentSets") &&
						sample.Value > 0 {
						return true, nil
					}
				}
			}
			if len(metricsErrors) == len(pods.Items) {
				return false, errors.Join(metricsErrors...)
			}
			return false, nil
		}, pollTimeout, pollInterval).Should(gomega.BeTrue(),
			"expected a successful MaxAvailableComponentSets request on the estimator for cluster %q", clusterName)
	})
}

func podsForDeployment(ctx context.Context, client kubernetes.Interface, namespace, deploymentName string) (*corev1.PodList, error) {
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
}

func addKubeconfigContextAlias(kubeConfigPath, sourceContext, alias string) error {
	config, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return err
	}

	contextConfig, exists := config.Contexts[sourceContext]
	if !exists {
		return fmt.Errorf("context %q not found in kubeconfig %q", sourceContext, kubeConfigPath)
	}
	contextCopy := *contextConfig
	config.Contexts[alias] = &contextCopy
	config.CurrentContext = alias
	return clientcmd.WriteToFile(*config, kubeConfigPath)
}

func deploySchedulerEstimator(hostKubeConfig, hostClusterContext, memberKubeConfig, memberClusterName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel()

	// The command is a fixed repository script; all values from the E2E setup are passed as arguments without shell expansion.
	cmd := exec.CommandContext(ctx, "../../../../hack/deploy-scheduler-estimator.sh", hostKubeConfig, hostClusterContext, memberKubeConfig, memberClusterName) //nolint:gosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deploy scheduler-estimator for cluster %q: %w: %s", memberClusterName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func removeSchedulerEstimator(client kubernetes.Interface, clusterName string) {
	estimatorName := names.GenerateEstimatorDeploymentName(clusterName)
	ginkgo.By(fmt.Sprintf("removing scheduler-estimator for cluster: %s", clusterName), func() {
		err := client.AppsV1().Deployments(names.NamespaceKarmadaSystem).Delete(context.TODO(), estimatorName, metav1.DeleteOptions{})
		gomega.Expect(ignoreNotFound(err)).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			_, err := client.AppsV1().Deployments(names.NamespaceKarmadaSystem).Get(context.TODO(), estimatorName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, pollTimeout, pollInterval).Should(gomega.BeTrue())

		err = client.CoreV1().Services(names.NamespaceKarmadaSystem).Delete(context.TODO(), estimatorName, metav1.DeleteOptions{})
		gomega.Expect(ignoreNotFound(err)).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			_, err := client.CoreV1().Services(names.NamespaceKarmadaSystem).Get(context.TODO(), estimatorName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, pollTimeout, pollInterval).Should(gomega.BeTrue())

		err = client.CoreV1().Secrets(names.NamespaceKarmadaSystem).Delete(context.TODO(), clusterName+"-kubeconfig", metav1.DeleteOptions{})
		gomega.Expect(ignoreNotFound(err)).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(func() bool {
			_, err := client.CoreV1().Secrets(names.NamespaceKarmadaSystem).Get(context.TODO(), clusterName+"-kubeconfig", metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, pollTimeout, pollInterval).Should(gomega.BeTrue())
	})
}

func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func waitCRDDisappearedOnCluster(client dynamic.Interface, clusterName, crdName string) {
	ginkgo.By(fmt.Sprintf("waiting for CRD(%s) to disappear on cluster(%s)", crdName, clusterName), func() {
		crdGVR := apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")
		gomega.Eventually(func() error {
			_, err := client.Resource(crdGVR).Get(context.TODO(), crdName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("CRD %q still exists on cluster %q", crdName, clusterName)
		}, pollTimeout, pollInterval).Should(gomega.Succeed())
	})
}

// assertSingleTemplateDeploymentUnschedulable creates a single-replica Deployment requesting
// cpuRequestMilliCPU in the given namespace, propagates it to targetCluster, and asserts that the
// ResourceBinding transitions to an unschedulable state because the assumption cache has
// already exhausted the available resources.
func assertSingleTemplateDeploymentUnschedulable(namespace, targetCluster string, cpuRequestMilliCPU int64, nodeSelector map[string]string) {
	cpuRequest := fmt.Sprintf("%dm", cpuRequestMilliCPU)
	deployName := fmt.Sprintf("deploy-%s", rand.String(RandomStrLength))
	deploy := helper.NewDeployment(namespace, deployName)
	deploy.Spec.Replicas = ptr.To[int32](1)
	deploy.Spec.Template.Spec.NodeSelector = nodeSelector
	deploy.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse(cpuRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse(cpuRequest),
		},
	}
	framework.CreateDeployment(kubeClient, deploy)
	ginkgo.DeferCleanup(func() {
		framework.RemoveDeployment(kubeClient, namespace, deployName)
	})

	pp := helper.NewPropagationPolicy(namespace, ppNamePrefix+rand.String(RandomStrLength),
		[]policyv1alpha1.ResourceSelector{{
			APIVersion: deploy.APIVersion,
			Kind:       deploy.Kind,
			Name:       deployName,
		}},
		policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetCluster},
			},
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MaxGroups:     1,
					MinGroups:     1,
				},
			},
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
		})
	framework.CreatePropagationPolicy(karmadaClient, pp)
	ginkgo.DeferCleanup(func() {
		framework.RemovePropagationPolicy(karmadaClient, namespace, pp.Name)
	})

	bindingName := names.GenerateBindingName(util.DeploymentKind, deployName)
	framework.WaitResourceBindingFitWith(karmadaClient, namespace, bindingName,
		func(binding *workv1alpha2.ResourceBinding) bool {
			cond := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
			return cond != nil && cond.Status == metav1.ConditionFalse &&
				cond.Reason == workv1alpha2.BindingReasonSchedulerError &&
				strings.Contains(cond.Message, "no enough resource")
		})
}
