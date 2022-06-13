package defaultinterpreter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestAggregateDeploymentStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"replicas":            1,
		"readyReplicas":       1,
		"updatedReplicas":     1,
		"availableReplicas":   1,
		"unavailableReplicas": 0,
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	oldDeploy := &appsv1.Deployment{}
	newDeploy := &appsv1.Deployment{Status: appsv1.DeploymentStatus{Replicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, UnavailableReplicas: 0}}
	oldObj, _ := helper.ToUnstructured(oldDeploy)
	newObj, _ := helper.ToUnstructured(newDeploy)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update deployment status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
		{
			name:                  "ignore update deployment status as up to date",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregateDeploymentStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregateServiceStatus(t *testing.T) {
	statusMapNotLB := map[string]interface{}{}
	rawNotLB, _ := helper.BuildStatusRawExtension(statusMapNotLB)
	aggregatedStatusItemsNotLB := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: rawNotLB, Applied: true},
	}

	statusMapLB := map[string]interface{}{
		"loadBalancer": corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: "8.8.8.8"}}},
	}
	rawLB, _ := helper.BuildStatusRawExtension(statusMapLB)
	aggregatedStatusItemsLB := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: rawLB, Applied: true},
	}

	serviceClusterIP := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}}
	serviceNodePort := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort}}
	serviceExternalName := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName}}
	objServiceClusterIP, _ := helper.ToUnstructured(serviceClusterIP)
	objServiceNodePort, _ := helper.ToUnstructured(serviceNodePort)
	objServiceExternalName, _ := helper.ToUnstructured(serviceExternalName)

	oldServiceLoadBalancer := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}}
	newServiceLoadBalancer := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}, Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: "8.8.8.8", Hostname: "member1"}}}}}
	oldObjServiceLoadBalancer, _ := helper.ToUnstructured(oldServiceLoadBalancer)
	newObjServiceLoadBalancer, _ := helper.ToUnstructured(newServiceLoadBalancer)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "ignore update service status when type is ClusterIP",
			curObj:                objServiceClusterIP,
			aggregatedStatusItems: aggregatedStatusItemsNotLB,
			expectedObj:           objServiceClusterIP,
		},
		{
			name:                  "ignore update service status when type is NodePort",
			curObj:                objServiceNodePort,
			aggregatedStatusItems: aggregatedStatusItemsNotLB,
			expectedObj:           objServiceNodePort,
		},
		{
			name:                  "ignore update service status when type is ExternalName",
			curObj:                objServiceExternalName,
			aggregatedStatusItems: aggregatedStatusItemsNotLB,
			expectedObj:           objServiceExternalName,
		},
		{
			name:                  "update service status when type is LoadBalancer",
			curObj:                oldObjServiceLoadBalancer,
			aggregatedStatusItems: aggregatedStatusItemsLB,
			expectedObj:           newObjServiceLoadBalancer,
		},
		{
			name:                  "ignore update service status as up to date when type is LoadBalancer",
			curObj:                newObjServiceLoadBalancer,
			aggregatedStatusItems: aggregatedStatusItemsLB,
			expectedObj:           newObjServiceLoadBalancer,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregateServiceStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregateIngressStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"loadBalancer": corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: "8.8.8.8"}}},
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
	}

	oldIngress := &networkingv1.Ingress{}
	newIngress := &networkingv1.Ingress{Status: networkingv1.IngressStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: "8.8.8.8", Hostname: "member1"}}}}}
	oldObj, _ := helper.ToUnstructured(oldIngress)
	newObj, _ := helper.ToUnstructured(newIngress)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update ingress status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
		{
			name:                  "ignore update ingress status as up to date",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregateIngressStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregateJobStatus(t *testing.T) {
	startTime := metav1.Now()
	completionTime := startTime
	statusMap := map[string]interface{}{
		"active":         0,
		"succeeded":      1,
		"failed":         0,
		"startTime":      startTime,
		"completionTime": completionTime,
		"conditions":     []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	statusMapWithJobfailed := map[string]interface{}{
		"active":         0,
		"succeeded":      0,
		"failed":         1,
		"startTime":      startTime,
		"completionTime": completionTime,
		"conditions":     []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}},
	}
	rawWithJobFailed, _ := helper.BuildStatusRawExtension(statusMapWithJobfailed)
	aggregatedStatusItemsWithJobFailed := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: rawWithJobFailed, Applied: true},
	}

	oldJob := &batchv1.Job{}
	newJob := &batchv1.Job{Status: batchv1.JobStatus{Active: 0, Succeeded: 2, Failed: 0, StartTime: &startTime, CompletionTime: &completionTime, Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, Reason: "Completed", Message: "Job completed"}}}}
	oldObj, _ := helper.ToUnstructured(oldJob)
	newObj, _ := helper.ToUnstructured(newJob)

	newJobWithJobFailed := &batchv1.Job{Status: batchv1.JobStatus{Active: 0, Succeeded: 1, Failed: 1, StartTime: &startTime, CompletionTime: &completionTime, Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "JobFailed", Message: "Job executed failed in member clusters member2"}}}}
	newObjWithJobFailed, _ := helper.ToUnstructured(newJobWithJobFailed)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update job status without job failed",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
		{
			name:                  "update job status with job failed",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItemsWithJobFailed,
			expectedObj:           newObjWithJobFailed,
		},
		{
			name:                  "ignore update job status as up to date",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregateJobStatus(tt.curObj, tt.aggregatedStatusItems)
		// Clean condition time before compare, due to issue: https://github.com/karmada-io/karmada/issues/1767
		actualObj = cleanUnstructuredJobConditionTime(actualObj)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregateDaemonSetStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"currentNumberScheduled": 1,
		"numberMisscheduled":     0,
		"desiredNumberScheduled": 1,
		"numberReady":            1,
		"updatedNumberScheduled": 1,
		"numberAvailable":        1,
		"numberUnavailable":      0,
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	oldDaemonSet := &appsv1.DaemonSet{}
	newDaemonSet := &appsv1.DaemonSet{Status: appsv1.DaemonSetStatus{CurrentNumberScheduled: 2, NumberMisscheduled: 0, DesiredNumberScheduled: 2, NumberReady: 2, UpdatedNumberScheduled: 2, NumberAvailable: 2, NumberUnavailable: 0}}
	oldObj, _ := helper.ToUnstructured(oldDaemonSet)
	newObj, _ := helper.ToUnstructured(newDaemonSet)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update daemonSet status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
		{
			name:                  "ignore update daemonSet status as up to date",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregateDaemonSetStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregateStatefulSetStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"replicas":          1,
		"readyReplicas":     1,
		"currentReplicas":   1,
		"updatedReplicas":   1,
		"availableReplicas": 1,
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	oldStatefulSet := &appsv1.StatefulSet{}
	newStatefulSet := &appsv1.StatefulSet{Status: appsv1.StatefulSetStatus{Replicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, CurrentReplicas: 2}}
	oldObj, _ := helper.ToUnstructured(oldStatefulSet)
	newObj, _ := helper.ToUnstructured(newStatefulSet)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update statefulSet status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
		{
			name:                  "ignore update statefulSet status as up to date",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregateStatefulSetStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func cleanUnstructuredJobConditionTime(object *unstructured.Unstructured) *unstructured.Unstructured {
	job, _ := helper.ConvertToJob(object)
	for i := range job.Status.Conditions {
		cond := &job.Status.Conditions[i]
		cond.LastProbeTime = metav1.Time{}
		cond.LastTransitionTime = metav1.Time{}
	}
	ret, _ := helper.ToUnstructured(job)
	return ret
}

func TestAggregatePodStatus(t *testing.T) {
	startedTrue := true
	timeNow := time.Now()

	containerStatuses1 := []corev1.ContainerStatus{
		{
			ContainerID:  "containerd://6cee0afa333472f672341352e0d",
			Image:        "nginx:latest",
			ImageID:      "docker.io/library/import-2022-06-05@sha256:dfb593",
			Name:         "busybox-2",
			Ready:        true,
			RestartCount: 0,
			Started:      &startedTrue,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
		{
			ContainerID:  "containerd://b373fb05ebf57020573cdf4a4518a3b2a8",
			Image:        "nginx:latest",
			ImageID:      "docker.io/library/import-2022-06-05@sha256:a29d07a75",
			Name:         "busybox-2",
			Ready:        true,
			RestartCount: 0,
			Started:      &startedTrue,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
	}

	newContainerStatuses1 := []corev1.ContainerStatus{
		{
			Ready: true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
		{
			Ready: true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
	}

	curObj, _ := helper.ToUnstructured(&corev1.Pod{})
	newObj, _ := helper.ToUnstructured(&corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: newContainerStatuses1,
		Phase:             corev1.PodRunning,
	}})

	statusMap1 := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{containerStatuses1[0]},
		"phase":             corev1.PodRunning,
	}
	raw1, _ := helper.BuildStatusRawExtension(statusMap1)
	statusMap2 := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{containerStatuses1[1]},
		"phase":             corev1.PodRunning,
	}
	raw2, _ := helper.BuildStatusRawExtension(statusMap2)
	aggregatedStatusItems1 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw1, Applied: true},
		{ClusterName: "member2", Status: raw2, Applied: true},
	}

	containerStatuses2 := []corev1.ContainerStatus{
		{
			ContainerID:  "containerd://6cee0afa333472f672341352e0d",
			Image:        "nginx:latest",
			ImageID:      "docker.io/library/import-2022-06-05@sha256:dfb593",
			Name:         "busybox-2",
			Ready:        true,
			RestartCount: 0,
			Started:      &startedTrue,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
		{
			ContainerID:  "containerd://b373fb05ebf57020573cdf4a4518a3b2a8",
			Image:        "nginx:latest",
			ImageID:      "docker.io/library/import-2022-06-05@sha256:a29d07a75",
			Name:         "busybox-2",
			Ready:        false,
			RestartCount: 0,
			Started:      &startedTrue,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
	}

	statusMap3 := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{containerStatuses2[0]},
		"phase":             corev1.PodRunning,
	}
	raw3, _ := helper.BuildStatusRawExtension(statusMap3)
	statusMap4 := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{containerStatuses2[1]},
		"phase":             corev1.PodPending,
	}
	raw4, _ := helper.BuildStatusRawExtension(statusMap4)
	aggregatedStatusItems2 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw3, Applied: true},
		{ClusterName: "member2", Status: raw4, Applied: true},
	}

	newContainerStatuses2 := []corev1.ContainerStatus{
		{
			Ready: true,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
		{
			Ready: false,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Time{
						Time: timeNow,
					},
				},
			},
		},
	}

	newPodFailed := &corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: newContainerStatuses2,
		Phase:             corev1.PodPending,
	}}
	newObjFailed, _ := helper.ToUnstructured(newPodFailed)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update pod status",
			curObj:                curObj,
			aggregatedStatusItems: aggregatedStatusItems1,
			expectedObj:           newObj,
		},
		{
			name:                  "ignore update pod status as up to date",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems1,
			expectedObj:           newObj,
		},
		{
			name:                  "update pod status as one Pod failed",
			curObj:                newObj,
			aggregatedStatusItems: aggregatedStatusItems2,
			expectedObj:           newObjFailed,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregatePodStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}
