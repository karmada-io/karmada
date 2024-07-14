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

package v1alpha1

import (
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Name returns the image name.
func (image *Image) Name() string {
	return fmt.Sprintf("%s:%s", image.ImageRepository, image.ImageTag)
}

// KarmadaInProgressing sets the Karmada condition to Progressing.
func KarmadaInProgressing(karmada *Karmada, conditionType ConditionType, message string) {
	karmada.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    string(conditionType),
		Status:  metav1.ConditionFalse,
		Reason:  "Progressing",
		Message: message,
	}

	apimeta.SetStatusCondition(&karmada.Status.Conditions, newCondition)
}

// KarmadaCompleted sets the Karmada condition to Completed.
func KarmadaCompleted(karmada *Karmada, conditionType ConditionType, message string) {
	karmada.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    string(conditionType),
		Status:  metav1.ConditionTrue,
		Reason:  "Completed",
		Message: message,
	}

	apimeta.SetStatusCondition(&karmada.Status.Conditions, newCondition)
}

// KarmadaFailed sets the Karmada condition to Failed.
func KarmadaFailed(karmada *Karmada, conditionType ConditionType, message string) {
	karmada.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    string(conditionType),
		Status:  metav1.ConditionFalse,
		Reason:  "Failed",
		Message: message,
	}

	apimeta.SetStatusCondition(&karmada.Status.Conditions, newCondition)
}
