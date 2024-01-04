package implementation

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type ReplicasScaler struct {
	Client client.Client
}

func (r *ReplicasScaler) ScaleReplicas(cronFHPA *autoscalingv1alpha1.CronFederatedHPA, rule autoscalingv1alpha1.CronFederatedHPARule) error {
	ctx := context.Background()

	scaleClient := r.Client.SubResource("scale")
	targetGV, err := schema.ParseGroupVersion(cronFHPA.Spec.ScaleTargetRef.APIVersion)
	if err != nil {
		klog.Errorf("CronFederatedHPA(%s/%s) parses GroupVersion(%s) failed: %v",
			cronFHPA.Namespace, cronFHPA.Name, cronFHPA.Spec.ScaleTargetRef.APIVersion, err)
		return err
	}
	targetGVK := schema.GroupVersionKind{
		Group:   targetGV.Group,
		Kind:    cronFHPA.Spec.ScaleTargetRef.Kind,
		Version: targetGV.Version,
	}
	targetResource := &unstructured.Unstructured{}
	targetResource.SetGroupVersionKind(targetGVK)
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: cronFHPA.Namespace, Name: cronFHPA.Spec.ScaleTargetRef.Name}, targetResource)
	if err != nil {
		klog.Errorf("Get Resource(%s/%s) failed: %v", cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, err)
		return err
	}

	scaleObj := &unstructured.Unstructured{}
	err = scaleClient.Get(ctx, targetResource, scaleObj)
	if err != nil {
		klog.Errorf("Get Scale for resource(%s/%s) failed: %v", cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, err)
		return err
	}

	scale := &autoscalingv1.Scale{}
	err = helper.ConvertToTypedObject(scaleObj, scale)
	if err != nil {
		klog.Errorf("Convert Scale failed: %v", err)
		return err
	}

	if scale.Spec.Replicas != *rule.TargetReplicas {
		if err := helper.ApplyReplica(scaleObj, int64(*rule.TargetReplicas), util.ReplicasField); err != nil {
			klog.Errorf("CronFederatedHPA(%s/%s) applies Replicas for %s/%s failed: %v",
				cronFHPA.Namespace, cronFHPA.Name, cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, err)
			return err
		}
		err := scaleClient.Update(ctx, targetResource, client.WithSubResourceBody(scaleObj))
		if err != nil {
			klog.Errorf("CronFederatedHPA(%s/%s) updates scale resource failed: %v", cronFHPA.Namespace, cronFHPA.Name, err)
			return err
		}
		klog.V(4).Infof("CronFederatedHPA(%s/%s) scales resource(%s/%s) successfully",
			cronFHPA.Namespace, cronFHPA.Name, cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name)
		return nil
	}
	return nil
}
