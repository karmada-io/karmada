package crdexplorer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/crdexplorer/customizedexplorer"
	"github.com/karmada-io/karmada/pkg/crdexplorer/customizedexplorer/webhook"
	"github.com/karmada-io/karmada/pkg/crdexplorer/defaultexplorer"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

// CustomResourceExplorer manages both default and customized webhooks to explore custom resource structure.
type CustomResourceExplorer interface {
	// Start starts running the component and will never stop running until the context is closed or an error occurs.
	Start(ctx context.Context) (err error)

	// HookEnabled tells if any hook exist for specific resource type and operation type.
	HookEnabled(object *unstructured.Unstructured, operationType configv1alpha1.OperationType) bool

	// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
	GetReplicas(object *unstructured.Unstructured) (replica int32, replicaRequires *workv1alpha2.ReplicaRequirements, err error)

	// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
	Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

	// other common method
}

// NewCustomResourceExplorer builds a new CustomResourceExplorer object.
func NewCustomResourceExplorer(kubeconfig string, informer informermanager.SingleClusterInformerManager) CustomResourceExplorer {
	return &customResourceExplorerImpl{
		kubeconfig: kubeconfig,
		informer:   informer,
	}
}

type customResourceExplorerImpl struct {
	kubeconfig string
	informer   informermanager.SingleClusterInformerManager

	customizedExplorer *customizedexplorer.CustomizedExplorer
	defaultExplorer    *defaultexplorer.DefaultExplorer
}

// Start starts running the component and will never stop running until the context is closed or an error occurs.
func (i *customResourceExplorerImpl) Start(ctx context.Context) (err error) {
	klog.Infof("Starting custom resource explorer.")

	i.customizedExplorer, err = customizedexplorer.NewCustomizedExplorer(i.kubeconfig, i.informer)
	if err != nil {
		return
	}

	i.defaultExplorer = defaultexplorer.NewDefaultExplorer()

	i.informer.Start()
	<-ctx.Done()
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// HookEnabled tells if any hook exist for specific resource type and operation type.
func (i *customResourceExplorerImpl) HookEnabled(object *unstructured.Unstructured, operationType configv1alpha1.OperationType) bool {
	attributes := &webhook.RequestAttributes{
		Operation: operationType,
		Object:    object,
	}
	return i.customizedExplorer.HookEnabled(attributes) || i.defaultExplorer.HookEnabled(object.GroupVersionKind(), operationType)
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (i *customResourceExplorerImpl) GetReplicas(object *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	klog.V(4).Infof("Begin to get replicas for request object: %v %s/%s.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	var hookEnabled bool
	replica, requires, hookEnabled, err = i.customizedExplorer.GetReplicas(context.TODO(), &webhook.RequestAttributes{
		Operation: configv1alpha1.ExploreReplica,
		Object:    object,
	})
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}

	replica, requires, err = i.defaultExplorer.GetReplicas(object)
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (i *customResourceExplorerImpl) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	// TODO(RainbowMango): consult to the dynamic webhooks first.

	return i.defaultExplorer.Retain(desired, observed)
}
