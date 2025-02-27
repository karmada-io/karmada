/*
Copyright 2025 The Karmada Authors.

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

package helper

import (
	"context"

	schedulingv1 "k8s.io/api/scheduling/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPriorityClassByName gets the k8s priority class by name.
//
// It returns the priorityClass and an error.
func GetPriorityClassByName(ctx context.Context, c client.Client, name string) (*schedulingv1.PriorityClass, error) {
	priorityClass := &schedulingv1.PriorityClass{}

	if err := c.Get(ctx, client.ObjectKey{Name: name}, priorityClass); err != nil {
		return nil, err
	}

	return priorityClass, nil
}
