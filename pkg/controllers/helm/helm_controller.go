package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/hashicorp/go-retryablehttp"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "helm-controller"

	ready = "Ready"
)

// Controller is to sync Work with helm release.
type Controller struct {
	client.Client
	EventRecorder      record.EventRecorder
	HelmReleaseWatcher objectwatcher.HelmReleaseWatcher
	PredicateFunc      predicate.Predicate
	RatelimiterOptions ratelimiterflag.Options

	// SourceClient is used to download chart artifact
	SourceClient        *retryablehttp.Client
	RequeueDependency   time.Duration
	NoCrossNamespaceRef bool
	HTTPRetry           int
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (h *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling Work %s", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := h.Client.Get(context.TODO(), req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for work %s/%s", work.Namespace, work.Name)
		return controllerruntime.Result{Requeue: true}, err
	}

	cluster, err := util.GetCluster(h.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to get the given member cluster %s", clusterName)
		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		// Abort deleting workload if cluster is unready when unjoining cluster, otherwise the unjoin process will be failed.
		if util.IsClusterReady(&cluster.Status) {
			err := h.tryDeleteWorkload(ctx, clusterName, work)
			if err != nil {
				klog.Errorf("Failed to delete work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
				return controllerruntime.Result{Requeue: true}, err
			}
		} else if cluster.DeletionTimestamp.IsZero() { // cluster is unready, but not terminating
			return controllerruntime.Result{Requeue: true}, fmt.Errorf("cluster(%s) not ready", cluster.Name)
		}

		return h.removeFinalizer(work)
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("Stop sync work(%s/%s) for cluster(%s) as cluster not ready.", work.Namespace, work.Name, cluster.Name)
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	return h.syncWork(ctx, clusterName, work)
}

// SetupWithManager creates a controller and register to controller manager.
func (h *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	// Index the work by the HelmChart references they point at
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &workv1alpha1.Work{}, v2.SourceIndexKey,
		func(o client.Object) []string {
			work := o.(*workv1alpha1.Work)
			annotations := work.Annotations
			if annotations == nil {
				return []string{}
			}
			rbName := annotations[workv1alpha1.ResourceBindingNameLabel]
			rbNamespace := annotations[workv1alpha1.ResourceBindingNamespaceLabel]

			hrNamespaceName := types.NamespacedName{
				Namespace: rbNamespace,
				Name:      strings.Split(rbName, "-")[0],
			}
			hr := &v2.HelmRelease{}
			if err := h.Client.Get(context.TODO(), hrNamespaceName, hr); err != nil {
				return []string{}
			}
			return []string{
				fmt.Sprintf("%s/%s", hr.Spec.Chart.GetNamespace(hr.GetNamespace()), hr.GetHelmChartName()),
			}
		},
	); err != nil {
		return err
	}

	httpClient := retryablehttp.NewClient()
	httpClient.RetryWaitMin = 5 * time.Second
	httpClient.RetryWaitMax = 30 * time.Second
	httpClient.RetryMax = h.HTTPRetry
	httpClient.Logger = nil
	h.SourceClient = httpClient

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha1.Work{}, builder.WithPredicates(
			predicate.And(predicate.GenerationChangedPredicate{}, h.PredicateFunc),
		)).
		Watches(
			&source.Kind{Type: &sourcev1.HelmChart{}},
			handler.EnqueueRequestsFromMapFunc(h.requestsForHelmChartChange),
			builder.WithPredicates(SourceRevisionChangePredicate{}),
		).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(h.RatelimiterOptions),
		}).
		Complete(h)
}

//nolint:gosec
func (h *Controller) requestsForHelmChartChange(o client.Object) []reconcile.Request {
	hc, ok := o.(*sourcev1.HelmChart)
	if !ok {
		panic(fmt.Sprintf("Expected a HelmChart, got %T", o))
	}
	// If we do not have an artifact, we have no requests to make
	if hc.GetArtifact() == nil {
		return nil
	}

	ctx := context.Background()
	var list workv1alpha1.WorkList
	if err := h.List(ctx, &list, client.MatchingFields{
		v2.SourceIndexKey: client.ObjectKeyFromObject(hc).String(),
	}); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for _, i := range list.Items {
		if len(i.Status.ManifestStatuses) == 0 {
			continue
		}

		hrStatus := &v2.HelmReleaseStatus{}
		if err := json.Unmarshal(i.Status.ManifestStatuses[0].Status.Raw, hrStatus); err != nil {
			continue
		}

		// If the revision of the artifact equals to the last attempted revision,
		// we should not make a request for this HelmRelease
		if hc.GetArtifact().Revision == hrStatus.LastAttemptedRevision {
			continue
		}

		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&i)})
	}
	return reqs
}

