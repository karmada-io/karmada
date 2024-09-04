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
	"context"
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

func TestNewCronFederatedHPAJob(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	eventRecorder := record.NewFakeRecorder(100)
	scheduler := gocron.NewScheduler(time.UTC)
	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron-fhpa",
			Namespace: "default",
		},
	}
	rule := autoscalingv1alpha1.CronFederatedHPARule{
		Name: "test-rule",
	}

	job := NewCronFederatedHPAJob(client, eventRecorder, scheduler, cronFHPA, rule)

	assert.NotNil(t, job)
	assert.Equal(t, client, job.client)
	assert.Equal(t, eventRecorder, job.eventRecorder)
	assert.Equal(t, scheduler, job.scheduler)
	assert.Equal(t, cronFHPA.Name, job.namespaceName.Name)
	assert.Equal(t, cronFHPA.Namespace, job.namespaceName.Namespace)
	assert.Equal(t, rule, job.rule)
}

func TestScaleFHPA(t *testing.T) {
	tests := []struct {
		name           string
		cronFHPA       *autoscalingv1alpha1.CronFederatedHPA
		existingFHPA   *autoscalingv1alpha1.FederatedHPA
		rule           autoscalingv1alpha1.CronFederatedHPARule
		expectedUpdate bool
		expectedErr    bool
	}{
		{
			name: "Update MaxReplicas",
			cronFHPA: &autoscalingv1alpha1.CronFederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cron-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.CronFederatedHPASpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Name: "test-fhpa",
					},
				},
			},
			existingFHPA: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					MaxReplicas: 5,
				},
			},
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				TargetMaxReplicas: intPtr(10),
			},
			expectedUpdate: true,
			expectedErr:    false,
		},
		{
			name: "Update MinReplicas",
			cronFHPA: &autoscalingv1alpha1.CronFederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cron-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.CronFederatedHPASpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Name: "test-fhpa",
					},
				},
			},
			existingFHPA: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					MinReplicas: intPtr(2),
				},
			},
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				TargetMinReplicas: intPtr(3),
			},
			expectedUpdate: true,
			expectedErr:    false,
		},
		{
			name: "No Updates Needed",
			cronFHPA: &autoscalingv1alpha1.CronFederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cron-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.CronFederatedHPASpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Name: "test-fhpa",
					},
				},
			},
			existingFHPA: &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-fhpa",
					Namespace: "default",
				},
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 5,
				},
			},
			rule: autoscalingv1alpha1.CronFederatedHPARule{
				TargetMinReplicas: intPtr(2),
				TargetMaxReplicas: intPtr(5),
			},
			expectedUpdate: false,
			expectedErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = autoscalingv1alpha1.Install(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.existingFHPA).Build()

			job := &ScalingJob{
				client: client,
				rule:   tt.rule,
			}

			err := job.ScaleFHPA(tt.cronFHPA)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedUpdate {
				updatedFHPA := &autoscalingv1alpha1.FederatedHPA{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: tt.existingFHPA.Name, Namespace: tt.existingFHPA.Namespace}, updatedFHPA)
				assert.NoError(t, err)
				if tt.rule.TargetMaxReplicas != nil {
					assert.Equal(t, *tt.rule.TargetMaxReplicas, updatedFHPA.Spec.MaxReplicas)
				}
				if tt.rule.TargetMinReplicas != nil {
					assert.Equal(t, *tt.rule.TargetMinReplicas, *updatedFHPA.Spec.MinReplicas)
				}
			}
		})
	}
}

func intPtr(i int32) *int32 {
	return &i
}

func TestFindExecutionHistory(t *testing.T) {
	tests := []struct {
		name          string
		histories     []autoscalingv1alpha1.ExecutionHistory
		ruleName      string
		expectedIndex int
	}{
		{
			name: "Found",
			histories: []autoscalingv1alpha1.ExecutionHistory{
				{RuleName: "rule1"},
				{RuleName: "rule2"},
				{RuleName: "rule3"},
			},
			ruleName:      "rule2",
			expectedIndex: 1,
		},
		{
			name: "Not Found",
			histories: []autoscalingv1alpha1.ExecutionHistory{
				{RuleName: "rule1"},
				{RuleName: "rule2"},
			},
			ruleName:      "rule3",
			expectedIndex: -1,
		},
		{
			name:          "Empty History",
			histories:     []autoscalingv1alpha1.ExecutionHistory{},
			ruleName:      "rule1",
			expectedIndex: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &ScalingJob{
				rule: autoscalingv1alpha1.CronFederatedHPARule{
					Name: tt.ruleName,
				},
			}
			result := job.findExecutionHistory(tt.histories)
			assert.Equal(t, tt.expectedIndex, result)
		})
	}
}
