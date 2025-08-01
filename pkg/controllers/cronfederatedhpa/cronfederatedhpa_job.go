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

package cronfederatedhpa

import (
	"context"
	"fmt"
	"time"

	"github.com/go-co-op/gocron"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// ScalingJob is a job to handle CronFederatedHPA.
type ScalingJob struct {
	client        client.Client
	eventRecorder record.EventRecorder
	scheduler     *gocron.Scheduler

	namespaceName types.NamespacedName
	rule          autoscalingv1alpha1.CronFederatedHPARule
}

// NewCronFederatedHPAJob creates a new CronFederatedHPAJob.
func NewCronFederatedHPAJob(client client.Client, eventRecorder record.EventRecorder, scheduler *gocron.Scheduler,
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) *ScalingJob {
	return &ScalingJob{
		client:        client,
		eventRecorder: eventRecorder,
		scheduler:     scheduler,
		namespaceName: types.NamespacedName{
			Name:      cronFHPA.Name,
			Namespace: cronFHPA.Namespace,
		},
		rule: rule,
	}
}

// RunCronFederatedHPARule runs the job to handle CronFederatedHPA.
func RunCronFederatedHPARule(c *ScalingJob) {
	klog.V(4).Infof("Start to handle CronFederatedHPA %s", c.namespaceName)
	defer klog.V(4).Infof("End to handle CronFederatedHPA %s", c.namespaceName)

	var err error
	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{}
	err = c.client.Get(context.TODO(), c.namespaceName, cronFHPA)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("CronFederatedHPA not found", "cronFederatedHPA", c.namespaceName)
		} else {
			// TODO: This may happen when the the network is down, we should do something here
			// But we are not sure what to do(retry not solve the problem)
			klog.ErrorS(err, "Get CronFederatedHPA failed", "cronFederatedHPA", c.namespaceName)
		}
		return
	}

	if helper.IsCronFederatedHPARuleSuspend(c.rule) {
		// If the rule is suspended, this job will be stopped soon
		klog.V(4).InfoS("CronFederatedHPA rule is suspended, skip it", "cronFederatedHPA", c.namespaceName, "rule", c.rule.Name)
		return
	}

	var scaleErr error
	start := time.Now()
	defer func() {
		metrics.ObserveProcessCronFederatedHPARuleLatency(scaleErr, start)
		if scaleErr != nil {
			c.eventRecorder.Event(cronFHPA, corev1.EventTypeWarning, "ScaleFailed", scaleErr.Error())
			err = c.addFailedExecutionHistory(cronFHPA, scaleErr.Error())
		} else {
			err = c.addSuccessExecutionHistory(cronFHPA, c.rule.TargetReplicas, c.rule.TargetMinReplicas, c.rule.TargetMaxReplicas)
		}
		if err != nil {
			c.eventRecorder.Event(cronFHPA, corev1.EventTypeWarning, "UpdateStatusFailed", err.Error())
		}
	}()

	if cronFHPA.Spec.ScaleTargetRef.APIVersion == autoscalingv1alpha1.GroupVersion.String() {
		if cronFHPA.Spec.ScaleTargetRef.Kind != autoscalingv1alpha1.FederatedHPAKind {
			scaleErr = fmt.Errorf("CronFederatedHPA(%s) do not support scale target %s/%s",
				c.namespaceName, cronFHPA.Spec.ScaleTargetRef.APIVersion, cronFHPA.Spec.ScaleTargetRef.Kind)
			return
		}

		scaleErr = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			err = c.ScaleFHPA(cronFHPA)
			return err
		})
		return
	}

	// scale workload directly
	scaleErr = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		err = c.ScaleWorkloads(cronFHPA)
		return err
	})
}

