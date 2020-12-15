package execution

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
	propagationstrategy "github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "execution-controller"
	finalizer      = "karmada.io/execution-controller"
)

// Controller is to sync PropagationWork.
type Controller struct {
	client.Client                            // used to operate PropagationWork resources.
	KubeClientSet kubernetes.Interface       // used to get kubernetes resources.
	KarmadaClient karmadaclientset.Interface // used to get MemberCluster resources.
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
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
	return controllerruntime.NewControllerManagedBy(mgr).For(&propagationstrategy.PropagationWork{}).Complete(c)
}

func (c *Controller) syncWork(propagationWork *propagationstrategy.PropagationWork) (controllerruntime.Result, error) {
	// ensure finalizer
	updated, err := c.ensureFinalizer(propagationWork)
	if err != nil {
		klog.Errorf("Failed to ensure finalizer for propagationWork %q, namespace is %v, err is %v", propagationWork.Name, propagationWork.Namespace, err)
		return controllerruntime.Result{Requeue: true}, err
	} else if updated {
		return controllerruntime.Result{}, nil
	}

	err = c.dispatchPropagationWork(propagationWork)
	if err != nil {
		klog.Errorf("Failed to dispatch propagationWork %q, namespace is %v, err is %v", propagationWork.Name, propagationWork.Namespace, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

// isMemberClusterReady checking readiness for the given member cluster
func (c *Controller) isMemberClusterReady(clusterStatus *v1alpha1.MemberClusterStatus) bool {
	for _, condition := range clusterStatus.Conditions {
		if condition.Type == "ClusterReady" {
			if condition.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
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
	memberClusterName, err := names.GetMemberClusterName(propagationWork.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for propagationWork %s/%s", propagationWork.Namespace, propagationWork.Name)
		return err
	}

	// TODO(RainbowMango): retrieve member cluster from the local cache instead of a real request to API server.
	memberCluster, err := c.KarmadaClient.MemberclusterV1alpha1().MemberClusters().Get(context.TODO(), memberClusterName, v1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get the given member cluster %s", memberClusterName)
		return err
	}

	// Do not clean up resource in the given member cluster if the status of the given member cluster is unready
	if !c.isMemberClusterReady(&memberCluster.Status) {
		klog.Infof("Do not clean up resource in the given member cluster if the status of the given member cluster %s is unready", memberCluster.Name)
		return nil
	}

	memberClusterDynamicClient, err := util.NewClusterDynamicClientSet(memberCluster, c.KubeClientSet)
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

		err = c.deleteResource(memberClusterDynamicClient, workload)
		if err != nil {
			klog.Errorf("Failed to delete resource in the given member cluster %v, err is %v", memberCluster.Name, err)
			return err
		}
	}

	return nil
}

func (c *Controller) dispatchPropagationWork(propagationWork *propagationstrategy.PropagationWork) error {
	memberClusterName, err := names.GetMemberClusterName(propagationWork.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for propagationWork %s/%s", propagationWork.Namespace, propagationWork.Name)
		return err
	}

	// TODO(RainbowMango): retrieve member cluster from the local cache instead of a real request to API server.
	memberCluster, err := c.KarmadaClient.MemberclusterV1alpha1().MemberClusters().Get(context.TODO(), memberClusterName, v1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to the get given member cluster %s", memberClusterName)
		return err
	}

	if !c.isMemberClusterReady(&memberCluster.Status) {
		klog.Errorf("The status of the given member cluster %s is unready", memberCluster.Name)
		return fmt.Errorf("cluster %s is not ready, requeuing operation until cluster state is ready", memberCluster.Name)
	}

	err = c.syncToMemberClusters(memberCluster, propagationWork)
	if err != nil {
		klog.Errorf("Failed to dispatch propagationWork %v, namespace is %v, err is %v", propagationWork.Name, propagationWork.Namespace, err)
		return err
	}

	return nil
}

// syncToMemberClusters ensures that the state of the given object is synchronized to member clusters.
func (c *Controller) syncToMemberClusters(memberCluster *v1alpha1.MemberCluster, propagationWork *propagationstrategy.PropagationWork) error {
	memberClusterDynamicClient, err := util.NewClusterDynamicClientSet(memberCluster, c.KubeClientSet)
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

		applied := c.isResourceApplied(&propagationWork.Status)
		if applied {
			err = c.updateResource(memberClusterDynamicClient, workload)
			if err != nil {
				klog.Errorf("Failed to update resource in the given member cluster %s, err is %v", memberCluster.Name, err)
				return err
			}
		} else {
			err = c.createResource(memberClusterDynamicClient, workload)
			if err != nil {
				klog.Errorf("Failed to create resource in the given member cluster %s, err is %v", memberCluster.Name, err)
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

// deleteResource delete resource in member cluster
func (c *Controller) deleteResource(memberClusterDynamicClient *util.DynamicClusterClient, workload *unstructured.Unstructured) error {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to delete resource(%s/%s) as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	err = memberClusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Delete(context.TODO(), workload.GetName(), v1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		klog.Errorf("Failed to delete resource %v, err is %v ", workload.GetName(), err)
		return err
	}
	return nil
}

// createResource create resource in member cluster
func (c *Controller) createResource(memberClusterDynamicClient *util.DynamicClusterClient, workload *unstructured.Unstructured) error {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to create resource(%s/%s) as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	_, err = memberClusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Create(context.TODO(), workload, v1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		klog.Errorf("Failed to create resource %v, err is %v ", workload.GetName(), err)
		return err
	}
	return nil
}

// updateResource update resource in member cluster
func (c *Controller) updateResource(memberClusterDynamicClient *util.DynamicClusterClient, workload *unstructured.Unstructured) error {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to update resource(%s/%s) as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	_, err = memberClusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Update(context.TODO(), workload, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update resource %v, err is %v ", workload.GetName(), err)
		return err
	}
	return nil
}

// removeFinalizer remove finalizer from the given propagationWork
func (c *Controller) removeFinalizer(propagationWork *propagationstrategy.PropagationWork) (controllerruntime.Result, error) {
	accessor, err := meta.Accessor(propagationWork)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if !finalizers.Has(finalizer) {
		return controllerruntime.Result{}, nil
	}
	finalizers.Delete(finalizer)
	accessor.SetFinalizers(finalizers.List())
	err = c.Client.Update(context.TODO(), propagationWork)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// ensureFinalizer ensure finalizer for the given PropagationWork
func (c *Controller) ensureFinalizer(propagationWork *propagationstrategy.PropagationWork) (bool, error) {
	accessor, err := meta.Accessor(propagationWork)
	if err != nil {
		return false, err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if finalizers.Has(finalizer) {
		return false, nil
	}
	finalizers.Insert(finalizer)
	accessor.SetFinalizers(finalizers.List())
	err = c.Client.Update(context.TODO(), propagationWork)
	return true, err
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
	err := c.Client.Update(context.TODO(), propagationWork)
	return err
}
