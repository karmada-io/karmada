package federatedhpa

import (
	"context"
	"fmt"
	"math"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	metricsclient "github.com/karmada-io/karmada/pkg/controllers/federatedhpa/metrics"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// This file is basically lifted from https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/controller/podautoscaler/replica_calculator.go.
// The main difference is:
// 1. ReplicaCalculator no longer has podLister built in. PodList is calculated in the outer controller.
// 2. ReplicaCalculator needs to import a calibration value to calibrate the calculation results
// when they are determined by the global number of ready Pods or metrics.

// ReplicaCalculator bundles all needed information to calculate the target amount of replicas
type ReplicaCalculator struct {
	metricsClient                 metricsclient.MetricsClient
	tolerance                     float64
	cpuInitializationPeriod       time.Duration
	delayOfInitialReadinessStatus time.Duration
}

// NewReplicaCalculator creates a new ReplicaCalculator and passes all necessary information to the new instance
func NewReplicaCalculator(metricsClient metricsclient.MetricsClient, tolerance float64, cpuInitializationPeriod, delayOfInitialReadinessStatus time.Duration) *ReplicaCalculator {
	return &ReplicaCalculator{
		metricsClient:                 metricsClient,
		tolerance:                     tolerance,
		cpuInitializationPeriod:       cpuInitializationPeriod,
		delayOfInitialReadinessStatus: delayOfInitialReadinessStatus,
	}
}

// GetResourceReplicas calculates the desired replica count based on a target resource utilization percentage
// of the given resource for pods matching the given selector in the given namespace, and the current replica count
//
//nolint:gocyclo
func (c *ReplicaCalculator) GetResourceReplicas(ctx context.Context, currentReplicas int32, targetUtilization int32, resource corev1.ResourceName, namespace string, selector labels.Selector, container string, podList []*corev1.Pod, calibration float64) (replicaCount int32, utilization int32, rawUtilization int64, timestamp time.Time, err error) {
	metrics, timestamp, err := c.metricsClient.GetResourceMetric(ctx, resource, namespace, selector, container)
	if err != nil {
		return 0, 0, 0, time.Time{}, fmt.Errorf("unable to get metrics for resource %s: %v", resource, err)
	}

	itemsLen := len(podList)
	if itemsLen == 0 {
		return 0, 0, 0, time.Time{}, fmt.Errorf("no pods returned by selector while calculating replica count")
	}

	readyPodCount, unreadyPods, missingPods, ignoredPods := groupPods(podList, metrics, resource, c.cpuInitializationPeriod, c.delayOfInitialReadinessStatus)
	removeMetricsForPods(metrics, ignoredPods)
	removeMetricsForPods(metrics, unreadyPods)
	requests, err := calculatePodRequests(podList, container, resource)
	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}

	if len(metrics) == 0 {
		return 0, 0, 0, time.Time{}, fmt.Errorf("did not receive metrics for any ready pods")
	}

	usageRatio, utilization, rawUtilization, err := metricsclient.GetResourceUtilizationRatio(metrics, requests, targetUtilization)
	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}

	scaleUpWithUnready := len(unreadyPods) > 0 && usageRatio > 1.0
	if !scaleUpWithUnready && len(missingPods) == 0 {
		if math.Abs(1.0-usageRatio) <= c.tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas, utilization, rawUtilization, timestamp, nil
		}

		// if we don't have any unready or missing pods, we can calculate the new replica count now
		return int32(math.Ceil(usageRatio * float64(readyPodCount) / calibration)), utilization, rawUtilization, timestamp, nil
	}

	if len(missingPods) > 0 {
		if usageRatio < 1.0 {
			// on a scale-down, treat missing pods as using 100% (all) of the resource request
			// or the utilization target for targets higher than 100%
			fallbackUtilization := int64(max(100, targetUtilization))
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: requests[podName] * fallbackUtilization / 100}
			}
		} else if usageRatio > 1.0 {
			// on a scale-up, treat missing pods as using 0% of the resource request
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: 0}
			}
		}
	}

	if scaleUpWithUnready {
		// on a scale-up, treat unready pods as using 0% of the resource request
		for podName := range unreadyPods {
			metrics[podName] = metricsclient.PodMetric{Value: 0}
		}
	}

	// re-run the utilization calculation with our new numbers
	newUsageRatio, _, _, err := metricsclient.GetResourceUtilizationRatio(metrics, requests, targetUtilization)
	if err != nil {
		return 0, utilization, rawUtilization, time.Time{}, err
	}

	if math.Abs(1.0-newUsageRatio) <= c.tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
		// return the current replicas if the change would be too small,
		// or if the new usage ratio would cause a change in scale direction
		return currentReplicas, utilization, rawUtilization, timestamp, nil
	}

	newReplicas := int32(math.Ceil(newUsageRatio * float64(len(metrics)) / calibration))
	if (newUsageRatio < 1.0 && newReplicas > currentReplicas) || (newUsageRatio > 1.0 && newReplicas < currentReplicas) {
		// return the current replicas if the change of metrics length would cause a change in scale direction
		return currentReplicas, utilization, rawUtilization, timestamp, nil
	}

	// return the result, where the number of replicas considered is
	// however many replicas factored into our calculation
	return newReplicas, utilization, rawUtilization, timestamp, nil
}

