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

package helper

import (
	"k8s.io/apimachinery/pkg/types"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

func IsCronFederatedHPARuleSuspend(rule autoscalingv1alpha1.CronFederatedHPARule) bool {
	if rule.Suspend == nil {
		return false
	}
	return *rule.Suspend
}

func GetCronFederatedHPASuccessHistoryLimits(rule autoscalingv1alpha1.CronFederatedHPARule) int {
	if rule.SuccessfulHistoryLimit == nil {
		return 3
	}
	return int(*rule.SuccessfulHistoryLimit)
}

func GetCronFederatedHPAFailedHistoryLimits(rule autoscalingv1alpha1.CronFederatedHPARule) int {
	if rule.FailedHistoryLimit == nil {
		return 3
	}
	return int(*rule.FailedHistoryLimit)
}

func GetCronFederatedHPAKey(cronFHPA *autoscalingv1alpha1.CronFederatedHPA) string {
	namespacedName := types.NamespacedName{Namespace: cronFHPA.Namespace, Name: cronFHPA.Name}
	return namespacedName.String()
}
