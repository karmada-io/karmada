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

package cronfederatedhpa

import (
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestCronFHPAScaleTargetRefUpdates(t *testing.T) {
	tests := []struct {
		name           string
		cronFHPAKey    string
		initialTarget  autoscalingv2.CrossVersionObjectReference
		updatedTarget  autoscalingv2.CrossVersionObjectReference
		expectedUpdate bool
	}{
		{
			name:          "New scale target",
			cronFHPAKey:   "default/new-cronhpa",
			initialTarget: autoscalingv2.CrossVersionObjectReference{}, // Empty for new target
			updatedTarget: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test-deployment",
				APIVersion: "apps/v1",
			},
			expectedUpdate: false,
		},
		{
			name:        "Same scale target",
			cronFHPAKey: "default/test-cronhpa",
			initialTarget: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test-deployment",
				APIVersion: "apps/v1",
			},
			updatedTarget: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test-deployment",
				APIVersion: "apps/v1",
			},
			expectedUpdate: false,
		},
		{
			name:        "Different scale target",
			cronFHPAKey: "default/test-cronhpa",
			initialTarget: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test-deployment",
				APIVersion: "apps/v1",
			},
			updatedTarget: autoscalingv2.CrossVersionObjectReference{
				Kind:       "StatefulSet",
				Name:       "test-statefulset",
				APIVersion: "apps/v1",
			},
			expectedUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

			// Empty initialTarget will be skipped
			if tt.initialTarget != (autoscalingv2.CrossVersionObjectReference{}) {
				updated := handler.CronFHPAScaleTargetRefUpdates(tt.cronFHPAKey, tt.initialTarget)
				assert.False(t, updated, "Initial target setting should return false")
			}

			updated := handler.CronFHPAScaleTargetRefUpdates(tt.cronFHPAKey, tt.updatedTarget)
			assert.Equal(t, tt.expectedUpdate, updated, "Unexpected result for %s", tt.name)
		})
	}
}

func TestAddCronExecutorIfNotExist(t *testing.T) {
	handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

	cronFHPAKey := "default/test-cronhpa"

	// Adding new executor
	handler.AddCronExecutorIfNotExist(cronFHPAKey)
	assert.Contains(t, handler.cronExecutorMap, cronFHPAKey, "Executor should be added")

	// Adding existing executor
	originalLen := len(handler.cronExecutorMap)
	handler.AddCronExecutorIfNotExist(cronFHPAKey)
	assert.Equal(t, originalLen, len(handler.cronExecutorMap), "Existing executor should not be added again")
}

func TestRuleCronExecutorExists(t *testing.T) {
	handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

	cronFHPAKey := "default/test-cronhpa"
	ruleName := "test-rule"

	// non-existent executor
	_, exists := handler.RuleCronExecutorExists(cronFHPAKey, ruleName)
	assert.False(t, exists, "Non-existent executor should return false")

	// Add an executor
	handler.AddCronExecutorIfNotExist(cronFHPAKey)
	handler.cronExecutorMap[cronFHPAKey][ruleName] = RuleCron{
		CronFederatedHPARule: autoscalingv1alpha1.CronFederatedHPARule{
			Name: ruleName,
		},
	}

	rule, exists := handler.RuleCronExecutorExists(cronFHPAKey, ruleName)
	assert.True(t, exists, "Existing executor should return true")
	assert.Equal(t, ruleName, rule.Name, "Returned rule should match the added rule")
}

func TestStopRuleExecutor(t *testing.T) {
	handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

	cronFHPAKey := "default/test-cronhpa"
	ruleName := "test-rule"

	handler.cronExecutorMap = make(map[string]map[string]RuleCron)
	handler.cronExecutorMap[cronFHPAKey] = make(map[string]RuleCron)
	handler.cronExecutorMap[cronFHPAKey][ruleName] = RuleCron{
		Scheduler: gocron.NewScheduler(time.UTC),
	}
	_, err := handler.cronExecutorMap[cronFHPAKey][ruleName].Scheduler.Every(1).Minute().Do(func() {})
	assert.NoError(t, err)
	handler.cronExecutorMap[cronFHPAKey][ruleName].Scheduler.StartAsync()

	assert.True(t, handler.cronExecutorMap[cronFHPAKey][ruleName].Scheduler.IsRunning())

	handler.StopRuleExecutor(cronFHPAKey, ruleName)

	assert.NotContains(t, handler.cronExecutorMap[cronFHPAKey], ruleName)
}

