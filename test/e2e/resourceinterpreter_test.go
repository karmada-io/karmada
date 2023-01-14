package e2e

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Resource interpreter webhook testing", func() {
	var policyNamespace, policyName string
	var workloadNamespace, workloadName string
	var workload *workloadv1alpha1.Workload
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName = workloadNamePrefix + rand.String(RandomStrLength)
		workloadNamespace = testNamespace
		workloadName = policyName

		workload = testhelper.NewWorkload(workloadNamespace, workloadName)
		policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: workload.APIVersion,
				Kind:       workload.Kind,
				Name:       workload.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateWorkload(dynamicClient, workload)
		ginkgo.DeferCleanup(func() {
			framework.RemoveWorkload(dynamicClient, workload.Namespace, workload.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("InterpreterOperation InterpretReplica testing", func() {
		ginkgo.It("InterpretReplica testing", func() {
			ginkgo.By("check if workload's replica is interpreted", func() {
				resourceBindingName := names.GenerateBindingName(workload.Kind, workload.Name)
				expectedReplicas := *workload.Spec.Replicas

				gomega.Eventually(func(g gomega.Gomega) (int32, error) {
					resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(workload.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof("ResourceBinding(%s/%s)'s replicas is %d, expected: %d.",
						resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.Replicas, expectedReplicas)
					return resourceBinding.Spec.Replicas, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
			})
		})
	})

	ginkgo.Context("InterpreterOperation Retain testing", func() {
		ginkgo.It("Retain testing", func() {
			ginkgo.By("wait workload exist on the member clusters", func() {
				for _, clusterName := range framework.ClusterNames() {
					framework.WaitWorkloadPresentOnClusterFitWith(clusterName, workload.Namespace, workload.Name,
						func(_ *workloadv1alpha1.Workload) bool {
							return true
						})
				}
			})

			ginkgo.By("update workload on the control plane", func() {
				gomega.Eventually(func(g gomega.Gomega) error {
					curWorkload := framework.GetWorkload(dynamicClient, workloadNamespace, workloadName)
					// construct two values that need to be changed, and only one value is retained.
					curWorkload.Spec.Replicas = pointer.Int32Ptr(2)
					curWorkload.Spec.Paused = true

					newUnstructuredObj, err := helper.ToUnstructured(curWorkload)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())

					workloadGVR := workloadv1alpha1.SchemeGroupVersion.WithResource("workloads")
					_, err = dynamicClient.Resource(workloadGVR).Namespace(curWorkload.Namespace).Update(context.TODO(), newUnstructuredObj, metav1.UpdateOptions{})
					return err
				}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("check if workload's spec.paused is retained", func() {
				framework.WaitWorkloadPresentOnClustersFitWith(framework.ClusterNames(), workload.Namespace, workload.Name,
					func(workload *workloadv1alpha1.Workload) bool {
						return *workload.Spec.Replicas == 2 && !workload.Spec.Paused
					})
			})
		})
	})

	ginkgo.Context("InterpreterOperation ReviseReplica testing", func() {
		ginkgo.BeforeEach(func() {
			sumWeight := 0
			staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
			for index, clusterName := range framework.ClusterNames() {
				staticWeightList := policyv1alpha1.StaticClusterWeight{
					TargetCluster: policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{clusterName},
					},
					Weight: int64(index + 1),
				}
				sumWeight += index + 1
				staticWeightLists = append(staticWeightLists, staticWeightList)
			}
			workload.Spec.Replicas = pointer.Int32Ptr(int32(sumWeight))
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: workload.APIVersion,
					Kind:       workload.Kind,
					Name:       workload.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: staticWeightLists,
					},
				},
			})
		})

		ginkgo.It("ReviseReplica testing", func() {
			for index, clusterName := range framework.ClusterNames() {
				framework.WaitWorkloadPresentOnClusterFitWith(clusterName, workload.Namespace, workload.Name, func(workload *workloadv1alpha1.Workload) bool {
					return *workload.Spec.Replicas == int32(index+1)
				})
			}
		})
	})

	ginkgo.Context("InterpreterOperation AggregateStatus testing", func() {
		ginkgo.It("AggregateStatus testing", func() {
			ginkgo.By("check whether the workload status can be correctly collected", func() {
				// Simulate the workload resource controller behavior, update the status information of workload resources of member clusters manually.
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Status.ReadyReplicas = *workload.Spec.Replicas
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")
				}

				wantedReplicas := *workload.Spec.Replicas * int32(len(framework.Clusters()))
				klog.Infof("Waiting for workload(%s/%s) collecting correctly status", workloadNamespace, workloadName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentWorkload := framework.GetWorkload(dynamicClient, workloadNamespace, workloadName)

					klog.Infof("workload(%s/%s) readyReplicas: %d, wanted replicas: %d", workloadNamespace, workloadName, currentWorkload.Status.ReadyReplicas, wantedReplicas)
					if currentWorkload.Status.ReadyReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("InterpreterOperation InterpretStatus testing", func() {
		ginkgo.It("InterpretStatus testing", func() {
			for _, cluster := range framework.ClusterNames() {
				clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
				gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

				memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
				memberWorkload.Status.ReadyReplicas = *workload.Spec.Replicas
				framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")

				workName := names.GenerateWorkName(workload.Kind, workload.Name, workload.Namespace)
				workNamespace := names.GenerateExecutionSpaceName(cluster)

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					work, err := karmadaClient.WorkV1alpha1().Works(workNamespace).Get(context.TODO(), workName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if len(work.Status.ManifestStatuses) == 0 || work.Status.ManifestStatuses[0].Status == nil {
						return false, nil
					}

					var observedStatus workloadv1alpha1.WorkloadStatus
					err = json.Unmarshal(work.Status.ManifestStatuses[0].Status.Raw, &observedStatus)
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof("work(%s/%s) readyReplicas: %d, want: %d", workNamespace, workName, observedStatus.ReadyReplicas, *workload.Spec.Replicas)

					// not collect status.conditions in webhook
					klog.Infof("work(%s/%s) length of conditions: %v, want: %v", workNamespace, workName, len(observedStatus.Conditions), 0)

					if observedStatus.ReadyReplicas == *workload.Spec.Replicas && len(observedStatus.Conditions) == 0 {
						return true, nil
					}
					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			}
		})
	})

	ginkgo.Context("InterpreterOperation InterpretHealth testing", func() {
		ginkgo.It("InterpretHealth testing", func() {
			resourceBindingName := names.GenerateBindingName(workload.Kind, workload.Name)

			SetReadyReplicas := func(readyReplicas int32) {
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())
					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Status.ReadyReplicas = readyReplicas
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")
				}
			}

			CheckResult := func(result workv1alpha2.ResourceHealth) interface{} {
				return func(g gomega.Gomega) (bool, error) {
					rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(workload.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if len(rb.Status.AggregatedStatus) != len(framework.ClusterNames()) {
						return false, nil
					}

					for _, status := range rb.Status.AggregatedStatus {
						klog.Infof("resourceBinding(%s/%s) on cluster %s got %s, want %s ", workload.Namespace, resourceBindingName, status.ClusterName, status.Health, result)
						if status.Health != result {
							return false, nil
						}
					}
					return true, nil
				}
			}

			ginkgo.By("workload healthy", func() {
				SetReadyReplicas(*workload.Spec.Replicas)
				gomega.Eventually(CheckResult(workv1alpha2.ResourceHealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
			})

			ginkgo.By("workload unhealthy", func() {
				SetReadyReplicas(1)
				gomega.Eventually(CheckResult(workv1alpha2.ResourceUnhealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
			})
		})
	})

	ginkgo.Context("InterpreterOperation InterpretDependency testing", func() {
		var configMapNamespace, configMapName string

		ginkgo.BeforeEach(func() {
			configMapNamespace = testNamespace
			configMapName = configMapNamePrefix + rand.String(RandomStrLength)

			workload.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				}}}
			// configmaps should be propagated automatically.
			policy.Spec.PropagateDeps = true

			cm := testhelper.NewConfigMap(configMapNamespace, configMapName, map[string]string{"RUN_ENV": "test"})

			framework.CreateConfigMap(kubeClient, cm)
			ginkgo.DeferCleanup(func() {
				framework.RemoveConfigMap(kubeClient, configMapNamespace, configMapName)
			})
		})

		ginkgo.It("InterpretDependency testing", func() {
			ginkgo.By("check if workload's dependency is interpreted", func() {
				clusterNames := framework.ClusterNames()
				gomega.Eventually(func(g gomega.Gomega) (int, error) {
					var configmapNum int
					for _, clusterName := range clusterNames {
						clusterClient := framework.GetClusterClient(clusterName)
						gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
						if _, err := clusterClient.CoreV1().ConfigMaps(configMapNamespace).Get(context.TODO(), configMapName, metav1.GetOptions{}); err != nil {
							if apierrors.IsNotFound(err) {
								continue
							}
							g.Expect(err).NotTo(gomega.HaveOccurred())
						}
						configmapNum++
					}
					return configmapNum, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(len(clusterNames)))
			})
		})
	})
})

var _ = framework.SerialDescribe("Resource interpreter customization testing", func() {
	var customization *configv1alpha1.ResourceInterpreterCustomization
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	// We only need to test any one of the member clusters.
	var targetCluster string

	ginkgo.When("Apply single ResourceInterpreterCustomization without DependencyInterpretation operation", func() {
		ginkgo.BeforeEach(func() {
			targetCluster = framework.ClusterNames()[rand.Intn(len(framework.ClusterNames()))]
			deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			policy = testhelper.NewPropagationPolicy(testNamespace, deployment.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetCluster},
				},
			})
		})

		ginkgo.JustBeforeEach(func() {
			framework.CreateResourceInterpreterCustomization(karmadaClient, customization)
			// Wait for resource interpreter informer synced.
			time.Sleep(time.Second)

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				framework.DeleteResourceInterpreterCustomization(karmadaClient, customization.Name)
			})
		})

		ginkgo.Context("InterpreterOperation InterpretReplica testing", func() {
			ginkgo.BeforeEach(func() {
				customization = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					configv1alpha1.CustomizationRules{
						ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{
							LuaScript: `
	function GetReplicas(desiredObj)
	  replica = desiredObj.spec.replicas + 1
	  requirement = {}
	  requirement.nodeClaim = {}
	  requirement.nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector
	  requirement.nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations
	  requirement.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits
	  return replica, requirement
	end`,
						},
					})
			})

			ginkgo.It("InterpretReplica testing", func() {
				ginkgo.By("check if workload's replica is interpreted", func() {
					resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
					// Just for the current test case to distinguish the build-in logic.
					expectedReplicas := *deployment.Spec.Replicas + 1
					expectedReplicaRequirements := &workv1alpha2.ReplicaRequirements{
						ResourceRequest: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU: resource.MustParse("100m"),
						}}

					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(deployment.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())

						klog.Infof("ResourceBinding(%s/%s)'s replicas is %d, expected: %d.",
							resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.Replicas, expectedReplicas)
						if resourceBinding.Spec.Replicas != expectedReplicas {
							return false, nil
						}

						klog.Infof("ResourceBinding(%s/%s)'s replicaRequirements is %+v, expected: %+v.",
							resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.ReplicaRequirements, expectedReplicaRequirements)
						return reflect.DeepEqual(resourceBinding.Spec.ReplicaRequirements, expectedReplicaRequirements), nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))
				})
			})
		})

		ginkgo.Context("InterpreterOperation ReviseReplica testing", func() {
			ginkgo.BeforeEach(func() {
				customization = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					configv1alpha1.CustomizationRules{
						ReplicaRevision: &configv1alpha1.ReplicaRevision{
							LuaScript: `
	function ReviseReplica(obj, desiredReplica)
	  obj.spec.replicas = desiredReplica + 1
	  return obj
	end`,
						},
					})
			})

			ginkgo.BeforeEach(func() {
				sumWeight := 0
				staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
				for index, clusterName := range framework.ClusterNames() {
					staticWeightList := policyv1alpha1.StaticClusterWeight{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{clusterName},
						},
						Weight: int64(index + 1),
					}
					sumWeight += index + 1
					staticWeightLists = append(staticWeightLists, staticWeightList)
				}
				deployment.Spec.Replicas = pointer.Int32Ptr(int32(sumWeight))
				policy.Spec.Placement = policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: framework.ClusterNames(),
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: staticWeightLists,
						},
					},
				}
			})

			ginkgo.It("ReviseReplica testing", func() {
				for index, clusterName := range framework.ClusterNames() {
					framework.WaitDeploymentPresentOnClusterFitWith(clusterName, deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool {
						return *deployment.Spec.Replicas == int32(index+1)+1
					})
				}
			})
		})

		ginkgo.Context("InterpreterOperation Retain testing", func() {
			ginkgo.BeforeEach(func() {
				customization = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					configv1alpha1.CustomizationRules{
						Retention: &configv1alpha1.LocalValueRetention{
							LuaScript: `
	function Retain(desiredObj, observedObj)
	  desiredObj.spec.paused = observedObj.spec.paused
	  return desiredObj
	end`,
						},
					})
			})

			ginkgo.It("Retain testing", func() {
				ginkgo.By("wait deployment exist on the member clusters", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deployment.Namespace, deployment.Name,
						func(_ *appsv1.Deployment) bool {
							return true
						})
				})

				ginkgo.By("update deployment on the control plane", func() {
					// construct two values that need to be changed, and only one value is retained.
					framework.UpdateDeploymentPaused(kubeClient, deployment, true)
					framework.UpdateDeploymentReplicas(kubeClient, deployment, 2)
				})

				ginkgo.By("check if deployment's spec.paused is retained", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deployment.Namespace, deployment.Name,
						func(deployment *appsv1.Deployment) bool {
							return *deployment.Spec.Replicas == 2 && !deployment.Spec.Paused
						})
				})
			})
		})

		ginkgo.Context("InterpreterOperation AggregateStatus testing", func() {
			ginkgo.BeforeEach(func() {
				customization = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					configv1alpha1.CustomizationRules{
						StatusAggregation: &configv1alpha1.StatusAggregation{
							LuaScript: `
	function AggregateStatus(desiredObj, statusItems)
		if statusItems == nil then
			return desiredObj
		end
		if desiredObj.status == nil then
			desiredObj.status = {}
		end
		replicas = 0
		for i = 1, #statusItems do
			if statusItems[i].status ~= nil and statusItems[i].status.replicas ~= nil  then
				replicas = replicas + statusItems[i].status.replicas + 1
			end 
		end
		desiredObj.status.replicas = replicas
		return desiredObj
	end`,
						},
					})
			})
			ginkgo.It("AggregateStatus testing", func() {
				ginkgo.By("check whether the deployment status can be correctly collected", func() {
					// Only in the current case, a special example is constructed to distinguish the build-in logic.
					wantedReplicas := *deployment.Spec.Replicas + 1
					gomega.Eventually(func() bool {
						var currentDeployment *appsv1.Deployment
						framework.WaitDeploymentGetByClientFitWith(kubeClient, deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool {
							currentDeployment = deployment
							return true
						})
						klog.Infof("deployment(%s/%s) replicas: %d, wanted replicas: %d", deployment.Namespace, deployment.Name, currentDeployment.Status.Replicas, wantedReplicas)
						return currentDeployment.Status.Replicas == wantedReplicas
					}, pollTimeout, pollInterval).Should(gomega.BeTrue())
				})
			})
		})

		ginkgo.Context("InterpreterOperation InterpretStatus testing", func() {
			ginkgo.BeforeEach(func() {
				customization = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					configv1alpha1.CustomizationRules{
						StatusReflection: &configv1alpha1.StatusReflection{
							LuaScript: `
	function ReflectStatus (observedObj)
		if observedObj.status == nil then
			return nil
		end
		return observedObj.status
	end`,
						},
					})
			})
			ginkgo.It("InterpretStatus testing", func() {
				gomega.Eventually(func(g gomega.Gomega) bool {
					deploy, err := kubeClient.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())

			})
		})

		ginkgo.Context("InterpreterOperation InterpretHealth testing", func() {
			ginkgo.BeforeEach(func() {
				customization = testhelper.NewResourceInterpreterCustomization(
					"interpreter-customization"+rand.String(RandomStrLength),
					configv1alpha1.CustomizationTarget{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					configv1alpha1.CustomizationRules{
						HealthInterpretation: &configv1alpha1.HealthInterpretation{
							LuaScript: `
	function InterpretHealth(observedObj)
		return observedObj.status.readyReplicas == observedObj.spec.replicas
	end `,
						},
					})
			})
			ginkgo.It("InterpretHealth testing", func() {
				resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
				SetReadyReplicas := func(readyReplicas int32) {
					clusterClient := framework.GetClusterClient(targetCluster)
					gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
					var memberDeployment *appsv1.Deployment
					framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deployment.Namespace, deployment.Name,
						func(deployment *appsv1.Deployment) bool {
							memberDeployment = deployment
							return true
						})
					memberDeployment.Status.ReadyReplicas = readyReplicas
					framework.UpdateDeploymentStatus(clusterClient, memberDeployment)
				}

				CheckResult := func(result workv1alpha2.ResourceHealth) interface{} {
					return func(g gomega.Gomega) (bool, error) {
						rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(deployment.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
						g.Expect(err).NotTo(gomega.HaveOccurred())
						if len(rb.Status.AggregatedStatus) != 1 {
							return false, nil
						}
						for _, status := range rb.Status.AggregatedStatus {
							klog.Infof("resourceBinding(%s/%s) on cluster %s got %s, want %s ", deployment.Namespace, resourceBindingName, status.ClusterName, status.Health, result)
							if status.Health != result {
								return false, nil
							}
						}
						return true, nil
					}
				}

				ginkgo.By("deployment healthy", func() {
					SetReadyReplicas(*deployment.Spec.Replicas)
					gomega.Eventually(CheckResult(workv1alpha2.ResourceHealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
				})
			})
		})
	})

	ginkgo.When("Apply multi ResourceInterpreterCustomization with DependencyInterpretation operation", func() {
		var customizationAnother *configv1alpha1.ResourceInterpreterCustomization

		var configMapName string
		var configMap *corev1.ConfigMap

		var saName string
		var sa *corev1.ServiceAccount

		ginkgo.BeforeEach(func() {
			targetCluster = framework.ClusterNames()[rand.Intn(len(framework.ClusterNames()))]

			deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))

			configMapName = configMapNamePrefix + rand.String(RandomStrLength)
			configMap = testhelper.NewConfigMap(testNamespace, configMapName, map[string]string{"user": "karmada"})
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{{
				Name: "vol-configmap",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						}}}}}

			saName = saNamePrefix + rand.String(RandomStrLength)
			sa = testhelper.NewServiceaccount(testNamespace, saName)
			deployment.Spec.Template.Spec.ServiceAccountName = saName

			policy = testhelper.NewPropagationPolicy(testNamespace, deployment.Name, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{targetCluster},
				},
			})
			policy.Spec.PropagateDeps = true

			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					DependencyInterpretation: &configv1alpha1.DependencyInterpretation{
						LuaScript: `
function GetDependencies(desiredObj)
	dependentSas = {}
	refs = {}
	if desiredObj.spec.template.spec.serviceAccountName ~= '' and desiredObj.spec.template.spec.serviceAccountName ~= 'default' then
		dependentSas[desiredObj.spec.template.spec.serviceAccountName] = true
	end
	local idx = 1
	for key, value in pairs(dependentSas) do    
		dependObj = {}    
		dependObj.apiVersion = 'v1'   
		dependObj.kind = 'ServiceAccount'    
		dependObj.name = key    
		dependObj.namespace = desiredObj.metadata.namespace
		refs[idx] = dependObj    
		idx = idx + 1
	end
	return refs
end `,
					},
				})

			customizationAnother = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					DependencyInterpretation: &configv1alpha1.DependencyInterpretation{
						LuaScript: `
function GetDependencies(desiredObj)
	dependentSas = {}
	refs = {}
	if desiredObj.spec.template.spec.volumes == nil then
		return refs
	end
	local idx = 1
	for index, volume in pairs(desiredObj.spec.template.spec.volumes) do
		if volume.configMap ~= nil then
			dependObj = {}
			dependObj.apiVersion = 'v1'
			dependObj.kind = 'ConfigMap'
			dependObj.name = volume.configMap.name
			dependObj.namespace = desiredObj.metadata.namespace
			refs[idx] = dependObj
			idx = idx + 1
		end
	end
	return refs
end `,
					},
				})
		})

		ginkgo.JustBeforeEach(func() {
			framework.CreateResourceInterpreterCustomization(karmadaClient, customization)
			framework.CreateResourceInterpreterCustomization(karmadaClient, customizationAnother)
			// Wait for resource interpreter informer synced.
			time.Sleep(time.Second)

			framework.CreateServiceAccount(kubeClient, sa)
			framework.CreateConfigMap(kubeClient, configMap)

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, deployment)

			ginkgo.DeferCleanup(func() {
				framework.RemoveServiceAccount(kubeClient, sa.Namespace, sa.Name)
				framework.RemoveConfigMap(kubeClient, configMap.Namespace, configMap.Name)

				framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)

				framework.DeleteResourceInterpreterCustomization(karmadaClient, customization.Name)
				framework.DeleteResourceInterpreterCustomization(karmadaClient, customizationAnother.Name)
			})
		})

		ginkgo.Context("InterpreterOperation DependencyInterpretation testing", func() {
			ginkgo.It("DependencyInterpretation testing", func() {
				ginkgo.By("check if the resources is propagated automatically", func() {
					framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, deployment.Namespace, deployment.Name,
						func(deployment *appsv1.Deployment) bool {
							return true
						})

					framework.WaitServiceAccountPresentOnClusterFitWith(targetCluster, sa.Namespace, sa.Name,
						func(sa *corev1.ServiceAccount) bool {
							return true
						})

					framework.WaitConfigMapPresentOnClusterFitWith(targetCluster, configMap.Namespace, configMap.Name,
						func(configmap *corev1.ConfigMap) bool {
							return true
						})
				})
			})

		})
	})
})
