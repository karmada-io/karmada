package namespace

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName              = "namespace-sync-controller"
	namespaceKarmadaSystem      = "karmada-system"
	namespaceKarmadaCluster     = "karmada-cluster"
	namespaceDefault            = "default"
	karmadaExecutionSpacePrefix = "karmada-es-"
	kubeSystemNamespacePrefix   = "kube-"
)

// Controller is to sync Work.
type Controller struct {
	client.Client // used to operate Work resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Namespaces sync controller reconciling %s", req.NamespacedName.String())
	if !c.namespaceShouldBeSynced(req.Name) {
		return controllerruntime.Result{}, nil
	}

	namespace := &v1.Namespace{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, namespace); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !namespace.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	clusterList := &v1alpha1.ClusterList{}
	if err := c.Client.List(context.TODO(), clusterList); err != nil {
		klog.Errorf("Failed to list clusters, error: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err := c.buildWorks(namespace, clusterList.Items)
	if err != nil {
		klog.Errorf("Failed to build work for namespace %s. Error: %v.", namespace.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *Controller) namespaceShouldBeSynced(namespace string) bool {
	if namespace == namespaceKarmadaCluster || namespace == namespaceKarmadaSystem || namespace == namespaceDefault ||
		strings.HasPrefix(namespace, karmadaExecutionSpacePrefix) || strings.HasPrefix(namespace, kubeSystemNamespacePrefix) {
		return false
	}
	return true
}

func (c *Controller) buildWorks(namespace *v1.Namespace, clusters []v1alpha1.Cluster) error {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(namespace)
	if err != nil {
		klog.Errorf("Failed to transform namespace %s. Error: %v", namespace.GetName(), err)
		return nil
	}
	namespaceObj := &unstructured.Unstructured{Object: uncastObj}
	util.RemoveIrrelevantField(namespaceObj)

	for _, cluster := range clusters {
		workNamespace, err := names.GenerateExecutionSpaceName(cluster.Name)
		if err != nil {
			klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", cluster.Name, err)
			return err
		}

		workName := names.GenerateClusterResourceBindingName(namespaceObj.GetKind(), namespaceObj.GetName())
		objectMeta := metav1.ObjectMeta{
			Name:       workName,
			Namespace:  workNamespace,
			Finalizers: []string{util.ExecutionControllerFinalizer},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(namespace, namespace.GroupVersionKind()),
			},
		}

		util.MergeLabel(namespaceObj, util.WorkNamespaceLabel, workNamespace)
		util.MergeLabel(namespaceObj, util.WorkNameLabel, workName)

		namespaceJSON, err := namespaceObj.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal namespace %s. Error: %v", namespaceObj.GetName(), err)
			return err
		}

		err = util.CreateOrUpdateWork(c.Client, objectMeta, namespaceJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	namespaceFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			var requests []reconcile.Request
			namespaceList := &v1.NamespaceList{}
			if err := c.Client.List(context.TODO(), namespaceList); err != nil {
				klog.Errorf("Failed to list namespace, error: %v", err)
				return nil
			}

			for _, namespace := range namespaceList.Items {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name: namespace.Name,
				}})
			}
			return requests
		})

	predicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Meta == nil {
				klog.Errorf("CreateEvent received with no metadata, event: %v", e)
				return false
			}
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&v1.Namespace{}).Watches(&source.Kind{Type: &v1alpha1.Cluster{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: namespaceFn},
		predicate).Complete(c)
}