// GetRawResourceReplicas calculates the desired replica count based on a target resource utilization (as a raw milli-value)
// for pods matching the given selector in the given namespace, and the current replica count
func (c *ReplicaCalculator) GetRawResourceReplicas(ctx context.Context, currentReplicas int32, targetUsage int64, resource corev1.ResourceName, namespace string, selector labels.Selector, container string, podList []*corev1.Pod, calibration float64) (replicaCount int32, usage int64, timestamp time.Time, err error) {
	metrics, timestamp, err := c.metricsClient.GetResourceMetric(ctx, resource, namespace, selector, container)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metrics for resource %s: %v", resource, err)
	}

	replicaCount, usage, err = c.calcPlainMetricReplicas(metrics, currentReplicas, targetUsage, resource, podList, calibration)
	return replicaCount, usage, timestamp, err
}

// GetMetricReplicas calculates the desired replica count based on a target metric usage
// (as a milli-value) for pods matching the given selector in the given namespace, and the
// current replica count
func (c *ReplicaCalculator) GetMetricReplicas(currentReplicas int32, targetUsage int64, metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector, podList []*corev1.Pod, calibration float64) (replicaCount int32, usage int64, timestamp time.Time, err error) {
	metrics, timestamp, err := c.metricsClient.GetRawMetric(metricName, namespace, selector, metricSelector)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metric %s: %v", metricName, err)
	}

	replicaCount, usage, err = c.calcPlainMetricReplicas(metrics, currentReplicas, targetUsage, corev1.ResourceName(""), podList, calibration)
	return replicaCount, usage, timestamp, err
}