// ScaleFHPA scales FederatedHPA's minReplicas and maxReplicas
func (c *ScalingJob) ScaleFHPA(cronFHPA *autoscalingv1alpha1.CronFederatedHPA) error {
	fhpaName := types.NamespacedName{
		Namespace: cronFHPA.Namespace,
		Name:      cronFHPA.Spec.ScaleTargetRef.Name,
	}

	fhpa := &autoscalingv1alpha1.FederatedHPA{}
	err := c.client.Get(context.TODO(), fhpaName, fhpa)
	if err != nil {
		return err
	}

	update := false
	if c.rule.TargetMaxReplicas != nil && fhpa.Spec.MaxReplicas != *c.rule.TargetMaxReplicas {
		fhpa.Spec.MaxReplicas = *c.rule.TargetMaxReplicas
		update = true
	}
	if c.rule.TargetMinReplicas != nil && *fhpa.Spec.MinReplicas != *c.rule.TargetMinReplicas {
		*fhpa.Spec.MinReplicas = *c.rule.TargetMinReplicas
		update = true
	}

	if update {
		err := c.client.Update(context.TODO(), fhpa)
		if err != nil {
			klog.ErrorS(err, "CronFederatedHPA updates FederatedHPA failed", "namespace", c.namespaceName.Namespace, "cronFederatedHPA", c.namespaceName.Name, "federatedHPA", fhpa.Name)
			return err
		}
		klog.V(4).InfoS("CronFederatedHPA scales FederatedHPA successfully", "namespace", c.namespaceName.Namespace, "cronFederatedHPA", c.namespaceName.Name, "federatedHPA", fhpa.Name)
		return nil
	}

	klog.V(4).InfoS("CronFederatedHPA find nothing updated for FederatedHPA, skip it", "namespace", c.namespaceName.Namespace, "cronFederatedHPA", c.namespaceName.Name, "federatedHPA", fhpa.Name)
	return nil
}

// ScaleWorkloads scales workload's replicas directly
func (c *ScalingJob) ScaleWorkloads(cronFHPA *autoscalingv1alpha1.CronFederatedHPA) error {
	ctx := context.Background()

	scaleClient := c.client.SubResource("scale")

	targetGV, err := schema.ParseGroupVersion(cronFHPA.Spec.ScaleTargetRef.APIVersion)
	if err != nil {
		klog.ErrorS(err, "CronFederatedHPA parses GroupVersion failed", "cronFederatedHPA", c.namespaceName, "groupVersion", cronFHPA.Spec.ScaleTargetRef.APIVersion)
		return err
	}
	targetGVK := schema.GroupVersionKind{
		Group:   targetGV.Group,
		Kind:    cronFHPA.Spec.ScaleTargetRef.Kind,
		Version: targetGV.Version,
	}
	targetResource := &unstructured.Unstructured{}
	targetResource.SetGroupVersionKind(targetGVK)
	err = c.client.Get(ctx, types.NamespacedName{Namespace: cronFHPA.Namespace, Name: cronFHPA.Spec.ScaleTargetRef.Name}, targetResource)
	if err != nil {
		klog.ErrorS(err, "Get resource failed", "namespace", cronFHPA.Namespace, "name", cronFHPA.Spec.ScaleTargetRef.Name)
		return err
	}

	scaleObj := &unstructured.Unstructured{}
	err = scaleClient.Get(ctx, targetResource, scaleObj)
	if err != nil {
		klog.ErrorS(err, "Get Scale for resource failed", "namespace", cronFHPA.Namespace, "name", cronFHPA.Spec.ScaleTargetRef.Name)
		return err
	}

	scale := &autoscalingv1.Scale{}
	err = helper.ConvertToTypedObject(scaleObj, scale)
	if err != nil {
		klog.ErrorS(err, "Convert Scale failed", "namespace", cronFHPA.Namespace, "name", cronFHPA.Spec.ScaleTargetRef.Name)
		return err
	}

	if scale.Spec.Replicas != *c.rule.TargetReplicas {
		if err := helper.ApplyReplica(scaleObj, int64(*c.rule.TargetReplicas), util.ReplicasField); err != nil {
			klog.ErrorS(err, "CronFederatedHPA applies Replicas failed", "cronFederatedHPA", c.namespaceName, "namespace", cronFHPA.Namespace, "name", cronFHPA.Spec.ScaleTargetRef.Name)
			return err
		}
		err := scaleClient.Update(ctx, targetResource, client.WithSubResourceBody(scaleObj))
		if err != nil {
			klog.ErrorS(err, "CronFederatedHPA updates scale resource failed", "cronFederatedHPA", c.namespaceName, "namespace", cronFHPA.Namespace, "name", cronFHPA.Spec.ScaleTargetRef.Name)
			return err
		}
		klog.V(4).InfoS("CronFederatedHPA scales resource successfully", "cronFederatedHPA", c.namespaceName, "namespace", cronFHPA.Namespace, "name", cronFHPA.Spec.ScaleTargetRef.Name)
		return nil
	}
	return nil
}

