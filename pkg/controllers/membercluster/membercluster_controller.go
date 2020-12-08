package membercluster

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/huawei-cloudnative/karmada/pkg/apis/membercluster/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName           = "membercluster-controller"
	finalizer                = "karmada.io/membercluster-controller"
	executionSpaceLabelKey   = "karmada.io/executionspace"
	executionSpaceLabelValue = ""
)

// Controller is to sync MemberCluster.
type Controller struct {
	client.Client                      // used to operate MemberCluster resources.
	KubeClientSet kubernetes.Interface // used to get kubernetes resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling memberCluster %s", req.NamespacedName.String())

	memberCluster := &v1alpha1.MemberCluster{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, memberCluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !memberCluster.DeletionTimestamp.IsZero() {
		return c.removeMemberCluster(memberCluster)
	}

	return c.syncMemberCluster(memberCluster)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.MemberCluster{}).Complete(c)
}

func (c *Controller) syncMemberCluster(memberCluster *v1alpha1.MemberCluster) (controllerruntime.Result, error) {
	// create execution space
	err := c.createExecutionSpace(memberCluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	// ensure finalizer
	updated, err := c.ensureFinalizer(memberCluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	} else if updated {
		return controllerruntime.Result{}, nil
	}

	// update status of the given member cluster
	// TODO: update status of member cluster in status controller.
	updateIndividualClusterStatus(memberCluster, c.Client, c.KubeClientSet)

	return controllerruntime.Result{RequeueAfter: 10 * time.Second}, nil
}

func (c *Controller) removeMemberCluster(memberCluster *v1alpha1.MemberCluster) (controllerruntime.Result, error) {
	err := c.removeExecutionSpace(memberCluster)
	if apierrors.IsNotFound(err) {
		return c.removeFinalizer(memberCluster)
	}
	if err != nil {
		klog.Errorf("Failed to remove execution space %v, err is %v", memberCluster.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	// make sure the given execution space has been deleted
	existES, err := c.ensureRemoveExecutionSpace(memberCluster)
	if err != nil {
		klog.Errorf("Failed to check weather the execution space exist in the given member cluster or not, error is: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	} else if existES {
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("requeuing operation until the execution space %v deleted, ", memberCluster.Name)
	}

	return c.removeFinalizer(memberCluster)
}

// removeExecutionSpace delete the given execution space
func (c *Controller) removeExecutionSpace(memberCluster *v1alpha1.MemberCluster) error {
	executionSpace, err := names.GenerateExecutionSpaceName(memberCluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", memberCluster.Name, err)
		return err
	}

	if err := c.KubeClientSet.CoreV1().Namespaces().Delete(context.TODO(), executionSpace, v1.DeleteOptions{}); err != nil {
		klog.Errorf("Error while deleting namespace %s: %s", executionSpace, err)
		return err
	}
	return nil
}

// ensureRemoveExecutionSpace make sure the given execution space has been deleted
func (c *Controller) ensureRemoveExecutionSpace(memberCluster *v1alpha1.MemberCluster) (bool, error) {
	executionSpace, err := names.GenerateExecutionSpaceName(memberCluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", memberCluster.Name, err)
		return false, err
	}

	_, err = c.KubeClientSet.CoreV1().Namespaces().Get(context.TODO(), executionSpace, v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		klog.Errorf("Failed to get execution space %v, err is %v ", executionSpace, err)
		return false, err
	}
	return true, nil
}

func (c *Controller) removeFinalizer(memberCluster *v1alpha1.MemberCluster) (controllerruntime.Result, error) {
	accessor, err := meta.Accessor(memberCluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if !finalizers.Has(finalizer) {
		return controllerruntime.Result{}, nil
	}
	finalizers.Delete(finalizer)
	accessor.SetFinalizers(finalizers.List())
	err = c.Client.Update(context.TODO(), memberCluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

func (c *Controller) ensureFinalizer(memberCluster *v1alpha1.MemberCluster) (bool, error) {
	accessor, err := meta.Accessor(memberCluster)
	if err != nil {
		return false, err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if finalizers.Has(finalizer) {
		return false, nil
	}
	finalizers.Insert(finalizer)
	accessor.SetFinalizers(finalizers.List())
	err = c.Client.Update(context.TODO(), memberCluster)
	return true, err
}

// createExecutionSpace create member cluster execution space when member cluster joined
func (c *Controller) createExecutionSpace(memberCluster *v1alpha1.MemberCluster) error {
	executionSpace, err := names.GenerateExecutionSpaceName(memberCluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", memberCluster.Name, err)
		return err
	}

	// create member cluster execution space when member cluster joined
	_, err = c.KubeClientSet.CoreV1().Namespaces().Get(context.TODO(), executionSpace, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			memberClusterES := &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name:   executionSpace,
					Labels: map[string]string{executionSpaceLabelKey: executionSpaceLabelValue},
				},
			}
			_, err = c.KubeClientSet.CoreV1().Namespaces().Create(context.TODO(), memberClusterES, v1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed to create execution space for membercluster %v", memberCluster.Name)
				return err
			}
		} else {
			klog.Errorf("Could not get %s namespace: %v", executionSpace, err)
			return err
		}
	}
	return nil
}
