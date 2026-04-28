/*
Copyright 2023 The Karmada Authors.

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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("metrics testing", func() {
	var grabber *framework.Grabber

	var componentMetrics = map[string][]string{
		names.KarmadaControllerManagerComponentName: {
			"workqueue_queue_duration_seconds_sum",    // workqueue metrics
			"cluster_ready_state",                     // custom ClusterCollectors metrics
			"work_sync_workload_duration_seconds_sum", // custom ResourceCollectors metrics
		},
		names.KarmadaSchedulerComponentName: {
			"workqueue_queue_duration_seconds_sum",      // workqueue metrics
			"karmada_scheduler_schedule_attempts_total", // scheduler custom metrics
		},
		names.KarmadaDeschedulerComponentName: {
			"workqueue_queue_duration_seconds_sum", // workqueue metrics
		},
		names.KarmadaMetricsAdapterComponentName: {
			"workqueue_queue_duration_seconds_sum", // workqueue metrics
		},
		names.KarmadaSchedulerEstimatorComponentName: {
			"karmada_scheduler_estimator_estimating_request_total", // scheduler estimator custom metrics
		},
		names.KarmadaWebhookComponentName: {
			"controller_runtime_webhook_requests_total", // controller runtime hook server metrics
		},
	}

	ginkgo.BeforeEach(func() {
		var err error
		grabber, err = framework.NewMetricsGrabber(context.TODO(), hostKubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.Context("metrics presence testing", func() {
		ginkgo.It("metrics presence testing for each component", func() {
			ginkgo.By("do a simple scheduling to ensure above metrics exist", func() {
				name := deploymentNamePrefix + rand.String(RandomStrLength)
				deployment := testhelper.NewDeployment(testNamespace, name)
				policy := testhelper.NewPropagationPolicy(testNamespace, name, []policyv1alpha1.ResourceSelector{
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
				framework.CreateDeployment(kubeClient, deployment)
				framework.CreatePropagationPolicy(karmadaClient, policy)
				ginkgo.DeferCleanup(func() {
					framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
					framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
				})
				framework.WaitDeploymentPresentOnClustersFitWith(framework.ClusterNames(), deployment.Namespace, deployment.Name, func(_ *appsv1.Deployment) bool { return true })
			})

			for component, metricNameList := range componentMetrics {
				ginkgo.By("judge metrics presence of component: "+component, func() {
					podsMetrics, err := grabber.GrabMetricsFromComponent(context.TODO(), component)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

					for _, metricName := range metricNameList {
						metricExist := false
						for podName, metrics := range podsMetrics {
							// the output format of `metrics` is like:
							// {
							// 	"workqueue_queue_duration_seconds_sum":  [{
							//		"metric": {
							//			"__name__": "workqueue_queue_duration_seconds_sum",
							//			"controller": "work-status-controller",
							//			"name": "work-status-controller"
							//		},
							//			"value": [0, "0.12403110800000001"]
							//	}]
							// }
							framework.PrintMetricSample(podName, metrics[metricName])
							if metrics[metricName].Len() > 0 {
								metricExist = true
								break
							}
						}
						if !metricExist {
							klog.Errorf("metric %s not found in component %s", metricName, component)
							gomega.Expect(metricExist).ShouldNot(gomega.BeFalse())
						}
					}
				})
			}
		})
	})
})
