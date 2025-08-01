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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "cronfederatedhpa-controller"
)

// CronFHPAController is used to operate CronFederatedHPA.
type CronFHPAController struct {
	client.Client // used to operate Cron resources.
	EventRecorder record.EventRecorder

	RateLimiterOptions ratelimiterflag.Options
	CronHandler        *CronHandler
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *CronFHPAController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling CronFederatedHPA", "namespace", req.Namespace, "name", req.Name)

	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{}
	if err := c.Client.Get(ctx, req.NamespacedName, cronFHPA); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).InfoS("Begin to cleanup the cron jobs for CronFederatedHPA", "namespace", req.Namespace, "name", req.Name)
			c.CronHandler.StopCronFHPAExecutor(req.NamespacedName.String())
			return controllerruntime.Result{}, nil
		}

		klog.ErrorS(err, "Fail to get CronFederatedHPA", "namespace", req.Namespace, "name", req.Name)
		return controllerruntime.Result{}, err
	}

	//  If this CronFederatedHPA is deleting, stop all related cron executors
	if !cronFHPA.DeletionTimestamp.IsZero() {
		c.CronHandler.StopCronFHPAExecutor(req.NamespacedName.String())
		return controllerruntime.Result{}, nil
	}

	var err error
	startTime := time.Now()
	defer func() {
		metrics.ObserveProcessCronFederatedHPALatency(err, startTime)
	}()

	origRuleSets := sets.New[string]()
	for _, history := range cronFHPA.Status.ExecutionHistories {
		origRuleSets.Insert(history.RuleName)
	}

	// If scale target is updated, stop all the rule executors, and next steps will create the new executors
	if c.CronHandler.CronFHPAScaleTargetRefUpdates(req.NamespacedName.String(), cronFHPA.Spec.ScaleTargetRef) {
		c.CronHandler.StopCronFHPAExecutor(req.NamespacedName.String())
	}

	c.CronHandler.AddCronExecutorIfNotExist(req.NamespacedName.String())

	newRuleSets := sets.New[string]()
	for _, rule := range cronFHPA.Spec.Rules {
		if err = c.processCronRule(ctx, cronFHPA, rule); err != nil {
			return controllerruntime.Result{}, err
		}
		newRuleSets.Insert(rule.Name)
	}

	// If rule is deleted, remove the rule executor from the handler
	for name := range origRuleSets {
		if newRuleSets.Has(name) {
			continue
		}
		c.CronHandler.StopRuleExecutor(req.NamespacedName.String(), name)
		if err = c.removeCronFHPAHistory(ctx, cronFHPA, name); err != nil {
			return controllerruntime.Result{}, err
		}
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *CronFHPAController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.CronHandler = NewCronHandler(mgr.GetClient(), mgr.GetEventRecorderFor(ControllerName))
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&autoscalingv1alpha1.CronFederatedHPA{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(c)
}

// processCronRule processes the cron rule
func (c *CronFHPAController) processCronRule(ctx context.Context, cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error {
	cronFHPAKey := helper.GetCronFederatedHPAKey(cronFHPA)
	if ruleOld, exists := c.CronHandler.RuleCronExecutorExists(cronFHPAKey, rule.Name); exists {
		if equality.Semantic.DeepEqual(ruleOld, rule) {
			return nil
		}
		c.CronHandler.StopRuleExecutor(cronFHPAKey, rule.Name)
	}

	if !helper.IsCronFederatedHPARuleSuspend(rule) {
		if err := c.CronHandler.CreateCronJobForExecutor(cronFHPA, rule); err != nil {
			c.EventRecorder.Event(cronFHPA, corev1.EventTypeWarning, "StartRuleFailed", err.Error())
			klog.ErrorS(err, "Fail to start cron for CronFederatedHPA rule", "cronFederatedHPA", cronFHPAKey, "rule", rule.Name)
			return err
		}
	}

	if err := c.updateRuleHistory(ctx, cronFHPA, rule); err != nil {
		c.EventRecorder.Event(cronFHPA, corev1.EventTypeWarning, "UpdateCronFederatedHPAFailed", err.Error())
		return err
	}
	return nil
}

// updateRuleHistory updates the rule history
func (c *CronFHPAController) updateRuleHistory(ctx context.Context, cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error {
	var nextExecutionTime *metav1.Time
	if !helper.IsCronFederatedHPARuleSuspend(rule) {
		// If rule is not suspended, we should set the nextExecutionTime filed, or the nextExecutionTime will be nil
		next, err := c.CronHandler.GetRuleNextExecuteTime(cronFHPA, rule.Name)
		if err != nil {
			klog.ErrorS(err, "Fail to get next execution time for CronFederatedHPA rule", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name, "rule", rule.Name)
			return err
		}
		nextExecutionTime = &metav1.Time{Time: next}
	}

	exists := false
	for index, history := range cronFHPA.Status.ExecutionHistories {
		if history.RuleName != rule.Name {
			continue
		}
		exists = true
		cronFHPA.Status.ExecutionHistories[index].NextExecutionTime = nextExecutionTime
		break
	}

	if !exists {
		ruleHistory := autoscalingv1alpha1.ExecutionHistory{
			RuleName:          rule.Name,
			NextExecutionTime: nextExecutionTime,
		}
		cronFHPA.Status.ExecutionHistories = append(cronFHPA.Status.ExecutionHistories, ruleHistory)
	}

	if err := c.Client.Status().Update(ctx, cronFHPA); err != nil {
		klog.ErrorS(err, "Fail to update CronFederatedHPA rule's next execution time", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name, "rule", rule.Name)
		return err
	}

	return nil
}

// removeCronFHPAHistory removes the rule history in status
func (c *CronFHPAController) removeCronFHPAHistory(ctx context.Context, cronFHPA *autoscalingv1alpha1.CronFederatedHPA, ruleName string) error {
	exists := false
	for index, history := range cronFHPA.Status.ExecutionHistories {
		if history.RuleName != ruleName {
			continue
		}
		cronFHPA.Status.ExecutionHistories = append(cronFHPA.Status.ExecutionHistories[:index], cronFHPA.Status.ExecutionHistories[index+1:]...)
		exists = true
		break
	}

	if !exists {
		return nil
	}
	if err := c.Client.Status().Update(ctx, cronFHPA); err != nil {
		c.EventRecorder.Event(cronFHPA, corev1.EventTypeWarning, "UpdateCronFederatedHPAFailed", err.Error())
		klog.ErrorS(err, "Fail to remove CronFederatedHPA rule history", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name, "rule", ruleName)
		return err
	}

	return nil
}
