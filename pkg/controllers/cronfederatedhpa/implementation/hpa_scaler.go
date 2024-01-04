package implementation

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

type HPAScaler struct {
	Client client.Client
}

func (h *HPAScaler) ScaleMinMaxThreshold(cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error {
	hpaName := types.NamespacedName{
		Namespace: cronFHPA.Namespace,
		Name:      cronFHPA.Spec.ScaleTargetRef.Name,
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := h.Client.Get(context.TODO(), hpaName, hpa)
	if err != nil {
		return err
	}

	update := false
	if rule.TargetMaxReplicas != nil && hpa.Spec.MaxReplicas != *rule.TargetMaxReplicas {
		hpa.Spec.MaxReplicas = *rule.TargetMaxReplicas
		update = true
	}
	if rule.TargetMinReplicas != nil && *hpa.Spec.MinReplicas != *rule.TargetMinReplicas {
		*hpa.Spec.MinReplicas = *rule.TargetMinReplicas
		update = true
	}

	if update {
		err := h.Client.Update(context.TODO(), hpa)
		if err != nil {
			klog.Errorf("CronFederatedHPA(%s/%s) updates HPA(%s/%s) failed: %v",
				cronFHPA.Namespace, cronFHPA.Name, hpa.Namespace, hpa.Name, err)
			return err
		}
		klog.V(4).Infof("CronFederatedHPA(%s/%s) scales HPA(%s/%s) successfully",
			cronFHPA.Namespace, cronFHPA.Name, hpa.Namespace, hpa.Name)
		return nil
	}

	klog.V(4).Infof("CronFederatedHPA(%s/%s) find nothing updated for HPA(%s/%s), skip it",
		cronFHPA.Namespace, cronFHPA.Name, hpa.Namespace, hpa.Name)
	return nil
}
