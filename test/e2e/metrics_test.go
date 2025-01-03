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

package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	"k8s.io/component-base/metrics/testutil"

	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("metrics testing", func() {
	var grabber *testhelper.Grabber

	const workqueueQueueDuration = "workqueue_queue_duration_seconds_sum"

	ginkgo.BeforeEach(func() {
		var err error
		grabber, err = testhelper.NewMetricsGrabber(context.TODO(), hostKubeClient)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	ginkgo.Context("workqueue metrics testing", func() {

		ginkgo.It("verify whether workqueue metrics exist", func() {
			var metrics *testutil.Metrics
			var workqueueQueueDurationMetrics model.Samples

			ginkgo.By("1. grab metrics from karmada controller", func() {
				// the output format of `metrics` is like:
				// [{
				//	"metric": {
				//		"__name__": "workqueue_queue_duration_seconds_sum",
				//		"controller": "work-status-controller",
				//		"name": "work-status-controller"
				//	},
				//	"value": [0, "0.12403110800000001"]
				//}]
				var err error
				metrics, err = grabber.GrabMetricsFromKarmadaControllerManager(context.TODO())
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("2. verify whether workqueue metrics exist", func() {
				workqueueQueueDurationMetrics = (*metrics)[workqueueQueueDuration]
				gomega.Expect(workqueueQueueDurationMetrics.Len()).Should(gomega.BeNumerically(">", 0))
			})

			ginkgo.By("3. verify the value of work-status-controller metric greater than 0", func() {
				workStatusMetric := testhelper.GetMetricByName(workqueueQueueDurationMetrics, "work-status-controller")
				gomega.Expect(workStatusMetric).ShouldNot(gomega.BeNil())
				gomega.Expect(workStatusMetric.Value).Should(gomega.BeNumerically(">", 0))
			})
		})
	})
})
