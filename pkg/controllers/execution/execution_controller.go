package execution

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	propagationstrategy "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "execution-controller"
)

// Controller is to sync PropagationWork.
type Controller struct {
	client.Client                      // used to operate PropagationWork resources.
	KubeClientSet kubernetes.Interface // used to get kubernetes resources.
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
	ObjectWatcher objectwatcher.ObjectWatcher
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationWork %s", req.NamespacedName.String())

	work := &propagationstrategy.PropagationWork{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		applied := c.isResourceApplied(&work.Status)
		if applied {
			err := c.tryDeleteWorkload(work)
			if err != nil {
				klog.Errorf("Failed to delete propagationWork %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
				return controllerruntime.Result{Requeue: true}, err
			}
		}
		return c.removeFinalizer(work)
	}

	return c.syncWork(work)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&propagationstrategy.PropagationWork{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(c)
}

func (c *Controller) syncWork(propagationWork *propagationstrategy.PropagationWork) (controllerruntime.Result, error) {
	err := c.dispatchPropagationWork(propagationWork)
	if err != nil {
		klog.Errorf("Failed to dispatch propagationWork %q, namespace is %v, err is %v", propagationWork.Name, propagationWork.Namespace, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

// isResourceApplied checking weather resource has been dispatched to member cluster or not
func (c *Controller) isResourceApplied(propagationWorkStatus *propagationstrategy.PropagationWorkStatus) bool {
	for _, condition := range propagationWorkStatus.Conditions {
		if condition.Type == "Applied" {
			if condition.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// tryDeleteWorkload tries to delete resource in the given member cluster.
// Abort deleting when the member cluster is unready, otherwise we can't unjoin the member cluster when the member cluster is unready
func (c *Controller) tryDeleteWorkload(propagationWork *propagationstrategy.PropagationWork) error {
	clusterName, err := names.GetClusterName(propagationWork.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for propagationWork %s/%s", propagationWork.Namespace, propagationWork.Name)
		return err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to get the given member cluster %s", clusterName)
		return err
	}

	// Do not clean up resource in the given member cluster if the status of the given member cluster is unready
	if !util.IsClusterReady(&cluster.Status) {
		klog.Infof("Do not clean up resource in the given member cluster if the status of the given member cluster %s is unready", cluster.Name)
		return nil
	}

	for _, manifest := range propagationWork.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("failed to unmarshal workload, error is: %v", err)
			return err
		}

		err = c.ObjectWatcher.Delete(clusterName, workload)
		if err != nil {
			klog.Errorf("Failed to delete resource in the given member cluster %v, err is %v", cluster.Name, err)
			return err
		}
	}

	return nil
}

func (c *Controller) dispatchPropagationWork(propagationWork *propagationstrategy.PropagationWork) error {
	clusterName, err := names.GetClusterName(propagationWork.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for propagationWork %s/%s", propagationWork.Namespace, propagationWork.Name)
		return err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to the get given member cluster %s", clusterName)
		return err
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("The status of the given member cluster %s is unready", cluster.Name)
		return fmt.Errorf("cluster %s is not ready, requeuing operation until cluster state is ready", cluster.Name)
	}

	err = c.syncToClusters(cluster, propagationWork)
	if err != nil {
		klog.Errorf("Failed to dispatch propagationWork %v, namespace is %v, err is %v", propagationWork.Name, propagationWork.Namespace, err)
		return err
	}

	return nil
}

// syncToClusters ensures that the state of the given object is synchronized to member clusters.
func (c *Controller) syncToClusters(cluster *v1alpha1.Cluster, propagationWork *propagationstrategy.PropagationWork) error {
	clusterDynamicClient, err := util.NewClusterDynamicClientSet(cluster, c.KubeClientSet)
	if err != nil {
		return err
	}

	for _, manifest := range propagationWork.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("failed to unmarshal workload, error is: %v", err)
			return err
		}

		util.MergeLabel(workload, util.OwnerLabel, names.GenerateOwnerLabelValue(propagationWork.GetNamespace(), propagationWork.GetName()))

		applied := c.isResourceApplied(&propagationWork.Status)
		if applied {
			// todo: get clusterObj from cache
			dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
			if err != nil {
				klog.Errorf("Failed to get resource(%s/%s) as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
				return err
			}

			clusterObj, err := clusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Get(context.TODO(), workload.GetName(), v1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get resource %v from member cluster, err is %v ", workload.GetName(), err)
				return err
			}

			err = c.ObjectWatcher.Update(cluster.Name, clusterObj, workload)
			if err != nil {
				klog.Errorf("Failed to update resource in the given member cluster %s, err is %v", cluster.Name, err)
				return err
			}
		} else {
			err = c.ObjectWatcher.Create(cluster.Name, workload)
			if err != nil {
				klog.Errorf("Failed to create resource in the given member cluster %s, err is %v", cluster.Name, err)
				return err
			}

			err := c.updateAppliedCondition(propagationWork)
			if err != nil {
				klog.Errorf("Failed to update applied status for given propagationWork %v, namespace is %v, err is %v", propagationWork.Name, propagationWork.Namespace, err)
				return err
			}
		}
	}
	return nil
}

// removeFinalizer remove finalizer from the given propagationWork
func (c *Controller) removeFinalizer(propagationWork *propagationstrategy.PropagationWork) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(propagationWork, util.ExecutionControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(propagationWork, util.ExecutionControllerFinalizer)
	err := c.Client.Update(context.TODO(), propagationWork)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// updateAppliedCondition update the Applied condition for the given PropagationWork
func (c *Controller) updateAppliedCondition(propagationWork *propagationstrategy.PropagationWork) error {
	currentTime := v1.Now()
	propagationWorkApplied := "Applied"
	appliedSuccess := "AppliedSuccess"
	resourceApplied := "Success sync resource in member cluster"
	newPropagationWorkAppliedCondition := v1.Condition{
		Type:               propagationWorkApplied,
		Status:             v1.ConditionTrue,
		Reason:             appliedSuccess,
		Message:            resourceApplied,
		LastTransitionTime: currentTime,
	}
	propagationWork.Status.Conditions = append(propagationWork.Status.Conditions, newPropagationWorkAppliedCondition)
	err := c.Client.Status().Update(context.TODO(), propagationWork)
	return err
}
