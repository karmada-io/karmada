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

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type CronFederatedHPAJob struct {
	client        client.Client
	eventRecorder record.EventRecorder
	scheduler     *gocron.Scheduler

	namespaceName types.NamespacedName
	rule          autoscalingv1alpha1.CronFederatedHPARule
}

func NewCronFederatedHPAJob(client client.Client, eventRecorder record.EventRecorder, scheduler *gocron.Scheduler,
	cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) *CronFederatedHPAJob {
	return &CronFederatedHPAJob{
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
	defer func() {
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

func (c *CronFederatedHPAJob) ScaleFHPA(cronFHPA *autoscalingv1alpha1.CronFederatedHPA) error {
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
			klog.Errorf("CronFederatedHPA(%s) updates FederatedHPA(%s/%s) failed: %v",
				c.namespaceName, fhpa.Namespace, fhpa.Name, err)
			return err
		}
		klog.V(4).Infof("CronFederatedHPA(%s) scales FederatedHPA(%s/%s) successfully",
			c.namespaceName, fhpa.Namespace, fhpa.Name)
		return nil
	}

	klog.V(4).Infof("CronFederatedHPA(%s) find nothing updated for FederatedHPA(%s/%s), skip it",
		c.namespaceName, fhpa.Namespace, fhpa.Name)
	return nil
}

func (c *CronFederatedHPAJob) ScaleWorkloads(cronFHPA *autoscalingv1alpha1.CronFederatedHPA) error {
	ctx := context.Background()

	scaleClient := c.client.SubResource("scale")

	targetGV, err := schema.ParseGroupVersion(cronFHPA.Spec.ScaleTargetRef.APIVersion)
	if err != nil {
		klog.Errorf("CronFederatedHPA(%s) parses GroupVersion(%s) failed: %v",
			c.namespaceName, cronFHPA.Spec.ScaleTargetRef.APIVersion, err)
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
		klog.Errorf("Get Resource(%s/%s) failed: %v", cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, err)
		return err
	}

	scaleObj := &unstructured.Unstructured{}
	err = scaleClient.Get(ctx, targetResource, scaleObj)
	if err != nil {
		klog.Errorf("Get Scale for resource(%s/%s) failed: %v", cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, err)
		return err
	}

	scale := &autoscalingv1.Scale{}
	err = helper.ConvertToTypedObject(scaleObj, scale)
	if err != nil {
		klog.Errorf("Convert Scale failed: %v", err)
		return err
	}

	if scale.Spec.Replicas != *c.rule.TargetReplicas {
		if err := helper.ApplyReplica(scaleObj, int64(*c.rule.TargetReplicas), util.ReplicasField); err != nil {
			klog.Errorf("CronFederatedHPA(%s) applies Replicas for %s/%s failed: %v",
				c.namespaceName, cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, err)
			return err
		}
		err := scaleClient.Update(ctx, targetResource, client.WithSubResourceBody(scaleObj))
		if err != nil {
			klog.Errorf("CronFederatedHPA(%s) updates scale resource failed: %v", c.namespaceName, err)
			return err
		}
		klog.V(4).Infof("CronFederatedHPA(%s) scales resource(%s/%s) successfully",
			c.namespaceName, cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name)
		return nil
	}
	return nil
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
	appliedReplicas, appliedMaxReplicas, appliedMinReplicas *int32) error {
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
