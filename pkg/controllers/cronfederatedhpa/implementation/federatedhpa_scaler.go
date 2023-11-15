package implementation

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
)

type FederatedHPAScaler struct {
	Client client.Client
}

func (f *FederatedHPAScaler) ScaleMinMaxThreshold(cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error {
	fhpaName := types.NamespacedName{
		Namespace: cronFHPA.Namespace,
		Name:      cronFHPA.Spec.ScaleTargetRef.Name,
	}

	fhpa := &autoscalingv1alpha1.FederatedHPA{}
	err := f.Client.Get(context.TODO(), fhpaName, fhpa)
	if err != nil {
		return err
	}

	update := false
	if rule.TargetMaxReplicas != nil && fhpa.Spec.MaxReplicas != *rule.TargetMaxReplicas {
		fhpa.Spec.MaxReplicas = *rule.TargetMaxReplicas
		update = true
	}
	if rule.TargetMinReplicas != nil && *fhpa.Spec.MinReplicas != *rule.TargetMinReplicas {
		*fhpa.Spec.MinReplicas = *rule.TargetMinReplicas
		update = true
	}

	if update {
		err := f.Client.Update(context.TODO(), fhpa)
		if err != nil {
			klog.Errorf("CronFederatedHPA(%s/%s) updates FederatedHPA(%s/%s) failed: %v",
				cronFHPA.Namespace, cronFHPA.Name, fhpa.Namespace, fhpa.Name, err)
			return err
		}
		klog.V(4).Infof("CronFederatedHPA(%s/%s) scales FederatedHPA(%s/%s) successfully",
			cronFHPA.Namespace, cronFHPA.Name, fhpa.Namespace, fhpa.Name)
		return nil
	}

	klog.V(4).Infof("CronFederatedHPA(%s/%s) find nothing updated for FederatedHPA(%s/%s), skip it",
		cronFHPA.Namespace, cronFHPA.Name, fhpa.Namespace, fhpa.Name)
	return nil
}
