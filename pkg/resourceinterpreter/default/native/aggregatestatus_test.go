package native

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
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
	newIngress := &networkingv1.Ingress{Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "8.8.8.8", Hostname: "member1"}}}}}
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
	job := &batchv1.Job{}
	_ = helper.ConvertToTypedObject(object, job)
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

	containerStatusesRunning := []corev1.ContainerStatus{
		{
			Ready: true,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}},
		},
	}
	newContainerStatusesFail := []corev1.ContainerStatus{
		{
			Ready: false,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 125}},
		},
	}
	newContainerStatusesSucceeded := []corev1.ContainerStatus{
		{
			Ready: false,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
		},
	}
	statusMapRunning := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{containerStatusesRunning[0]},
		"phase":             corev1.PodRunning,
	}
	statusMapFailed := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{newContainerStatusesFail[0]},
		"phase":             corev1.PodFailed,
	}
	statusMapSucceeded := map[string]interface{}{
		"containerStatuses": []corev1.ContainerStatus{newContainerStatusesSucceeded[0]},
		"phase":             corev1.PodSucceeded,
	}

	rawRunning, _ := helper.BuildStatusRawExtension(statusMapRunning)

	// test failed
	rawFail, _ := helper.BuildStatusRawExtension(statusMapFailed)
	aggregatedStatusItemsFail := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: rawRunning, Applied: true},
		{ClusterName: "member2", Status: rawFail, Applied: true},
	}
	aggregatedStatusItemsPending := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: rawRunning, Applied: true},
		{ClusterName: "member2", Status: nil, Applied: true},
	}

	failObj, _ := helper.ToUnstructured(&corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{containerStatusesRunning[0], newContainerStatusesFail[0]},
		Phase:             corev1.PodFailed,
	}})

	// test succeeded
	rawSucceeded, _ := helper.BuildStatusRawExtension(statusMapSucceeded)
	aggregatedStatusItemsSucceeded := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: rawRunning, Applied: true},
		{ClusterName: "member2", Status: rawSucceeded, Applied: true},
	}

	succeededObj, _ := helper.ToUnstructured(&corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{containerStatusesRunning[0], newContainerStatusesSucceeded[0]},
		Phase:             corev1.PodRunning,
	}})

	pendingObj, _ := helper.ToUnstructured(&corev1.Pod{Status: corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{containerStatusesRunning[0]},
		Phase:             corev1.PodPending,
	}})

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
		// More details please refer to: https://github.com/karmada-io/karmada/issues/2137
		{
			name:                  "failed pod status",
			curObj:                curObj,
			aggregatedStatusItems: aggregatedStatusItemsFail, //  Running + Failed => Failed
			expectedObj:           failObj,
		},
		{
			name:                  "succeeded pod status",
			curObj:                curObj,
			aggregatedStatusItems: aggregatedStatusItemsSucceeded, //  Running + Succeeded => Running
			expectedObj:           succeededObj,
		},
		{
			name:                  "pending pod status",
			curObj:                curObj,
			aggregatedStatusItems: aggregatedStatusItemsPending, //  Running + nil => Pending
			expectedObj:           pendingObj,
		},
	}

	for _, tt := range tests {
		actualObj, _ := aggregatePodStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregatePVCStatus(t *testing.T) {
	statusBoundMap := map[string]interface{}{
		"phase": corev1.ClaimBound,
	}
	// Bound status
	boundRaw1, _ := helper.BuildStatusRawExtension(statusBoundMap)
	boundRaw2, _ := helper.BuildStatusRawExtension(statusBoundMap)

	aggregatedStatusItems1 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: boundRaw1, Applied: true},
		{ClusterName: "member2", Status: boundRaw2, Applied: true},
	}

	// Lost status
	statusLostMap := map[string]interface{}{
		"phase": corev1.ClaimLost,
	}
	lostRaw1, _ := helper.BuildStatusRawExtension(statusBoundMap)
	lostRaw2, _ := helper.BuildStatusRawExtension(statusLostMap)

	aggregatedStatusItems2 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: lostRaw1, Applied: true},
		{ClusterName: "member2", Status: lostRaw2, Applied: true},
	}

	// Pending status
	statusPendingMap := map[string]interface{}{
		"phase": corev1.ClaimPending,
	}
	pendingRaw1, _ := helper.BuildStatusRawExtension(statusBoundMap)
	pendingRaw2, _ := helper.BuildStatusRawExtension(statusPendingMap)

	aggregatedStatusItems3 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: pendingRaw1, Applied: true},
		{ClusterName: "member2", Status: pendingRaw2, Applied: true},
	}

	// test aggregatePersistentVolumeClaimStatus function
	oldPVC := &corev1.PersistentVolumeClaim{}
	oldObj, _ := helper.ToUnstructured(oldPVC)

	boundNewPVC := &corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	newBoundPVCObj, _ := helper.ToUnstructured(boundNewPVC)

	lostNewPVC := &corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimLost}}
	newLostPVCObj, _ := helper.ToUnstructured(lostNewPVC)

	pendingNewPVC := &corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending}}
	newPendingPVCObj, _ := helper.ToUnstructured(pendingNewPVC)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update pvc status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems1,
			expectedObj:           newBoundPVCObj,
		},
		{
			name:                  "update pvc status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems2,
			expectedObj:           newLostPVCObj,
		},
		{
			name:                  "update pvc status",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems3,
			expectedObj:           newPendingPVCObj,
		},
		{
			name:                  "ignore update pvc status as up to date",
			curObj:                newBoundPVCObj,
			aggregatedStatusItems: aggregatedStatusItems1,
			expectedObj:           newBoundPVCObj,
		},
	}
	for _, tt := range tests {
		actualObj, _ := aggregatePersistentVolumeClaimStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj)
	}
}

