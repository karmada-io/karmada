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

package framework

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	karmadaNamespace = "karmada-system"
	metricsBindPort  = 8080
	leaderPodMetric  = "leader_election_master_status"
	queryTimeout     = 10 * time.Second
)

// following refers to https://github.com/kubernetes/kubernetes/blob/master/test/e2e/framework/metrics/metrics_grabber.go

// Grabber is used to grab metrics from karmada components
type Grabber struct {
	hostKubeClient         clientset.Interface
	controllerManagerPods  []string
	schedulerPods          []string
	deschedulerPods        []string
	metricsAdapterPods     []string
	schedulerEstimatorPods []string
	webhookPods            []string
}

// NewMetricsGrabber creates a new metrics grabber
func NewMetricsGrabber(ctx context.Context, c clientset.Interface) (*Grabber, error) {
	grabber := Grabber{hostKubeClient: c}
	regKarmadaControllerManagerPods := regexp.MustCompile(names.KarmadaControllerManagerComponentName + "-.*")
	regKarmadaSchedulerPods := regexp.MustCompile(names.KarmadaSchedulerComponentName + "-.*")
	regKarmadaDeschedulerPods := regexp.MustCompile(names.KarmadaDeschedulerComponentName + "-.*")
	regKarmadaMetricsAdapterPods := regexp.MustCompile(names.KarmadaMetricsAdapterComponentName + "-.*")
	regKarmadaSchedulerEstimatorPods := regexp.MustCompile(names.KarmadaSchedulerEstimatorComponentName + "-" + ClusterNames()[0] + "-.*")
	regKarmadaWebhookPods := regexp.MustCompile(names.KarmadaWebhookComponentName + "-.*")

	podList, err := c.CoreV1().Pods(karmadaNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(podList.Items) < 1 {
		klog.Warningf("Can't find any pods in namespace %s to grab metrics from", karmadaNamespace)
	}
	for _, pod := range podList.Items {
		if regKarmadaControllerManagerPods.MatchString(pod.Name) {
			grabber.controllerManagerPods = append(grabber.controllerManagerPods, pod.Name)
			continue
		}
		if regKarmadaDeschedulerPods.MatchString(pod.Name) {
			grabber.deschedulerPods = append(grabber.deschedulerPods, pod.Name)
			continue
		}
		if regKarmadaMetricsAdapterPods.MatchString(pod.Name) {
			grabber.metricsAdapterPods = append(grabber.metricsAdapterPods, pod.Name)
			continue
		}
		if regKarmadaSchedulerEstimatorPods.MatchString(pod.Name) {
			grabber.schedulerEstimatorPods = append(grabber.schedulerEstimatorPods, pod.Name)
			continue
		}
		if regKarmadaSchedulerPods.MatchString(pod.Name) {
			grabber.schedulerPods = append(grabber.schedulerPods, pod.Name)
			continue
		}
		if regKarmadaWebhookPods.MatchString(pod.Name) {
			grabber.webhookPods = append(grabber.webhookPods, pod.Name)
		}
	}
	return &grabber, nil
}

// GrabMetricsFromComponent fetch metrics from the leader of a specified Karmada component
func (g *Grabber) GrabMetricsFromComponent(ctx context.Context, component string) (map[string]testutil.Metrics, error) {
	pods, fromLeader := make([]string, 0), false
	switch component {
	case names.KarmadaControllerManagerComponentName:
		pods, fromLeader = g.controllerManagerPods, true
	case names.KarmadaSchedulerComponentName:
		pods, fromLeader = g.schedulerPods, true
	case names.KarmadaDeschedulerComponentName:
		pods, fromLeader = g.deschedulerPods, true
	case names.KarmadaMetricsAdapterComponentName:
		pods = g.metricsAdapterPods
	case names.KarmadaSchedulerEstimatorComponentName:
		pods = g.schedulerEstimatorPods
	case names.KarmadaWebhookComponentName:
		pods = g.webhookPods
	}
	return g.grabMetricsFromPod(ctx, component, pods, fromLeader)
}

// grabMetricsFromPod fetch metrics from the leader pod
func (g *Grabber) grabMetricsFromPod(ctx context.Context, component string, pods []string, fromLeader bool) (map[string]testutil.Metrics, error) {
	var output string
	var lastMetricsFetchErr error

	result := make(map[string]testutil.Metrics)
	for _, podName := range pods {
		if metricsWaitErr := wait.PollUntilContextTimeout(ctx, time.Second, queryTimeout, true, func(ctx context.Context) (bool, error) {
			output, lastMetricsFetchErr = GetMetricsFromPod(ctx, g.hostKubeClient, podName, karmadaNamespace, metricsBindPort)
			return lastMetricsFetchErr == nil, nil
		}); metricsWaitErr != nil {
			klog.Errorf("error waiting for %s to expose metrics: %v; %v", podName, metricsWaitErr, lastMetricsFetchErr)
			continue
		}

		podMetrics := testutil.Metrics{}
		metricsParseErr := testutil.ParseMetrics(output, &podMetrics)
		if metricsParseErr != nil {
			klog.Errorf("failed to parse metrics for %s: %v", podName, metricsParseErr)
			continue
		}

		// judge which pod is the leader pod
		if fromLeader && !isLeaderPod(podMetrics[leaderPodMetric]) {
			klog.Infof("skip fetch %s since it is not the leader pod", podName)
			continue
		}

		result[podName] = podMetrics
		klog.Infof("successfully grabbed metrics of %s", podName)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("failed to fetch metrics from the pod of %s", component)
	}
	return result, nil
}

// GetMetricsFromPod retrieves metrics data.
func GetMetricsFromPod(ctx context.Context, client clientset.Interface, podName string, namespace string, port int) (string, error) {
	rawOutput, err := client.CoreV1().RESTClient().Get().
		Namespace(namespace).
		Resource("pods").
		SubResource("proxy").
		Name(fmt.Sprintf("%s:%d", podName, port)).
		Suffix("metrics").
		Do(ctx).Raw()
	if err != nil {
		return "", err
	}
	return string(rawOutput), nil
}

func isLeaderPod(samples model.Samples) bool {
	for _, sample := range samples {
		if sample.Value > 0 {
			return true
		}
	}
	return false
}

// PrintMetricSample prints the metric sample
func PrintMetricSample(podName string, sample model.Samples) {
	if sample.Len() == 0 {
		return
	}
	if podName != "" {
		klog.Infof("metrics from pod: %s", podName)
	}
	for _, s := range sample {
		klog.Infof("metric: %v, value: %v, timestamp: %v", s.Metric, s.Value, s.Timestamp)
	}
}
