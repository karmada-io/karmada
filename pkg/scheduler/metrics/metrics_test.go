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

package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestBindingSchedule(t *testing.T) {
	tests := []struct {
		name         string
		scheduleType string
		duration     float64
		err          error
	}{
		{
			name:         "Successful schedule",
			scheduleType: "test",
			duration:     1.5,
			err:          nil,
		},
		{
			name:         "Failed schedule",
			scheduleType: "test",
			duration:     0.5,
			err:          errors.New("schedule failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// We can't easily test the metric values directly, so we'll just ensure the function doesn't panic
			BindingSchedule(tt.scheduleType, tt.duration, tt.err)
		})
	}
}

func TestScheduleStep(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		duration time.Duration
	}{
		{
			name:     "Filter step",
			action:   ScheduleStepFilter,
			duration: 100 * time.Millisecond,
		},
		{
			name:     "Score step",
			action:   ScheduleStepScore,
			duration: 200 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			startTime := time.Now().Add(-tt.duration)
			// Ensure the function doesn't panic
			ScheduleStep(tt.action, startTime)
		})
	}
}

func TestCountSchedulerBindings(t *testing.T) {
	tests := []struct {
		name  string
		event string
	}{
		{
			name:  "Binding add event",
			event: BindingAdd,
		},
		{
			name:  "Binding update event",
			event: BindingUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// Ensure the function doesn't panic
			CountSchedulerBindings(tt.event)
		})
	}
}