func TestAggregatePVStatus(t *testing.T) {
	statusAvailableMap := map[string]interface{}{
		"phase": corev1.VolumeAvailable,
	}
	// Available status
	availableRaw1, _ := helper.BuildStatusRawExtension(statusAvailableMap)
	availableRaw2, _ := helper.BuildStatusRawExtension(statusAvailableMap)

	aggregatedStatusItems1 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: availableRaw1, Applied: true},
		{ClusterName: "member2", Status: availableRaw2, Applied: true},
	}

	statusReleasedMap := map[string]interface{}{
		"phase": corev1.VolumeReleased,
	}
	// Release status
	releasedRaw1, _ := helper.BuildStatusRawExtension(statusReleasedMap)
	releasedRaw2, _ := helper.BuildStatusRawExtension(statusReleasedMap)

	aggregatedStatusItems2 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: releasedRaw1, Applied: true},
		{ClusterName: "member2", Status: releasedRaw2, Applied: true},
	}

	statusBoundMap := map[string]interface{}{
		"phase": corev1.VolumeBound,
	}
	// Bound status
	boundRaw1, _ := helper.BuildStatusRawExtension(statusBoundMap)
	boundRaw2, _ := helper.BuildStatusRawExtension(statusReleasedMap)

	aggregatedStatusItems3 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: boundRaw1, Applied: true},
		{ClusterName: "member2", Status: boundRaw2, Applied: true},
	}

	// Failed status
	statusFailedMap := map[string]interface{}{
		"phase": corev1.VolumeFailed,
	}
	failedRaw1, _ := helper.BuildStatusRawExtension(releasedRaw1)
	failedRaw2, _ := helper.BuildStatusRawExtension(statusFailedMap)

	aggregatedStatusItems4 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: failedRaw1, Applied: true},
		{ClusterName: "member2", Status: failedRaw2, Applied: true},
	}

	// Pending status
	statusPendingMap := map[string]interface{}{
		"phase": corev1.VolumePending,
	}
	pendingRaw1, _ := helper.BuildStatusRawExtension(statusAvailableMap)
	pendingRaw2, _ := helper.BuildStatusRawExtension(statusPendingMap)

	aggregatedStatusItems5 := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: pendingRaw1, Applied: true},
		{ClusterName: "member2", Status: pendingRaw2, Applied: true},
	}

	// test aggregatePersistentVolumeStatus function
	oldPVC := &corev1.PersistentVolume{}
	oldObj, _ := helper.ToUnstructured(oldPVC)

	availableNewPv := &corev1.PersistentVolume{Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeAvailable}}
	newAvailablePvObj, _ := helper.ToUnstructured(availableNewPv)

	boundNewPV := &corev1.PersistentVolume{Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound}}
	newBoundPVObj, _ := helper.ToUnstructured(boundNewPV)

	releaseNewPV := &corev1.PersistentVolume{Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeReleased}}
	newReleasePVObj, _ := helper.ToUnstructured(releaseNewPV)

	failedNewPV := &corev1.PersistentVolume{Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeFailed}}
	newFailedPVObj, _ := helper.ToUnstructured(failedNewPV)

	pendingNewPV := &corev1.PersistentVolume{Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumePending}}
	newPendingPVObj, _ := helper.ToUnstructured(pendingNewPV)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update pvc status1",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems1,
			expectedObj:           newAvailablePvObj,
		},
		{
			name:                  "update pvc status2",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems2,
			expectedObj:           newReleasePVObj,
		},
		{
			name:                  "update pvc status3",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems3,
			expectedObj:           newBoundPVObj,
		},
		{
			name:                  "update pvc status4",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems4,
			expectedObj:           newFailedPVObj,
		},
		{
			name:                  "update pvc status5",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems5,
			expectedObj:           newPendingPVObj,
		},
		{
			name:                  "ignore update pvc status as up to date",
			curObj:                newAvailablePvObj,
			aggregatedStatusItems: aggregatedStatusItems1,
			expectedObj:           newAvailablePvObj,
		},
	}
	for _, tt := range tests {
		actualObj, _ := aggregatePersistentVolumeStatus(tt.curObj, tt.aggregatedStatusItems)
		assert.Equal(t, tt.expectedObj, actualObj, tt.name)
	}
}