// calcPlainMetricReplicas calculates the desired replicas for plain (i.e. non-utilization percentage) metrics.
//
//nolint:gocyclo
func (c *ReplicaCalculator) calcPlainMetricReplicas(metrics metricsclient.PodMetricsInfo, currentReplicas int32, targetUsage int64, resource corev1.ResourceName, podList []*corev1.Pod, calibration float64) (replicaCount int32, usage int64, err error) {
	if len(podList) == 0 {
		return 0, 0, fmt.Errorf("no pods returned by selector while calculating replica count")
	}

	readyPodCount, unreadyPods, missingPods, ignoredPods := groupPods(podList, metrics, resource, c.cpuInitializationPeriod, c.delayOfInitialReadinessStatus)
	removeMetricsForPods(metrics, ignoredPods)
	removeMetricsForPods(metrics, unreadyPods)

	if len(metrics) == 0 {
		return 0, 0, fmt.Errorf("did not receive metrics for any ready pods")
	}

	usageRatio, usage := metricsclient.GetMetricUsageRatio(metrics, targetUsage)

	scaleUpWithUnready := len(unreadyPods) > 0 && usageRatio > 1.0

	if !scaleUpWithUnready && len(missingPods) == 0 {
		if math.Abs(1.0-usageRatio) <= c.tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas, usage, nil
		}

		// if we don't have any unready or missing pods, we can calculate the new replica count now
		return int32(math.Ceil(usageRatio * float64(readyPodCount) / calibration)), usage, nil
	}

	if len(missingPods) > 0 {
		if usageRatio < 1.0 {
			// on a scale-down, treat missing pods as using 100% of the resource request
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: targetUsage}
			}
		} else if usageRatio > 1.0 {
			// on a scale-up, treat missing pods as using 0% of the resource request
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: 0}
			}
		}
	}

	if scaleUpWithUnready {
		// on a scale-up, treat unready pods as using 0% of the resource request
		for podName := range unreadyPods {
			metrics[podName] = metricsclient.PodMetric{Value: 0}
		}
	}

	// re-run the utilization calculation with our new numbers
	newUsageRatio, _ := metricsclient.GetMetricUsageRatio(metrics, targetUsage)

	if math.Abs(1.0-newUsageRatio) <= c.tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
		// return the current replicas if the change would be too small,
		// or if the new usage ratio would cause a change in scale direction
		return currentReplicas, usage, nil
	}

	newReplicas := int32(math.Ceil(newUsageRatio * float64(len(metrics)) / calibration))
	if (newUsageRatio < 1.0 && newReplicas > currentReplicas) || (newUsageRatio > 1.0 && newReplicas < currentReplicas) {
		// return the current replicas if the change of metrics length would cause a change in scale direction
		return currentReplicas, usage, nil
	}

	// return the result, where the number of replicas considered is
	// however many replicas factored into our calculation
	return newReplicas, usage, nil
}

// GetObjectMetricReplicas calculates the desired replica count based on a target metric usage (as a milli-value)
// for the given object in the given namespace, and the current replica count.
func (c *ReplicaCalculator) GetObjectMetricReplicas(currentReplicas int32, targetUsage int64, metricName string, namespace string, objectRef *autoscalingv2.CrossVersionObjectReference, metricSelector labels.Selector, podList []*corev1.Pod, calibration float64) (replicaCount int32, usage int64, timestamp time.Time, err error) {
	usage, _, err = c.metricsClient.GetObjectMetric(metricName, namespace, objectRef, metricSelector)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metric %s: %v on %s %s/%s", metricName, objectRef.Kind, namespace, objectRef.Name, err)
	}

	usageRatio := float64(usage) / float64(targetUsage)
	replicaCount, timestamp, err = c.getUsageRatioReplicaCount(currentReplicas, usageRatio, podList, calibration)
	return replicaCount, usage, timestamp, err
}

// GetObjectPerPodMetricReplicas calculates the desired replica count based on a target metric usage (as a milli-value)
// for the given object in the given namespace, and the current replica count.
func (c *ReplicaCalculator) GetObjectPerPodMetricReplicas(statusReplicas int32, targetAverageUsage int64, metricName string, namespace string, objectRef *autoscalingv2.CrossVersionObjectReference, metricSelector labels.Selector, calibration float64) (replicaCount int32, usage int64, timestamp time.Time, err error) {
	// The usage here refers to the total value of all metrics from the pods.
	usage, timestamp, err = c.metricsClient.GetObjectMetric(metricName, namespace, objectRef, metricSelector)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metric %s: %v on %s %s/%s", metricName, objectRef.Kind, namespace, objectRef.Name, err)
	}

	replicaCount = statusReplicas
	usageRatio := float64(usage) / (float64(targetAverageUsage) * float64(replicaCount))
	if math.Abs(1.0-usageRatio) > c.tolerance {
		// update number of replicas if change is large enough
		replicaCount = int32(math.Ceil(float64(usage) / float64(targetAverageUsage) / calibration))
	}
	usage = int64(math.Ceil(float64(usage) / float64(statusReplicas)))
	return int32(math.Ceil(float64(replicaCount) / calibration)), usage, timestamp, nil
}

