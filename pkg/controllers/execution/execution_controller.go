package execution

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "execution-controller"
)

// Controller is to sync Work.
type Controller struct {
	client.Client                      // used to operate Work resources.
	KubeClientSet kubernetes.Interface // used to get kubernetes resources.
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
	ObjectWatcher objectwatcher.ObjectWatcher
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling Work %s", req.NamespacedName.String())

	work := &policyv1alpha1.Work{}
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
				klog.Errorf("Failed to delete work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
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
		For(&policyv1alpha1.Work{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(c)
}

func (c *Controller) syncWork(work *policyv1alpha1.Work) (controllerruntime.Result, error) {
	err := c.dispatchWork(work)
	if err != nil {
		klog.Errorf("Failed to dispatch work %q, namespace is %v, err is %v", work.Name, work.Namespace, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

// isResourceApplied checking weather resource has been dispatched to member cluster or not
func (c *Controller) isResourceApplied(workStatus *policyv1alpha1.WorkStatus) bool {
	for _, condition := range workStatus.Conditions {
		if condition.Type == policyv1alpha1.WorkApplied {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// tryDeleteWorkload tries to delete resource in the given member cluster.
// Abort deleting when the member cluster is unready, otherwise we can't unjoin the member cluster when the member cluster is unready
func (c *Controller) tryDeleteWorkload(work *policyv1alpha1.Work) error {
	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for work %s/%s", work.Namespace, work.Name)
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

	for _, manifest := range work.Spec.Workload.Manifests {
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

func (c *Controller) dispatchWork(work *policyv1alpha1.Work) error {
	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for work %s/%s", work.Namespace, work.Name)
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

	err = c.syncToClusters(cluster, work)
	if err != nil {
		klog.Errorf("Failed to dispatch work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
		return err
	}

	return nil
}

// syncToClusters ensures that the state of the given object is synchronized to member clusters.
func (c *Controller) syncToClusters(cluster *v1alpha1.Cluster, work *policyv1alpha1.Work) error {
	clusterDynamicClient, err := util.NewClusterDynamicClientSet(cluster, c.KubeClientSet)
	if err != nil {
		return err
	}

	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("failed to unmarshal workload, error is: %v", err)
			return err
		}

		util.MergeLabel(workload, util.OwnerLabel, names.GenerateOwnerLabelValue(work.GetNamespace(), work.GetName()))

		applied := c.isResourceApplied(&work.Status)
		if applied {
			// todo: get clusterObj from cache
			dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
			if err != nil {
				klog.Errorf("Failed to get resource(%s/%s) as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
				return err
			}

			clusterObj, err := clusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Get(context.TODO(), workload.GetName(), metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get resource %v from member cluster, err is %v ", workload.GetName(), err)
				return err
			}

			err = c.ObjectWatcher.Update(cluster.Name, workload, clusterObj)
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

			err := c.updateAppliedCondition(work)
			if err != nil {
				klog.Errorf("Failed to update applied status for given work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
				return err
			}
		}
	}
	return nil
}

// removeFinalizer remove finalizer from the given Work
func (c *Controller) removeFinalizer(work *policyv1alpha1.Work) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(work, util.ExecutionControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(work, util.ExecutionControllerFinalizer)
	err := c.Client.Update(context.TODO(), work)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// updateAppliedCondition update the Applied condition for the given Work
func (c *Controller) updateAppliedCondition(work *policyv1alpha1.Work) error {
	newWorkAppliedCondition := metav1.Condition{
		Type:               policyv1alpha1.WorkApplied,
		Status:             metav1.ConditionTrue,
		Reason:             "AppliedSuccessful",
		Message:            "Manifest has been successfully applied",
		LastTransitionTime: metav1.Now(),
	}
	work.Status.Conditions = append(work.Status.Conditions, newWorkAppliedCondition)
	err := c.Client.Status().Update(context.TODO(), work)
	return err
}