func TestAggregatedPodDisruptionBudgetStatus(t *testing.T) {
	currPdbObj, _ := helper.ToUnstructured(&policyv1.PodDisruptionBudget{
		Status: policyv1.PodDisruptionBudgetStatus{
			CurrentHealthy:     1,
			DesiredHealthy:     1,
			DisruptionsAllowed: 1,
			ExpectedPods:       1,
		},
	})

	expectedPdbObj, _ := helper.ToUnstructured(&policyv1.PodDisruptionBudget{
		Status: policyv1.PodDisruptionBudgetStatus{
			CurrentHealthy:     2,
			DesiredHealthy:     2,
			DisruptionsAllowed: 2,
			ExpectedPods:       2,
		},
	})

	healthyStatusRaw, _ := helper.BuildStatusRawExtension(map[string]interface{}{
		"currentHealthy":     1,
		"desiredHealthy":     1,
		"disruptionsAllowed": 1,
		"expectedPods":       1,
	})

	evictionTime := metav1.Now()

	unHealthyStatusRaw, _ := helper.BuildStatusRawExtension(map[string]interface{}{
		"currentHealthy":     0,
		"desiredHealthy":     1,
		"disruptionsAllowed": 0,
		"expectedPods":       1,
		"disruptedPods": map[string]metav1.Time{
			"pod-1234": evictionTime,
		},
	})

	expectedUnhealthyPdbObj, _ := helper.ToUnstructured(&policyv1.PodDisruptionBudget{
		Status: policyv1.PodDisruptionBudgetStatus{
			CurrentHealthy:     0,
			DesiredHealthy:     2,
			DisruptionsAllowed: 0,
			ExpectedPods:       2,
			DisruptedPods: map[string]metav1.Time{
				"member1/pod-1234": evictionTime,
				"member2/pod-1234": evictionTime,
			},
		},
	})

	aggregateStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: healthyStatusRaw, Applied: true},
		{ClusterName: "member2", Status: healthyStatusRaw, Applied: true},
	}

	unhealthyAggregateStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: unHealthyStatusRaw, Applied: true},
		{ClusterName: "member2", Status: unHealthyStatusRaw, Applied: true},
	}

	for _, tt := range []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update pdb status",
			curObj:                currPdbObj,
			aggregatedStatusItems: aggregateStatusItems,
			expectedObj:           expectedPdbObj,
		},
		{
			name:                  "update pdb status with disrupted pods",
			curObj:                currPdbObj,
			aggregatedStatusItems: unhealthyAggregateStatusItems,
			expectedObj:           expectedUnhealthyPdbObj,
		},
		{
			name:                  "ignore update pdb status as up to date",
			curObj:                expectedPdbObj,
			aggregatedStatusItems: aggregateStatusItems,
			expectedObj:           expectedPdbObj,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actualObj, _ := aggregatePodDisruptionBudgetStatus(tt.curObj, tt.aggregatedStatusItems)
			assert.Equal(t, tt.expectedObj, actualObj)
		})
	}
}