// getUsageRatioReplicaCount calculates the desired replica count based on usageRatio and ready pods count.
// For currentReplicas=0 doesn't take into account ready pods count and tolerance to support scaling to zero pods.
func (c *ReplicaCalculator) getUsageRatioReplicaCount(currentReplicas int32, usageRatio float64, podList []*corev1.Pod, calibration float64) (replicaCount int32, timestamp time.Time, err error) {
	if currentReplicas != 0 {
		if math.Abs(1.0-usageRatio) <= c.tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas, timestamp, nil
		}
		readyPodCount := int64(0)
		readyPodCount, err = c.getReadyPodsCount(podList)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("unable to calculate ready pods: %s", err)
		}
		replicaCount = int32(math.Ceil(usageRatio * float64(readyPodCount) / calibration))
	} else {
		// Scale to zero or n pods depending on usageRatio
		replicaCount = int32(math.Ceil(usageRatio))
	}

	return replicaCount, timestamp, err
}

// @TODO(mattjmcnaughton) Many different functions in this module use variations
// of this function. Make this function generic, so we don't repeat the same
// logic in multiple places.
func (c *ReplicaCalculator) getReadyPodsCount(podList []*corev1.Pod) (int64, error) {
	if len(podList) == 0 {
		return 0, fmt.Errorf("no pods returned by selector while calculating replica count")
	}

	readyPodCount := 0

	for _, pod := range podList {
		if pod.Status.Phase == corev1.PodRunning && helper.IsPodReady(pod) {
			readyPodCount++
		}
	}

	return int64(readyPodCount), nil
}

func groupPods(pods []*corev1.Pod, metrics metricsclient.PodMetricsInfo, resource corev1.ResourceName, cpuInitializationPeriod, delayOfInitialReadinessStatus time.Duration) (readyPodCount int, unreadyPods, missingPods, ignoredPods sets.Set[string]) {
	missingPods = sets.New[string]()
	unreadyPods = sets.New[string]()
	ignoredPods = sets.New[string]()
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil || pod.Status.Phase == corev1.PodFailed {
			ignoredPods.Insert(pod.Name)
			continue
		}
		// Pending pods are ignored.
		if pod.Status.Phase == corev1.PodPending {
			unreadyPods.Insert(pod.Name)
			continue
		}
		// Pods missing metrics.
		metric, found := metrics[pod.Name]
		if !found {
			missingPods.Insert(pod.Name)
			continue
		}
		// Unready pods are ignored.
		if resource == corev1.ResourceCPU {
			var unready bool
			_, condition := helper.GetPodCondition(&pod.Status, corev1.PodReady)
			if condition == nil || pod.Status.StartTime == nil {
				unready = true
			} else {
				// Pod still within possible initialisation period.
				if pod.Status.StartTime.Add(cpuInitializationPeriod).After(time.Now()) {
					// Ignore sample if pod is unready or one window of metric wasn't collected since last state transition.
					unready = condition.Status == corev1.ConditionFalse || metric.Timestamp.Before(condition.LastTransitionTime.Time.Add(metric.Window))
				} else {
					// Ignore metric if pod is unready and it has never been ready.
					unready = condition.Status == corev1.ConditionFalse && pod.Status.StartTime.Add(delayOfInitialReadinessStatus).After(condition.LastTransitionTime.Time)
				}
			}
			if unready {
				unreadyPods.Insert(pod.Name)
				continue
			}
		}
		readyPodCount++
	}
	return
}

func calculatePodRequests(pods []*corev1.Pod, container string, resource corev1.ResourceName) (map[string]int64, error) {
	requests := make(map[string]int64, len(pods))
	for _, pod := range pods {
		podSum := int64(0)
		for _, c := range pod.Spec.Containers {
			if container == "" || container == c.Name {
				if containerRequest, ok := c.Resources.Requests[resource]; ok {
					podSum += containerRequest.MilliValue()
				} else {
					return nil, fmt.Errorf("missing request for %s in container %s of Pod %s", resource, c.Name, pod.ObjectMeta.Name)
				}
			}
		}
		requests[pod.Name] = podSum
	}
	return requests, nil
}

func removeMetricsForPods(metrics metricsclient.PodMetricsInfo, pods sets.Set[string]) {
	for _, pod := range pods.UnsortedList() {
		delete(metrics, pod)
	}
}