func TestStopCronFHPAExecutor(t *testing.T) {
	handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

	cronFHPAKey := "default/test-cronhpa"
	ruleName1 := "test-rule-1"
	ruleName2 := "test-rule-2"

	handler.cronExecutorMap = make(map[string]map[string]RuleCron)
	handler.cronExecutorMap[cronFHPAKey] = make(map[string]RuleCron)
	handler.cronExecutorMap[cronFHPAKey][ruleName1] = RuleCron{
		Scheduler: gocron.NewScheduler(time.UTC),
	}
	handler.cronExecutorMap[cronFHPAKey][ruleName2] = RuleCron{
		Scheduler: gocron.NewScheduler(time.UTC),
	}

	_, err := handler.cronExecutorMap[cronFHPAKey][ruleName1].Scheduler.Every(1).Minute().Do(func() {})
	assert.NoError(t, err)

	_, err = handler.cronExecutorMap[cronFHPAKey][ruleName2].Scheduler.Every(1).Minute().Do(func() {})
	assert.NoError(t, err)

	handler.cronExecutorMap[cronFHPAKey][ruleName1].Scheduler.StartAsync()
	handler.cronExecutorMap[cronFHPAKey][ruleName2].Scheduler.StartAsync()

	assert.True(t, handler.cronExecutorMap[cronFHPAKey][ruleName1].Scheduler.IsRunning())
	assert.True(t, handler.cronExecutorMap[cronFHPAKey][ruleName2].Scheduler.IsRunning())

	handler.StopCronFHPAExecutor(cronFHPAKey)

	assert.NotContains(t, handler.cronExecutorMap, cronFHPAKey)
}

func TestCreateCronJobForExecutor(t *testing.T) {
	handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cronhpa",
			Namespace: "default",
		},
	}

	tests := []struct {
		name     string
		rule     autoscalingv1alpha1.CronFederatedHPARule
		timeZone *string
		wantErr  bool
	}{
		{
			name: "Valid rule without time zone",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				Name:     "test-rule",
				Schedule: "*/5 * * * *",
			},
			timeZone: nil,
			wantErr:  false,
		},
		{
			name: "Valid rule with valid time zone",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				Name:     "test-rule-tz",
				Schedule: "*/5 * * * *",
			},
			timeZone: ptr.To[string]("America/New_York"),
			wantErr:  false,
		},
		{
			name: "Valid rule with invalid time zone",
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				Name:     "test-rule-invalid-tz",
				Schedule: "*/5 * * * *",
			},
			timeZone: ptr.To[string]("Invalid/TimeZone"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cronFHPAKey := helper.GetCronFederatedHPAKey(cronFHPA)
			handler.cronExecutorMap = make(map[string]map[string]RuleCron)
			handler.cronExecutorMap[cronFHPAKey] = make(map[string]RuleCron)

			tt.rule.TimeZone = tt.timeZone
			err := handler.CreateCronJobForExecutor(cronFHPA, tt.rule)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, handler.cronExecutorMap[cronFHPAKey], tt.rule.Name)
				ruleCron := handler.cronExecutorMap[cronFHPAKey][tt.rule.Name]
				assert.NotNil(t, ruleCron.Scheduler)
				assert.True(t, ruleCron.Scheduler.IsRunning())
			}

			// Clean up
			if !tt.wantErr {
				handler.cronExecutorMap[cronFHPAKey][tt.rule.Name].Scheduler.Stop()
			}
		})
	}
}

func TestGetRuleNextExecuteTime(t *testing.T) {
	handler := NewCronHandler(fake.NewClientBuilder().Build(), record.NewFakeRecorder(100))

	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cronhpa",
			Namespace: "default",
		},
	}

	rule := autoscalingv1alpha1.CronFederatedHPARule{
		Name:     "test-rule",
		Schedule: "*/5 * * * *",
	}

	cronFHPAKey := helper.GetCronFederatedHPAKey(cronFHPA)
	handler.cronExecutorMap = make(map[string]map[string]RuleCron)
	handler.cronExecutorMap[cronFHPAKey] = make(map[string]RuleCron)

	err := handler.CreateCronJobForExecutor(cronFHPA, rule)
	assert.NoError(t, err)

	nextTime, err := handler.GetRuleNextExecuteTime(cronFHPA, rule.Name)
	assert.NoError(t, err)
	assert.False(t, nextTime.IsZero())
	assert.True(t, nextTime.After(time.Now()))

	_, err = handler.GetRuleNextExecuteTime(cronFHPA, "non-existent-rule")
	assert.Error(t, err)

	handler.cronExecutorMap[cronFHPAKey][rule.Name].Scheduler.Stop()
}