func (h *Controller) syncWork(ctx context.Context, clusterName string, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	result, err := h.syncToClusters(ctx, clusterName, work)
	if err != nil {
		msg := fmt.Sprintf("Failed to sync work(%s) to cluster(%s): %v", work.Name, clusterName, err)
		klog.Errorf(msg)
		h.EventRecorder.Event(work, corev1.EventTypeWarning, workv1alpha1.EventReasonSyncWorkFailed, msg)
		return result, err
	}
	msg := fmt.Sprintf("Sync work (%s) to cluster(%s) successful.", work.Name, clusterName)
	klog.V(4).Infof(msg)
	h.EventRecorder.Event(work, corev1.EventTypeNormal, workv1alpha1.EventReasonSyncWorkSucceed, msg)
	return result, nil
}

// syncToClusters ensures that the state of the given object is synchronized to member clusters.
func (h *Controller) syncToClusters(ctx context.Context, clusterName string, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	var errs []error
	var msg string
	isReady := false
	// If there is no error or just error from reconcileV2HelmRelease, the requeue internal depends on reconcileV2HelmRelease.
	// Else it will requeue again.
	result := controllerruntime.Result{Requeue: true}
	syncSucceedNum := 0
	for i, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			errs = append(errs, err)
			continue
		}

		hr, err := helper.ConvertToHelmRelease(workload)
		if err != nil {
			klog.Errorf("Failed to convert workload to helmrelease, err is: %v", err)
			errs = append(errs, err)
			continue
		}

		hr.Status = v2.HelmReleaseStatus{}

		if len(work.Status.ManifestStatuses) > 0 {
			hrStatus := &v2.HelmReleaseStatus{}
			if err = json.Unmarshal(work.Status.ManifestStatuses[i].Status.Raw, hrStatus); err != nil {
				klog.Errorf("Failed to convert manifestStatus to helmrelease status, err is: %v", err)
				errs = append(errs, err)
				continue
			}
			hr.Status = *hrStatus
		}

		*hr, result, err = h.reconcileV2HelmRelease(ctx, *hr, clusterName)
		if err != nil {
			klog.Errorf("Failed to reconcile helmrelease(%v/%v) in the given member cluster %s, err is: %v", hr.GetNamespace(), hr.GetName(), clusterName, err)
			errs = append(errs, err)
			continue
		}

		err = h.patchManifestStatus(work, *hr)
		if err != nil {
			klog.Errorf("Failed to patch manifestStatus for helmrelease(%v/%v), err is: %v", hr.GetNamespace(), hr.GetName(), err)
			errs = append(errs, err)
			result = controllerruntime.Result{Requeue: true}
			continue
		}

		if meta.IsStatusConditionTrue(hr.Status.Conditions, ready) {
			isReady = true
			msg = fmt.Sprintf("Successfully synced helmrelease(%v/%v) to cluster %s", hr.GetNamespace(), hr.GetName(), clusterName)
			klog.V(4).Info(msg)
			syncSucceedNum++
		}
	}

	if len(errs) > 0 || !isReady {
		total := len(work.Spec.Workload.Manifests)
		message := fmt.Sprintf("Failed to apply all manifests (%v/%v): %v", syncSucceedNum, total, errors.NewAggregate(errs).Error())
		err := h.updateAppliedCondition(work, metav1.ConditionFalse, "AppliedFailed", message)
		if err != nil {
			klog.Errorf("Failed to update applied status for given work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
			errs = append(errs, err)
		}
		return result, errors.NewAggregate(errs)
	}

	err := h.updateAppliedCondition(work, metav1.ConditionTrue, "AppliedSuccessful", "Manifest has been successfully applied")
	if err != nil {
		klog.Errorf("Failed to update applied status for given work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
		result = controllerruntime.Result{Requeue: true}
		return result, err
	}

	return result, nil
}

// removeFinalizer remove finalizer from the given Work
func (h *Controller) removeFinalizer(work *workv1alpha1.Work) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(work, util.HelmControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(work, util.HelmControllerFinalizer)
	err := h.Client.Update(context.TODO(), work)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// updateAppliedCondition update the Applied condition for the given Work
func (h *Controller) updateAppliedCondition(work *workv1alpha1.Work, status metav1.ConditionStatus, reason, message string) error {
	newWorkAppliedCondition := metav1.Condition{
		Type:               workv1alpha1.WorkApplied,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		meta.SetStatusCondition(&work.Status.Conditions, newWorkAppliedCondition)
		updateErr := h.Status().Update(context.TODO(), work)
		if updateErr == nil {
			return nil
		}
		updated := &workv1alpha1.Work{}
		if err = h.Get(context.TODO(), client.ObjectKey{Namespace: work.Namespace, Name: work.Name}, updated); err == nil {
			// make a copy, so we don't mutate the shared cache
			work = updated.DeepCopy()
		} else {
			klog.Errorf("Failed to get updated work %s/%s: %v", work.Namespace, work.Name, err)
		}
		return updateErr
	})
}

func (h *Controller) patchManifestStatus(work *workv1alpha1.Work, hr v2.HelmRelease) error {
	status, err := helper.BuildStatusRawExtension(hr.Status)
	if err != nil {
		klog.Errorf("Failed to convert status from helmrelease, err is: %v", err)
	}

	manifestStatus := workv1alpha1.ManifestStatus{
		Identifier: workv1alpha1.ResourceIdentifier{
			Ordinal:   0,
			Group:     hr.GroupVersionKind().Group,
			Version:   hr.GroupVersionKind().Version,
			Kind:      hr.Kind,
			Namespace: hr.GetNamespace(),
			Name:      hr.GetName(),
		},
		Status: status,
	}

	workCopy := work.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		manifestStatuses := h.mergeStatus(workCopy.Status.ManifestStatuses, manifestStatus)
		if reflect.DeepEqual(workCopy.Status.ManifestStatuses, manifestStatuses) {
			return nil
		}
		workCopy.Status.ManifestStatuses = manifestStatuses
		updateErr := h.Status().Update(context.TODO(), workCopy)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha1.Work{}
		if err = h.Get(context.TODO(), client.ObjectKey{Namespace: workCopy.Namespace, Name: workCopy.Name}, updated); err == nil {
			//make a copy, so we don't mutate the shared cache
			workCopy = updated.DeepCopy()
		} else {
			klog.Errorf("Failed to get updated work %s/%s: %v", workCopy.Namespace, workCopy.Name, err)
		}
		return updateErr
	})
}

