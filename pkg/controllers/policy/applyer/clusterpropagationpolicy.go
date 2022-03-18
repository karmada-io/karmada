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

// ClusterPropagationPolicyApplyer apply ClusterPropagationPolicy to a specific object
type ClusterPropagationPolicyApplyer struct {
	client.Client
	resourceInterpreter resourceinterpreter.ResourceInterpreter
}

func NewClusterPropagationPolicyApplyer(c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter) Applyer {
	return &PropagationPolicyApplyer{
		Client:              c,
		resourceInterpreter: resourceInterpreter,
	}
}

func (p *ClusterPropagationPolicyApplyer) Apply(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, unstructuredPolicy *unstructured.Unstructured) error {
	policy, err := helper.ConvertToPropagationPolicy(unstructuredPolicy)
	if err != nil {
		return err
	}
	if err := p.claimPolicy(object, policy.Name, policy.Namespace); err != nil {
		klog.Errorf("Failed to claim cluster policy(%s) for object: %s", policy.Name, object)
		return err
	}

	policyLabels := map[string]string{
		policyv1alpha1.ClusterPropagationPolicyLabel: policy.GetName(),
	}
	builder := newBuilder(object, objectKey, policyLabels, policy.Spec.PropagateDeps, p.resourceInterpreter)
	// Build `ResourceBinding` or `ClusterResourceBinding` according to the resource template's scope.
	// For namespace-scoped resources, which namespace is not empty, building `ResourceBinding`.
	// For cluster-scoped resources, which namespace is empty, building `ClusterResourceBinding`.
	if object.GetNamespace() != "" {
		binding, err := builder.buildResourceBinding()
		if err != nil {
			klog.Errorf("Failed to build resourceBinding for object: %s. error: %v", objectKey, err)
			return err
		}
		bindingCopy := binding.DeepCopy()
		var operationResult controllerutil.OperationResult
		// TODO: remove duplicate code
		err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			operationResult, err = controllerutil.CreateOrUpdate(context.TODO(), p.Client, bindingCopy, func() error {
				// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
				bindingCopy.Labels = binding.Labels
				bindingCopy.OwnerReferences = binding.OwnerReferences
				bindingCopy.Finalizers = binding.Finalizers
				bindingCopy.Spec.Resource = binding.Spec.Resource
				bindingCopy.Spec.ReplicaRequirements = binding.Spec.ReplicaRequirements
				bindingCopy.Spec.Replicas = binding.Spec.Replicas
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			klog.Errorf("Failed to apply cluster policy(%s) for object: %s. error: %v", policy.Name, objectKey, err)
			return err
		}

		if operationResult == controllerutil.OperationResultCreated {
			klog.Infof("Create ResourceBinding(%s) successfully.", binding.GetName())
		} else if operationResult == controllerutil.OperationResultUpdated {
			klog.Infof("Update ResourceBinding(%s) successfully.", binding.GetName())
		} else {
			klog.V(2).Infof("ResourceBinding(%s) is up to date.", binding.GetName())
		}
	} else {
		binding, err := builder.buildClusterResourceBinding()
		if err != nil {
			klog.Errorf("Failed to build clusterResourceBinding for object: %s. error: %v", objectKey, err)
			return err
		}
		bindingCopy := binding.DeepCopy()
		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), p.Client, bindingCopy, func() error {
			// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
			bindingCopy.Labels = binding.Labels
			bindingCopy.OwnerReferences = binding.OwnerReferences
			bindingCopy.Finalizers = binding.Finalizers
			bindingCopy.Spec.Resource = binding.Spec.Resource
			bindingCopy.Spec.ReplicaRequirements = binding.Spec.ReplicaRequirements
			bindingCopy.Spec.Replicas = binding.Spec.Replicas
			return nil
		})
		if err != nil {
			klog.Errorf("Failed to apply cluster policy(%s) for object: %s. error: %v", policy.Name, objectKey, err)
			return err
		}

		if operationResult == controllerutil.OperationResultCreated {
			klog.Infof("Create ClusterResourceBinding(%s) successfully.", binding.GetName())
		} else if operationResult == controllerutil.OperationResultUpdated {
			klog.Infof("Update ClusterResourceBinding(%s) successfully.", binding.GetName())
		} else {
			klog.V(2).Infof("ClusterResourceBinding(%s) is up to date.", binding.GetName())
		}
	}

	return nil
}

func (p *ClusterPropagationPolicyApplyer) claimPolicy(object *unstructured.Unstructured, policyNamespace string, policyName string) error {
	claimedName := util.GetLabelValue(object.GetLabels(), policyv1alpha1.ClusterPropagationPolicyLabel)

	// object has been claimed, don't need to claim again
	if claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, policyv1alpha1.ClusterPropagationPolicyLabel, policyName)
	return p.Client.Update(context.TODO(), object)
}