func (c *ScalingJob) addFailedExecutionHistory(
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA, errMsg string) error {
	_, nextExecutionTime := c.scheduler.NextRun()

	// Add failed history record
	addFailedHistoryFunc := func(index int) {
		failedExecution := autoscalingv1alpha1.FailedExecution{
			ScheduleTime:  cronFHPA.Status.ExecutionHistories[index].NextExecutionTime,
			ExecutionTime: &metav1.Time{Time: time.Now()},
			Message:       errMsg,
		}
		historyLimits := helper.GetCronFederatedHPAFailedHistoryLimits(c.rule)
		if len(cronFHPA.Status.ExecutionHistories[index].FailedExecutions) > historyLimits-1 {
			cronFHPA.Status.ExecutionHistories[index].FailedExecutions = cronFHPA.Status.ExecutionHistories[index].FailedExecutions[:historyLimits-1]
		}
		cronFHPA.Status.ExecutionHistories[index].FailedExecutions =
			append([]autoscalingv1alpha1.FailedExecution{failedExecution}, cronFHPA.Status.ExecutionHistories[index].FailedExecutions...)
		cronFHPA.Status.ExecutionHistories[index].NextExecutionTime = &metav1.Time{Time: nextExecutionTime}
	}

	index := c.findExecutionHistory(cronFHPA.Status.ExecutionHistories)
	if index < 0 {
		// The failed history does not exist, it means the rule deleted, so just ignore it.
		return nil
	}

	var operationResult controllerutil.OperationResult
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = helper.UpdateStatus(context.Background(), c.client, cronFHPA, func() error {
			addFailedHistoryFunc(index)
			return nil
		})
		return err
	}); err != nil {
		klog.ErrorS(err, "Failed to add failed history record to CronFederatedHPA", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name)
		return err
	}

	if operationResult == controllerutil.OperationResultUpdatedStatusOnly {
		klog.V(4).InfoS("CronFederatedHPA status has been updated successfully", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name)
	}

	return nil
}

// findExecutionHistory finds the history record, returns -1 if there is no such rule.
func (c *ScalingJob) findExecutionHistory(histories []autoscalingv1alpha1.ExecutionHistory) int {
	for index, rule := range histories {
		if rule.RuleName == c.rule.Name {
			return index
		}
	}
	return -1
}

func (c *ScalingJob) addSuccessExecutionHistory(
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA,
	appliedReplicas, appliedMinReplicas, appliedMaxReplicas *int32) error {
	_, nextExecutionTime := c.scheduler.NextRun()

	// Add success history record
	addSuccessHistoryFunc := func(index int) {
		successExecution := autoscalingv1alpha1.SuccessfulExecution{
			ScheduleTime:       cronFHPA.Status.ExecutionHistories[index].NextExecutionTime,
			ExecutionTime:      &metav1.Time{Time: time.Now()},
			AppliedReplicas:    appliedReplicas,
			AppliedMaxReplicas: appliedMaxReplicas,
			AppliedMinReplicas: appliedMinReplicas,
		}
		historyLimits := helper.GetCronFederatedHPASuccessHistoryLimits(c.rule)
		if len(cronFHPA.Status.ExecutionHistories[index].SuccessfulExecutions) > historyLimits-1 {
			cronFHPA.Status.ExecutionHistories[index].SuccessfulExecutions = cronFHPA.Status.ExecutionHistories[index].SuccessfulExecutions[:historyLimits-1]
		}
		cronFHPA.Status.ExecutionHistories[index].SuccessfulExecutions =
			append([]autoscalingv1alpha1.SuccessfulExecution{successExecution}, cronFHPA.Status.ExecutionHistories[index].SuccessfulExecutions...)
		cronFHPA.Status.ExecutionHistories[index].NextExecutionTime = &metav1.Time{Time: nextExecutionTime}
	}

	index := c.findExecutionHistory(cronFHPA.Status.ExecutionHistories)
	if index < 0 {
		// The success history does not exist, it means the rule deleted, so just ignore it.
		return nil
	}

	var operationResult controllerutil.OperationResult
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = helper.UpdateStatus(context.Background(), c.client, cronFHPA, func() error {
			addSuccessHistoryFunc(index)
			return nil
		})
		return err
	}); err != nil {
		klog.ErrorS(err, "Failed to add success history record to CronFederatedHPA", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name)
		return err
	}

	if operationResult == controllerutil.OperationResultUpdatedStatusOnly {
		klog.V(4).InfoS("CronFederatedHPA status has been updated successfully", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name)
	}

	return nil
}
