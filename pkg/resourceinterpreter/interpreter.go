package resourceinterpreter

import (
	"context"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customizedinterpreter"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customizedinterpreter/webhook"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/defaultinterpreter"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

// ResourceInterpreter manages both default and customized webhooks to interpret custom resource structure.
type ResourceInterpreter interface {
	// Start starts running the component and will never stop running until the context is closed or an error occurs.
	Start(ctx context.Context) (err error)

	// HookEnabled tells if any hook exist for specific resource type and operation.
	HookEnabled(object *unstructured.Unstructured, operationType configv1alpha1.InterpreterOperation) bool

	// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
	GetReplicas(object *unstructured.Unstructured) (replica int32, replicaRequires *workv1alpha2.ReplicaRequirements, err error)

	// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
	Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

	// other common method
}

// NewResourceInterpreter builds a new ResourceInterpreter object.
func NewResourceInterpreter(kubeconfig string, informer informermanager.SingleClusterInformerManager) ResourceInterpreter {
	return &customResourceInterpreterImpl{
		kubeconfig: kubeconfig,
		informer:   informer,
	}
}

type customResourceInterpreterImpl struct {
	kubeconfig string
	informer   informermanager.SingleClusterInformerManager

	customizedInterpreter *customizedinterpreter.CustomizedInterpreter
	defaultInterpreter    *defaultinterpreter.DefaultInterpreter
}

// Start starts running the component and will never stop running until the context is closed or an error occurs.
func (i *customResourceInterpreterImpl) Start(ctx context.Context) (err error) {
	klog.Infof("Starting custom resource interpreter.")

	i.customizedInterpreter, err = customizedinterpreter.NewCustomizedInterpreter(i.kubeconfig, i.informer)
	if err != nil {
		return
	}

	i.defaultInterpreter = defaultinterpreter.NewDefaultInterpreter()

	i.informer.Start()
	<-ctx.Done()
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// HookEnabled tells if any hook exist for specific resource type and operation.
func (i *customResourceInterpreterImpl) HookEnabled(object *unstructured.Unstructured, operation configv1alpha1.InterpreterOperation) bool {
	attributes := &webhook.RequestAttributes{
		Operation: operation,
		Object:    object,
	}
	return i.customizedInterpreter.HookEnabled(attributes) || i.defaultInterpreter.HookEnabled(object.GroupVersionKind(), operation)
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (i *customResourceInterpreterImpl) GetReplicas(object *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	klog.V(4).Infof("Begin to get replicas for request object: %v %s/%s.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	var hookEnabled bool
	replica, requires, hookEnabled, err = i.customizedInterpreter.GetReplicas(context.TODO(), &webhook.RequestAttributes{
		Operation: configv1alpha1.InterpreterOperationInterpretReplica,
		Object:    object,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	replica, requires, err = i.defaultInterpreter.GetReplicas(object)
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (i *customResourceInterpreterImpl) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	klog.V(4).Infof("Begin to retain object: %v %s/%s.", desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName())

	var hookEnabled bool
	var patch []byte
	var patchType configv1alpha1.PatchType
	patch, patchType, hookEnabled, err = i.customizedInterpreter.Retain(context.TODO(), &webhook.RequestAttributes{
		Operation:   configv1alpha1.InterpreterOperationRetain,
		Object:      desired,
		ObservedObj: observed,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return applyPatch(desired, patch, patchType)
	}

	return i.defaultInterpreter.Retain(desired, observed)
}

// applyPatch uses patchType mode to patch object.
func applyPatch(object *unstructured.Unstructured, patch []byte, patchType configv1alpha1.PatchType) (*unstructured.Unstructured, error) {
	switch patchType {
	case configv1alpha1.PatchTypeJSONPatch:
		patchObj, err := jsonpatch.DecodePatch(patch)
		if err != nil {
			return nil, err
		}
		if len(patchObj) == 0 {
			return object, nil
		}

		objectJSONBytes, err := object.MarshalJSON()
		if err != nil {
			return nil, err
		}
		patchedObjectJSONBytes, err := patchObj.Apply(objectJSONBytes)
		if err != nil {
			return nil, err
		}

		err = object.UnmarshalJSON(patchedObjectJSONBytes)
		return object, err
	default:
		return nil, fmt.Errorf("return patch type %s is not support", patchType)
	}
}