func Test_aggregateCronJobStatus(t *testing.T) {
	currCronJobObj, _ := helper.ToUnstructured(&batchv1.CronJob{
		Status: batchv1.CronJobStatus{
			Active:             []corev1.ObjectReference{},
			LastScheduleTime:   nil,
			LastSuccessfulTime: nil,
		},
	})

	cronjobStatusRaw1, _ := helper.BuildStatusRawExtension(map[string]interface{}{
		"active": []corev1.ObjectReference{
			{
				APIVersion:      "batch/v1",
				Kind:            "Job",
				Name:            "foo",
				Namespace:       "default",
				ResourceVersion: "1",
				UID:             "1d5db04f-f2e8-4807-b6d4-7b78f402250d",
			},
		},
		"lastScheduleTime":   "2023-02-08T07:16:00Z",
		"lastSuccessfulTime": "2023-02-08T07:15:00Z",
	})
	cronjobStatusRaw2, _ := helper.BuildStatusRawExtension(map[string]interface{}{
		"active": []corev1.ObjectReference{
			{
				APIVersion:      "batch/v1",
				Kind:            "Job",
				Name:            "foo",
				Namespace:       "default",
				ResourceVersion: "1",
				UID:             "1d5db04f-f2e8-4807-b6d4-7b78f402250d",
			},
		},
		"lastScheduleTime":   "2023-02-08T07:17:00Z",
		"lastSuccessfulTime": "2023-02-08T07:17:00Z",
	})
	parse, _ := time.Parse("2006-01-02 15:04:05", "2023-02-08 07:17:00")
	successfulTime := metav1.NewTime(parse)
	expectedCronJobObj, _ := helper.ToUnstructured(&batchv1.CronJob{
		Status: batchv1.CronJobStatus{
			Active: []corev1.ObjectReference{
				{
					APIVersion:      "batch/v1",
					Kind:            "Job",
					Name:            "foo",
					Namespace:       "default",
					ResourceVersion: "1",
					UID:             "1d5db04f-f2e8-4807-b6d4-7b78f402250d",
				},
				{
					APIVersion:      "batch/v1",
					Kind:            "Job",
					Name:            "foo",
					Namespace:       "default",
					ResourceVersion: "1",
					UID:             "1d5db04f-f2e8-4807-b6d4-7b78f402250d",
				},
			},
			LastScheduleTime:   &successfulTime,
			LastSuccessfulTime: &successfulTime,
		},
	})

	aggregateStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: cronjobStatusRaw1, Applied: true},
		{ClusterName: "member2", Status: cronjobStatusRaw2, Applied: true},
	}
	for _, tt := range []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
	}{
		{
			name:                  "update cronjob status",
			curObj:                currCronJobObj,
			expectedObj:           expectedCronJobObj,
			aggregatedStatusItems: aggregateStatusItems,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actualObj, _ := aggregateCronJobStatus(tt.curObj, tt.aggregatedStatusItems)
			assert.Equal(t, tt.expectedObj, actualObj)
		})
	}
}