func (h *Controller) tryDeleteWorkload(ctx context.Context, clusterName string, work *workv1alpha1.Work) error {
	for i, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			return err
		}

		hr, err := helper.ConvertToHelmRelease(workload)
		if err != nil {
			klog.Errorf("Failed to convert workload to helmrelease, err is: %v", err)
			return err
		}

		if len(work.Status.ManifestStatuses) > 0 {
			hrStatus := &v2.HelmReleaseStatus{}
			if err = json.Unmarshal(work.Status.ManifestStatuses[i].Status.Raw, hrStatus); err != nil {
				klog.Errorf("Failed to convert manifestStatus to helmrelease status, err is: %v", err)
				return err
			}
			hr.Status = *hrStatus
		}

		err = h.deleteHelmChart(ctx, hr)
		if err != nil {
			klog.Error(err)
			return err
		}

		err = h.HelmReleaseWatcher.UninstallRelease(clusterName, *hr)
		if err != nil {
			klog.Errorf("Failed to delete helmrelease in the given member cluster %v, err is %v", clusterName, err)
			return err
		}
	}
	return nil
}

func (h *Controller) mergeStatus(statuses []workv1alpha1.ManifestStatus, newStatus workv1alpha1.ManifestStatus) []workv1alpha1.ManifestStatus {
	// For now, we only have at most one manifest in Work, so just override current 'statuses'.
	return []workv1alpha1.ManifestStatus{newStatus}
}
