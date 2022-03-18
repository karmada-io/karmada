package applyer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

// PropagationPolicyApplyer apply PropagationPolicy to a specific object
type PropagationPolicyApplyer struct {
	client.Client
	resourceInterpreter resourceinterpreter.ResourceInterpreter
}

func NewPropagationPolicyApplyer(c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter) Applyer {
	return &PropagationPolicyApplyer{
		Client:              c,
		resourceInterpreter: resourceInterpreter,
	}
}

func (p *PropagationPolicyApplyer) Apply(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, unstructuredPolicy *unstructured.Unstructured) error {
	policy, err := helper.ConvertToPropagationPolicy(unstructuredPolicy)
	if err != nil {
		return err
	}
	klog.Infof("Applying policy(%s%s) for object: %s", policy.Namespace, policy.Name, objectKey)
	// defer func() {
	// 	if err != nil {
	// 		c.eventRecorder.Eventf(object, corev1.EventTypeWarning, workv1alpha2.EventReasonApplyPolicyFailed, "Apply policy(%s/%s) failed", policy.Namespace, policy.Name)
	// 	} else {
	// 		c.eventRecorder.Eventf(object, corev1.EventTypeNormal, workv1alpha2.EventReasonApplyPolicySucceed, "Apply policy(%s/%s) succeed", policy.Namespace, policy.Name)
	// 	}
	// }()

	if err := p.claimPolicy(object, policy.Namespace, policy.Name); err != nil {
		klog.Errorf("Failed to claim policy(%s) for object: %s", policy.Name, object)
		return err
	}

	policyLabels := map[string]string{
		policyv1alpha1.PropagationPolicyNamespaceLabel: policy.GetNamespace(),
		policyv1alpha1.PropagationPolicyNameLabel:      policy.GetName(),
	}
	builder := newBuilder(object, objectKey, policyLabels, policy.Spec.PropagateDeps, p.resourceInterpreter)
	binding, err := builder.buildResourceBinding()
	if err != nil {
		klog.Errorf("Failed to build resourceBinding for object: %s. error: %v", objectKey, err)
		return err
	}
	bindingCopy := binding.DeepCopy()
	var operationResult controllerutil.OperationResult
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(context.TODO(), p.Client, bindingCopy, func() error {
			// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
			bindingCopy.Labels = util.DedupeAndMergeLabels(bindingCopy.Labels, binding.Labels)
			bindingCopy.OwnerReferences = binding.OwnerReferences
			bindingCopy.Finalizers = binding.Finalizers
			bindingCopy.Spec.Resource = binding.Spec.Resource
			bindingCopy.Spec.ReplicaRequirements = binding.Spec.ReplicaRequirements
			bindingCopy.Spec.Replicas = binding.Spec.Replicas
			bindingCopy.Spec.PropagateDeps = binding.Spec.PropagateDeps
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to apply policy(%s) for object: %s. error: %v", policy.Name, objectKey, err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.Infof("Create ResourceBinding(%s/%s) successfully.", binding.GetNamespace(), binding.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.Infof("Update ResourceBinding(%s/%s) successfully.", binding.GetNamespace(), binding.GetName())
	} else {
		klog.V(2).Infof("ResourceBinding(%s/%s) is up to date.", binding.GetNamespace(), binding.GetName())
	}

	return nil
}

func (p *PropagationPolicyApplyer) claimPolicy(object *unstructured.Unstructured, policyNamespace string, policyName string) error {
	claimedNS := util.GetLabelValue(object.GetLabels(), policyv1alpha1.PropagationPolicyNamespaceLabel)
	claimedName := util.GetLabelValue(object.GetLabels(), policyv1alpha1.PropagationPolicyNameLabel)

	// object has been claimed, don't need to claim again
	if claimedNS == policyNamespace && claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, policyv1alpha1.PropagationPolicyNamespaceLabel, policyNamespace)
	util.MergeLabel(object, policyv1alpha1.PropagationPolicyNameLabel, policyName)

	return p.Client.Update(context.TODO(), object)
}
