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
	"time"

	"github.com/go-co-op/gocron"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/cronfederatedhpa/implementation"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/util/cronfederatedhpa"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type ReplicasScalerInterface interface {
	ScaleReplicas(cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error
}

type MinMaxThresholdScalerInterface interface {
	ScaleMinMaxThreshold(cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error
}

type CronFederatedHPAJob struct {
	client        client.Client
	eventRecorder record.EventRecorder
	scheduler     *gocron.Scheduler

	minMaxScaleHandler   map[cronfederatedhpa.ScalerKey]MinMaxThresholdScalerInterface
	replicasScaleHandler ReplicasScalerInterface

	namespaceName types.NamespacedName
	rule          autoscalingv1alpha1.CronFederatedHPARule
}

func NewCronFederatedHPAJob(client client.Client, eventRecorder record.EventRecorder, scheduler *gocron.Scheduler,
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) *CronFederatedHPAJob {
	// If you want to support other types, please implement the related interface and put the implementation struct in the map
	minMaxScaleHandler := map[cronfederatedhpa.ScalerKey]MinMaxThresholdScalerInterface{
		{ApiVersion: autoscalingv1alpha1.GroupVersion.String(), Kind: "FederatedHPA"}:            &implementation.FederatedHPAScaler{Client: client},
		{ApiVersion: autoscalingv2.SchemeGroupVersion.String(), Kind: "HorizontalPodAutoscaler"}: &implementation.HPAScaler{Client: client},
	}
	return &CronFederatedHPAJob{
		client:        client,
		eventRecorder: eventRecorder,
		scheduler:     scheduler,
		namespaceName: types.NamespacedName{
			Name:      cronFHPA.Name,
			Namespace: cronFHPA.Namespace,
		},
		replicasScaleHandler: &implementation.ReplicasScaler{Client: client},
		minMaxScaleHandler:   minMaxScaleHandler,
		rule:                 rule,
	}
}

func RunCronFederatedHPARule(c *CronFederatedHPAJob) {
	klog.V(4).Infof("Start to handle CronFederatedHPA %s", c.namespaceName)
	defer klog.V(4).Infof("End to handle CronFederatedHPA %s", c.namespaceName)

	var err error
	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{}
	err = c.client.Get(context.TODO(), c.namespaceName, cronFHPA)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("CronFederatedHPA(%s) not found", c.namespaceName)
		} else {
			// TODO: This may happen when the the network is down, we should do something here
			// But we are not sure what to do(retry not solve the problem)
			klog.Errorf("Get CronFederatedHPA(%s) failed: %v", c.namespaceName, err)
		}
		return
	}

	if helper.IsCronFederatedHPARuleSuspend(c.rule) {
		// If the rule is suspended, this job will be stopped soon
		klog.V(4).Infof("CronFederatedHPA(%s) Rule(%s) is suspended, skip it", c.namespaceName, c.rule.Name)
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

	if handler, ok := c.minMaxScaleHandler[cronfederatedhpa.ScalerKey{
		ApiVersion: cronFHPA.Spec.ScaleTargetRef.APIVersion, Kind: cronFHPA.Spec.ScaleTargetRef.Kind}]; ok {
		scaleErr = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			err = handler.ScaleMinMaxThreshold(cronFHPA, c.rule)
			return err
		})
		return
	}

	// scale workload directly
	scaleErr = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		err = c.replicasScaleHandler.ScaleReplicas(cronFHPA, c.rule)
		return err
	})
}

