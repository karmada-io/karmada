/*
Copyright 2024 The Karmada Authors.

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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("testing for FederatedHPA and metrics-adapter", func() {
	var namespace string
	var deploymentName, serviceName, cppName, federatedHPAName, pressureJobName string
	var deployment *appsv1.Deployment
	var service *corev1.Service
	var cpp, jobCPP *policyv1alpha1.ClusterPropagationPolicy
	var federatedHPA *autoscalingv1alpha1.FederatedHPA
	var pressureJob *batchv1.Job
	var targetClusters []string
	var clusterNum int32

	ginkgo.BeforeEach(func() {
		namespace = testNamespace
		randomStr := rand.String(RandomStrLength)
		deploymentName = deploymentNamePrefix + randomStr
		serviceName = serviceNamePrefix + randomStr
		cppName = cppNamePrefix + randomStr
		federatedHPAName = federatedHPANamePrefix + randomStr
		pressureJobName = jobNamePrefix + randomStr
		targetClusters = framework.ClusterNames()
		clusterNum = int32(len(targetClusters))

		deployment = testhelper.NewDeployment(namespace, deploymentName)
		deployment.Spec.Selector.MatchLabels = map[string]string{"app": deploymentName}
		deployment.Spec.Template.Labels = map[string]string{"app": deploymentName}
		deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("10Mi"),
			}}
		service = testhelper.NewService(namespace, serviceName, corev1.ServiceTypeNodePort)
		service.Spec.Selector = map[string]string{"app": deploymentName}
		cpp = testhelper.NewClusterPropagationPolicy(cppName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deploymentName,
			},
			{
				APIVersion: service.APIVersion,
				Kind:       service.Kind,
				Name:       serviceName,
			},
		}, policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: targetClusters,
			},
		})
		federatedHPA = testhelper.NewFederatedHPA(namespace, federatedHPAName, deploymentName)
		federatedHPA.Spec.MaxReplicas = 6
		federatedHPA.Spec.Behavior.ScaleUp = &autoscalingv2.HPAScalingRules{StabilizationWindowSeconds: ptr.To[int32](3)}
		federatedHPA.Spec.Behavior.ScaleDown = &autoscalingv2.HPAScalingRules{StabilizationWindowSeconds: ptr.To[int32](3)}
		federatedHPA.Spec.Metrics[0].Resource.Target.AverageUtilization = ptr.To[int32](10)

		pressureJob = testhelper.NewJob(namespace, pressureJobName)
		pressureJob.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
		pressureJob.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:    pressureJobName,
				Image:   "alpine:3.19.1",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "apk add curl; while true; do for i in `seq 200`; do curl http://" + serviceName + "." + namespace + ":80; done; sleep 1; done"},
			},
		}
		// pressure job always use Duplicated schedule type to prevent certain member cluster from missing it.
		jobCPP = testhelper.NewClusterPropagationPolicy(pressureJobName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: pressureJob.APIVersion,
				Kind:       pressureJob.Kind,
				Name:       pressureJobName,
			},
		}, policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: targetClusters,
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, cpp)
		framework.CreateDeployment(kubeClient, deployment)
		framework.CreateService(kubeClient, service)
		framework.CreateFederatedHPA(karmadaClient, federatedHPA)

		ginkgo.DeferCleanup(func() {
			framework.RemoveFederatedHPA(karmadaClient, namespace, federatedHPAName)
			framework.RemoveService(kubeClient, namespace, serviceName)
			framework.RemoveDeployment(kubeClient, namespace, deploymentName)
			framework.RemoveClusterPropagationPolicy(karmadaClient, cppName)
		})
	})

	// runPressureJob run a job to pressure the deployment to increase its cpu/mem.
	var runPressureJob = func() {
		framework.CreateClusterPropagationPolicy(karmadaClient, jobCPP)
		framework.CreateJob(kubeClient, pressureJob)
		framework.WaitJobPresentOnClustersFitWith(targetClusters, namespace, pressureJobName, func(_ *batchv1.Job) bool { return true })
		ginkgo.DeferCleanup(func() {
			framework.RemoveJob(kubeClient, namespace, pressureJobName)
			framework.RemoveClusterPropagationPolicy(karmadaClient, pressureJobName)
		})
	}

	// 1. duplicated scheduling
	ginkgo.Context("FederatedHPA scale Deployment in Duplicated schedule type", func() {
		ginkgo.BeforeEach(func() {
			deployment.Spec.Replicas = ptr.To[int32](1)
		})

		ginkgo.It("do scale when metrics of cpu/mem utilization up in Duplicated schedule type", func() {
			ginkgo.By("step1: check initial replicas result should equal to cluster number", func() {
				framework.WaitDeploymentStatus(kubeClient, deployment, clusterNum)
			})

			ginkgo.By("step2: pressure test the deployment of member clusters to increase the cpu/mem", runPressureJob)

			ginkgo.By("step3: check final replicas result should greater than deployment initial replicas", func() {
				framework.WaitDeploymentFitWith(kubeClient, namespace, deploymentName, func(deploy *appsv1.Deployment) bool {
					return *deploy.Spec.Replicas > 1
				})
			})
		})
	})

	// 2. static weight scheduling
	ginkgo.Context("FederatedHPA scale Deployment in Static Weight schedule type", func() {
		ginkgo.BeforeEach(func() {
			// 1:1:1 static weight scheduling type
			staticSameWeight := newSliceWithDefaultValue(clusterNum, 1)
			cpp.Spec.Placement.ReplicaScheduling = testhelper.NewStaticWeightPolicyStrategy(targetClusters, staticSameWeight)
		})

		ginkgo.It("do scale when metrics of cpu/mem utilization up in static weight schedule type", func() {
			ginkgo.By("step1: check initial replicas result should equal to deployment initial replicas", func() {
				framework.WaitDeploymentStatus(kubeClient, deployment, 3)
			})

			ginkgo.By("step2: pressure test the deployment of member clusters to increase the cpu/mem", runPressureJob)

			ginkgo.By("step3: check final replicas result should greater than deployment initial replicas", func() {
				framework.WaitDeploymentFitWith(kubeClient, namespace, deploymentName, func(deploy *appsv1.Deployment) bool {
					return *deploy.Spec.Replicas > 3
				})
			})
		})
	})

	// 3. dynamic weight scheduling
	ginkgo.Context("FederatedHPA scale Deployment in Dynamic Weight schedule type", func() {
		ginkgo.BeforeEach(func() {
			cpp.Spec.Placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
				},
			}
		})

		ginkgo.It("do scale when metrics of cpu/mem utilization up in dynamic weight schedule type", func() {
			ginkgo.By("step1: check initial replicas result should equal to deployment initial replicas", func() {
				framework.WaitDeploymentStatus(kubeClient, deployment, 3)
			})

			ginkgo.By("step2: pressure test the deployment of member clusters to increase the cpu/mem", runPressureJob)

			ginkgo.By("step3: check final replicas result should greater than deployment initial replicas", func() {
				framework.WaitDeploymentFitWith(kubeClient, namespace, deploymentName, func(deploy *appsv1.Deployment) bool {
					return *deploy.Spec.Replicas > 3
				})
			})
		})
	})
})

func newSliceWithDefaultValue(size int32, defaultValue int64) []int64 {
	slice := make([]int64, size)
	for i := range slice {
		slice[i] = defaultValue
	}
	return slice
}
