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

package helper

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
)

const (
	karmadaNamespace                     = "karmada-system"
	karmadaControllerManagerDeploy       = "karmada-controller-manager"
	karmadaControllerManagerPort         = 8080
	karmadaControllerManagerLeaderMetric = "leader_election_master_status"
)

// Grabber is used to grab metrics from karmada components
type Grabber struct {
	hostKubeClient        clientset.Interface
	controllerManagerPods []string
}

// NewMetricsGrabber creates a new metrics grabber
func NewMetricsGrabber(ctx context.Context, c clientset.Interface) (*Grabber, error) {
	grabber := Grabber{hostKubeClient: c}
	regKarmadaControllerManager := regexp.MustCompile(karmadaControllerManagerDeploy + "-.*")

	podList, err := c.CoreV1().Pods(karmadaNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(podList.Items) < 1 {
		klog.Warningf("Can't find any pods in namespace %s to grab metrics from", karmadaNamespace)
	}
	for _, pod := range podList.Items {
		if regKarmadaControllerManager.MatchString(pod.Name) {
			grabber.controllerManagerPods = append(grabber.controllerManagerPods, pod.Name)
		}
	}
	return &grabber, nil
}

// GrabMetricsFromKarmadaControllerManager fetch metrics from the leader of karmada controller manager
func (g *Grabber) GrabMetricsFromKarmadaControllerManager(ctx context.Context) (*testutil.Metrics, error) {
	var output string
	var lastMetricsFetchErr error

	var result *testutil.Metrics
	// judge which pod is the leader of karmada controller manager
	for _, podName := range g.controllerManagerPods {
		if metricsWaitErr := wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
			output, lastMetricsFetchErr = GetMetricsFromPod(ctx, g.hostKubeClient, podName, karmadaNamespace, karmadaControllerManagerPort)
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

		if !isKarmadaControllerManagerLeader(podMetrics[karmadaControllerManagerLeaderMetric]) {
			klog.Infof("skip fetch %s since it is not the leader pod", podName)
			continue
		}

		result = &podMetrics
		break
	}

	if result == nil {
		return nil, fmt.Errorf("failed to fetch metrics from the leader of karmada controller manager")
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

func isKarmadaControllerManagerLeader(samples model.Samples) bool {
	for _, sample := range samples {
		if sample.Metric["name"] == karmadaControllerManagerDeploy && sample.Value > 0 {
			return true
		}
	}
	return false
}

// GetMetricByName returns the metric value with the given name.
func GetMetricByName(samples model.Samples, name string) *model.Sample {
	for _, sample := range samples {
		if sample.Metric["name"] == model.LabelValue(name) {
			return sample
		}
	}
	return nil
}
