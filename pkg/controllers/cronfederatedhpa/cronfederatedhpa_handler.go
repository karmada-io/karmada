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
	"fmt"
	"sync"
	"time"
	_ "time/tzdata" // import tzdata to support time zone parsing, this is needed by function time.LoadLocation

	"github.com/go-co-op/gocron"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// RuleCron is the wrapper of gocron.Scheduler and CronFederatedHPARule
type RuleCron struct {
	*gocron.Scheduler
	autoscalingv1alpha1.CronFederatedHPARule
}

// CronHandler is the handler for CronFederatedHPA
type CronHandler struct {
	client        client.Client
	eventRecorder record.EventRecorder

	// cronExecutorMap is [cronFederatedHPA name][rule name]RuleCron
	cronExecutorMap map[string]map[string]RuleCron
	executorLock    sync.RWMutex

	// cronFHPAScaleTargetMap is [cronFHPA name]CrossVersionObjectReference
	cronFHPAScaleTargetMap map[string]autoscalingv2.CrossVersionObjectReference
	scaleTargetLock        sync.RWMutex
}

// NewCronHandler creates new cron handler
func NewCronHandler(client client.Client, eventRecorder record.EventRecorder) *CronHandler {
	return &CronHandler{
		client:                 client,
		eventRecorder:          eventRecorder,
		cronExecutorMap:        make(map[string]map[string]RuleCron),
		cronFHPAScaleTargetMap: make(map[string]autoscalingv2.CrossVersionObjectReference),
	}
}

// CronFHPAScaleTargetRefUpdates checks if the scale target changed
func (c *CronHandler) CronFHPAScaleTargetRefUpdates(cronFHPAKey string, scaleTarget autoscalingv2.CrossVersionObjectReference) bool {
	c.scaleTargetLock.Lock()
	defer c.scaleTargetLock.Unlock()

	origTarget, ok := c.cronFHPAScaleTargetMap[cronFHPAKey]
	if !ok {
		c.cronFHPAScaleTargetMap[cronFHPAKey] = scaleTarget
		return false
	}

	return !equality.Semantic.DeepEqual(origTarget, scaleTarget)
}

// AddCronExecutorIfNotExist creates the executor for CronFederatedHPA if not exist
func (c *CronHandler) AddCronExecutorIfNotExist(cronFHPAKey string) {
	c.executorLock.Lock()
	defer c.executorLock.Unlock()

	if _, ok := c.cronExecutorMap[cronFHPAKey]; ok {
		return
	}

	c.cronExecutorMap[cronFHPAKey] = make(map[string]RuleCron)
}

// RuleCronExecutorExists checks if the executor for specific CronFederatedHPA rule exists
func (c *CronHandler) RuleCronExecutorExists(cronFHPAKey string,
	ruleName string) (autoscalingv1alpha1.CronFederatedHPARule, bool) {
	c.executorLock.RLock()
	defer c.executorLock.RUnlock()

	if _, ok := c.cronExecutorMap[cronFHPAKey]; !ok {
		return autoscalingv1alpha1.CronFederatedHPARule{}, false
	}
	cronRule, exists := c.cronExecutorMap[cronFHPAKey][ruleName]
	return cronRule.CronFederatedHPARule, exists
}

// StopRuleExecutor stops the executor for specific CronFederatedHPA rule
func (c *CronHandler) StopRuleExecutor(cronFHPAKey string, ruleName string) {
	c.executorLock.Lock()
	defer c.executorLock.Unlock()

	if _, ok := c.cronExecutorMap[cronFHPAKey]; !ok {
		return
	}
	if _, ok := c.cronExecutorMap[cronFHPAKey][ruleName]; !ok {
		return
	}
	c.cronExecutorMap[cronFHPAKey][ruleName].Stop()
	delete(c.cronExecutorMap[cronFHPAKey], ruleName)
}

// StopCronFHPAExecutor stops the executor for specific CronFederatedHPA
func (c *CronHandler) StopCronFHPAExecutor(cronFHPAKey string) {
	c.executorLock.Lock()
	defer c.executorLock.Unlock()

	if _, ok := c.cronExecutorMap[cronFHPAKey]; !ok {
		return
	}
	for _, scheduler := range c.cronExecutorMap[cronFHPAKey] {
		scheduler.Stop()
	}

	delete(c.cronExecutorMap, cronFHPAKey)
}

// CreateCronJobForExecutor creates the executor for a rule of CronFederatedHPA
func (c *CronHandler) CreateCronJobForExecutor(cronFHPA *autoscalingv1alpha1.CronFederatedHPA,
	rule autoscalingv1alpha1.CronFederatedHPARule) error {
	var err error
	timeZone := time.Now().Location()

	if rule.TimeZone != nil {
		timeZone, err = time.LoadLocation(*rule.TimeZone)
		if err != nil {
			// This should not happen because there is validation in webhook
			klog.ErrorS(err, "Invalid CronFederatedHPA rule time zone", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name, "rule", rule.Name, "timeZone", *rule.TimeZone)
			return err
		}
	}

	scheduler := gocron.NewScheduler(timeZone)
	cronJob := NewCronFederatedHPAJob(c.client, c.eventRecorder, scheduler, cronFHPA, rule)
	if _, err := scheduler.Cron(rule.Schedule).Do(RunCronFederatedHPARule, cronJob); err != nil {
		klog.ErrorS(err, "Create cron job for CronFederatedHPA rule error", "namespace", cronFHPA.Namespace, "name", cronFHPA.Name, "rule", rule.Name)
		return err
	}
	scheduler.StartAsync()

	cronFHPAKey := helper.GetCronFederatedHPAKey(cronFHPA)
	c.executorLock.Lock()
	defer c.executorLock.Unlock()
	ruleExecutorMap := c.cronExecutorMap[cronFHPAKey]
	ruleExecutorMap[rule.Name] = RuleCron{Scheduler: scheduler, CronFederatedHPARule: rule}
	return nil
}

// GetRuleNextExecuteTime returns the next execute time of a rule of CronFederatedHPA
func (c *CronHandler) GetRuleNextExecuteTime(cronFHPA *autoscalingv1alpha1.CronFederatedHPA, ruleName string) (time.Time, error) {
	c.executorLock.RLock()
	defer c.executorLock.RUnlock()

	if _, ok := c.cronExecutorMap[helper.GetCronFederatedHPAKey(cronFHPA)]; !ok {
		return time.Time{}, fmt.Errorf("CronFederatedHPA(%s/%s) not start", cronFHPA.Namespace, cronFHPA.Name)
	}

	ruleCron, exists := c.cronExecutorMap[helper.GetCronFederatedHPAKey(cronFHPA)][ruleName]
	if !exists {
		return time.Time{}, fmt.Errorf("CronFederatedHPA(%s/%s) rule(%s) not exist", cronFHPA.Namespace, cronFHPA.Name, ruleName)
	}

	_, next := ruleCron.Scheduler.NextRun()
	return next, nil
}