func (c *CronFederatedHPAJob) addFailedExecutionHistory(
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA, errMsg string) error {
	_, nextExecutionTime := c.scheduler.NextRun()

	// Add success history record, return false if there is no such rule
	addFailedHistoryFunc := func() bool {
		exists := false
		for index, rule := range cronFHPA.Status.ExecutionHistories {
			if rule.RuleName != c.rule.Name {
				continue
			}
			failedExecution := autoscalingv1alpha1.FailedExecution{
				ScheduleTime:  rule.NextExecutionTime,
				ExecutionTime: &metav1.Time{Time: time.Now()},
				Message:       errMsg,
			}
			historyLimits := helper.GetCronFederatedHPAFailedHistoryLimits(c.rule)
			if len(rule.FailedExecutions) > historyLimits-1 {
				rule.FailedExecutions = rule.FailedExecutions[:historyLimits-1]
			}
			cronFHPA.Status.ExecutionHistories[index].FailedExecutions =
				append([]autoscalingv1alpha1.FailedExecution{failedExecution}, rule.FailedExecutions...)
			cronFHPA.Status.ExecutionHistories[index].NextExecutionTime = &metav1.Time{Time: nextExecutionTime}
			exists = true
			break
		}

		return exists
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		// If this history not exist, it means the rule is suspended or deleted, so just ignore it.
		if exists := addFailedHistoryFunc(); !exists {
			return nil
		}

		updateErr := c.client.Status().Update(context.Background(), cronFHPA)
		if updateErr == nil {
			klog.V(4).Infof("CronFederatedHPA(%s/%s) status has been updated successfully", cronFHPA.Namespace, cronFHPA.Name)
			return nil
		}

		updated := &autoscalingv1alpha1.CronFederatedHPA{}
		if err = c.client.Get(context.Background(), client.ObjectKey{Namespace: cronFHPA.Namespace, Name: cronFHPA.Name}, updated); err == nil {
			cronFHPA = updated
		} else {
			klog.Errorf("Get CronFederatedHPA(%s/%s) failed: %v", cronFHPA.Namespace, cronFHPA.Name, err)
		}
		return updateErr
	})
}

func (c *CronFederatedHPAJob) addSuccessExecutionHistory(
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA,
	appliedReplicas, appliedMinReplicas, appliedMaxReplicas *int32) error {
	_, nextExecutionTime := c.scheduler.NextRun()

	// Add success history record, return false if there is no such rule
	addSuccessHistoryFunc := func() bool {
		exists := false
		for index, rule := range cronFHPA.Status.ExecutionHistories {
			if rule.RuleName != c.rule.Name {
				continue
			}
			successExecution := autoscalingv1alpha1.SuccessfulExecution{
				ScheduleTime:       rule.NextExecutionTime,
				ExecutionTime:      &metav1.Time{Time: time.Now()},
				AppliedReplicas:    appliedReplicas,
				AppliedMaxReplicas: appliedMaxReplicas,
				AppliedMinReplicas: appliedMinReplicas,
			}
			historyLimits := helper.GetCronFederatedHPASuccessHistoryLimits(c.rule)
			if len(rule.SuccessfulExecutions) > historyLimits-1 {
				rule.SuccessfulExecutions = rule.SuccessfulExecutions[:historyLimits-1]
			}
			cronFHPA.Status.ExecutionHistories[index].SuccessfulExecutions =
				append([]autoscalingv1alpha1.SuccessfulExecution{successExecution}, rule.SuccessfulExecutions...)
			cronFHPA.Status.ExecutionHistories[index].NextExecutionTime = &metav1.Time{Time: nextExecutionTime}
			exists = true
			break
		}

		return exists
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		// If this history not exist, it means the rule deleted, so just ignore it.
		if exists := addSuccessHistoryFunc(); !exists {
			return nil
		}

		updateErr := c.client.Status().Update(context.Background(), cronFHPA)
		if updateErr == nil {
			klog.V(4).Infof("CronFederatedHPA(%s/%s) status has been updated successfully", cronFHPA.Namespace, cronFHPA.Name)
			return err
		}

		updated := &autoscalingv1alpha1.CronFederatedHPA{}
		if err = c.client.Get(context.Background(), client.ObjectKey{Namespace: cronFHPA.Namespace, Name: cronFHPA.Name}, updated); err == nil {
			cronFHPA = updated
		} else {
			klog.Errorf("Get CronFederatedHPA(%s/%s) failed: %v", cronFHPA.Namespace, cronFHPA.Name, err)
		}
		return updateErr
	})
}
