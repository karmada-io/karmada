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
