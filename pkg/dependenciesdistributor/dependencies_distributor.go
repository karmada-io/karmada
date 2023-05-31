package dependenciesdistributor

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/detector"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	// bindingDependedByLabelKeyPrefix is the prefix to a label key specifying an attached binding referred by which independent binding.
	// the key is in the label of an attached binding which should be unique, because resource like secret can be referred by multiple deployments.
	bindingDependedByLabelKeyPrefix = "resourcebinding.karmada.io/depended-by-"
	// bindingDependenciesAnnotationKey represents the key of dependencies data (json serialized)
	// in the annotations of an independent binding.
	bindingDependenciesAnnotationKey = "resourcebinding.karmada.io/dependencies"
)

// DependenciesDistributor is to automatically propagate relevant resources.
// Resource binding will be created when a resource(e.g. deployment) is matched by a propagation policy, we call it independent binding in DependenciesDistributor.
// And when DependenciesDistributor works, it will create or update reference resource bindings of relevant resources(e.g. secret), which we call them attached bindings.
type DependenciesDistributor struct {
	// Client is used to retrieve objects, it is often more convenient than lister.
	Client client.Client
	// DynamicClient used to fetch arbitrary resources.
	DynamicClient       dynamic.Interface
	InformerManager     genericmanager.SingleClusterInformerManager
	EventHandler        cache.ResourceEventHandler
	EventRecorder       record.EventRecorder
	Processor           util.AsyncWorker
	RESTMapper          meta.RESTMapper
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	RateLimiterOptions  ratelimiterflag.Options
	GenericEvent        chan event.GenericEvent

	stopCh <-chan struct{}
}

// Check if our DependenciesDistributor implements necessary interfaces
var _ manager.Runnable = &DependenciesDistributor{}
var _ manager.LeaderElectionRunnable = &DependenciesDistributor{}

// NeedLeaderElection implements LeaderElectionRunnable interface.
// So that the distributor could run in the leader election mode.
func (d *DependenciesDistributor) NeedLeaderElection() bool {
	return true
}

// OnAdd handles object add event and push the object to queue.
func (d *DependenciesDistributor) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	d.Processor.Enqueue(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (d *DependenciesDistributor) OnUpdate(_, newObj interface{}) {
	d.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (d *DependenciesDistributor) OnDelete(obj interface{}) {
	d.OnAdd(obj)
}

// Reconcile performs a full reconciliation for the object referred to by the key.
// The key will be re-queued if an error is non-nil.
func (d *DependenciesDistributor) reconcile(key util.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("Invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.V(4).Infof("DependenciesDistributor start to reconcile object: %s", clusterWideKey)
	bindingList := &workv1alpha2.ResourceBindingList{}
	err := d.Client.List(context.TODO(), bindingList, &client.ListOptions{
		Namespace:     clusterWideKey.Namespace,
		LabelSelector: labels.Everything()})
	if err != nil {
		return err
	}

	var errs []error
	for i := range bindingList.Items {
		binding := &bindingList.Items[i]
		if !binding.DeletionTimestamp.IsZero() {
			continue
		}

		matched, err := dependentObjectReferenceMatches(clusterWideKey, binding)
		if err != nil {
			klog.Errorf("Failed to evaluate if binding(%s/%s) need to sync dependencies: %v", binding.Namespace, binding.Name, err)
			errs = append(errs, err)
			continue
		} else if !matched {
			klog.V(4).Infof("No need to sync binding(%s/%s)", binding.Namespace, binding.Name)
			continue
		}

		klog.V(4).Infof("Resource binding(%s/%s) is matched for resource(%s/%s)", binding.Namespace, binding.Name, clusterWideKey.Namespace, clusterWideKey.Name)
		d.GenericEvent <- event.GenericEvent{Object: binding}
	}

	return utilerrors.NewAggregate(errs)
}

// dependentObjectReferenceMatches tells if the given object is referred by current resource binding.
func dependentObjectReferenceMatches(objectKey keys.ClusterWideKey, referenceBinding *workv1alpha2.ResourceBinding) (bool, error) {
	dependencies, exist := referenceBinding.Annotations[bindingDependenciesAnnotationKey]
	if !exist {
		return false, nil
	}

	var dependenciesSlice []configv1alpha1.DependentObjectReference
	err := json.Unmarshal([]byte(dependencies), &dependenciesSlice)
	if err != nil {
		return false, err
	}

	if len(dependenciesSlice) == 0 {
		return false, nil
	}

	for _, dependence := range dependenciesSlice {
		if objectKey.GroupVersion().String() == dependence.APIVersion &&
			objectKey.Kind == dependence.Kind &&
			objectKey.Namespace == dependence.Namespace &&
			objectKey.Name == dependence.Name {
			return true, nil
		}
	}

	return false, nil
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (d *DependenciesDistributor) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("Start to reconcile ResourceBinding(%s)", request.NamespacedName)
	bindingObject := &workv1alpha2.ResourceBinding{}
	err := d.Client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: request.Name}, bindingObject)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ResourceBinding(%s) has been removed.", request.NamespacedName)
			return reconcile.Result{}, d.handleResourceBindingDeletion(request.Namespace, request.Name)
		}
		klog.Errorf("Failed to get ResourceBinding(%s): %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// in case users set PropagateDeps field from "true" to "false"
	if !bindingObject.Spec.PropagateDeps || !bindingObject.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, d.handleResourceBindingDeletion(request.Namespace, request.Name)
	}

	workload, err := helper.FetchResourceTemplate(d.DynamicClient, d.InformerManager, d.RESTMapper, bindingObject.Spec.Resource)
	if err != nil {
		klog.Errorf("Failed to fetch workload for resourceBinding(%s/%s). Error: %v.", bindingObject.Namespace, bindingObject.Name, err)
		return reconcile.Result{}, err
	}

	if !d.ResourceInterpreter.HookEnabled(workload.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretDependency) {
		return reconcile.Result{}, nil
	}

	dependencies, err := d.ResourceInterpreter.GetDependencies(workload)
	if err != nil {
		klog.Errorf("Failed to customize dependencies for %s(%s), %v", workload.GroupVersionKind(), workload.GetName(), err)
		d.EventRecorder.Eventf(workload, corev1.EventTypeWarning, events.EventReasonGetDependenciesFailed, err.Error())
		return reconcile.Result{}, err
	}
	d.EventRecorder.Eventf(workload, corev1.EventTypeNormal, events.EventReasonGetDependenciesSucceed, "Get dependencies(%+v) succeed.", dependencies)
	return reconcile.Result{}, d.syncScheduleResultToAttachedBindings(bindingObject, dependencies)
}

func (d *DependenciesDistributor) handleResourceBindingDeletion(namespace, name string) error {
	attachedBindings, err := d.listAttachedBindings(namespace, name)
	if err != nil {
		return err
	}

	return d.removeScheduleResultFromAttachedBindings(namespace, name, attachedBindings)
}

func (d *DependenciesDistributor) syncScheduleResultToAttachedBindings(binding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) (err error) {
	defer func() {
		if err != nil {
			d.EventRecorder.Eventf(binding, corev1.EventTypeWarning, events.EventReasonSyncScheduleResultToDependenciesFailed, err.Error())
		} else {
			d.EventRecorder.Eventf(binding, corev1.EventTypeNormal, events.EventReasonSyncScheduleResultToDependenciesSucceed, "Sync schedule results to dependencies succeed.")
		}
	}()

	if err := d.recordDependenciesForIndependentBinding(binding, dependencies); err != nil {
		return err
	}

	// remove orphan attached bindings
	orphanBindings, err := d.findOrphanAttachedResourceBindings(binding, dependencies)
	if err != nil {
		klog.Errorf("Failed to find orphan attached bindings for resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return err
	}

	err = d.removeScheduleResultFromAttachedBindings(binding.Namespace, binding.Name, orphanBindings)
	if err != nil {
		klog.Errorf("Failed to remove orphan attached bindings by resourceBinding(%s/%s). Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return err
	}

	// create or update attached bindings
	var errs []error
	var startInformerManager bool
	for _, dependent := range dependencies {
		resource := workv1alpha2.ObjectReference{
			APIVersion: dependent.APIVersion,
			Kind:       dependent.Kind,
			Namespace:  dependent.Namespace,
			Name:       dependent.Name,
		}
		gvr, err := restmapper.GetGroupVersionResource(d.RESTMapper, schema.FromAPIVersionAndKind(dependent.APIVersion, dependent.Kind))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !d.InformerManager.IsHandlerExist(gvr, d.EventHandler) {
			d.InformerManager.ForResource(gvr, d.EventHandler)
			startInformerManager = true
		}
		rawObject, err := helper.FetchResourceTemplate(d.DynamicClient, d.InformerManager, d.RESTMapper, resource)
		if err != nil {
			// do nothing if resource template not exist.
			if apierrors.IsNotFound(err) {
				continue
			}
			errs = append(errs, err)
			continue
		}

		attachedBinding := buildAttachedBinding(binding, rawObject)
		if err := d.createOrUpdateAttachedBinding(attachedBinding); err != nil {
			errs = append(errs, err)
		}
	}
	if startInformerManager {
		d.InformerManager.Start()
		d.InformerManager.WaitForCacheSync()
	}
	return utilerrors.NewAggregate(errs)
}

func (d *DependenciesDistributor) recordDependenciesForIndependentBinding(binding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) error {
	dependenciesBytes, err := json.Marshal(dependencies)
	if err != nil {
		klog.Errorf("Failed to marshal dependencies of binding(%s/%s): %v", binding.Namespace, binding.Name, err)
		return err
	}

	objectAnnotation := binding.GetAnnotations()
	if objectAnnotation == nil {
		objectAnnotation = make(map[string]string, 1)
	}

	// dependencies are not updated, no need to update annotation.
	if oldDependencies, exist := objectAnnotation[bindingDependenciesAnnotationKey]; exist && oldDependencies == string(dependenciesBytes) {
		return nil
	}

	objectAnnotation[bindingDependenciesAnnotationKey] = string(dependenciesBytes)

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		binding.SetAnnotations(objectAnnotation)
		updateErr := d.Client.Update(context.TODO(), binding)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha2.ResourceBinding{}
		if err = d.Client.Get(context.TODO(), client.ObjectKey{Namespace: binding.Namespace, Name: binding.Name}, updated); err == nil {
			binding = updated
		} else {
			klog.Errorf("Failed to get updated binding %s/%s: %v", binding.Namespace, binding.Name, err)
		}
		return updateErr
	})
}

func (d *DependenciesDistributor) findOrphanAttachedResourceBindings(independentBinding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) ([]*workv1alpha2.ResourceBinding, error) {
	attachedBindings, err := d.listAttachedBindings(independentBinding.Namespace, independentBinding.Name)
	if err != nil {
		return nil, err
	}

	dependenciesSets := sets.NewString()
	for _, dependency := range dependencies {
		key := generateDependencyKey(dependency.Kind, dependency.APIVersion, dependency.Namespace, dependency.Name)
		dependenciesSets.Insert(key)
	}

	var orphanAttachedBindings []*workv1alpha2.ResourceBinding
	for _, attachedBinding := range attachedBindings {
		key := generateDependencyKey(attachedBinding.Spec.Resource.Kind, attachedBinding.Spec.Resource.APIVersion, attachedBinding.Spec.Resource.Namespace, attachedBinding.Spec.Resource.Name)
		if !dependenciesSets.Has(key) {
			orphanAttachedBindings = append(orphanAttachedBindings, attachedBinding)
		}
	}
	return orphanAttachedBindings, nil
}

func (d *DependenciesDistributor) listAttachedBindings(bindingNamespace, bindingName string) (res []*workv1alpha2.ResourceBinding, err error) {
	label := generateBindingDependedByLabel(bindingNamespace, bindingName)
	selector := labels.SelectorFromSet(label)
	bindingList := &workv1alpha2.ResourceBindingList{}
	err = d.Client.List(context.TODO(), bindingList, &client.ListOptions{
		Namespace:     bindingNamespace,
		LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	for i := range bindingList.Items {
		res = append(res, &bindingList.Items[i])
	}
	return res, nil
}

func (d *DependenciesDistributor) removeScheduleResultFromAttachedBindings(bindingNamespace, bindingName string, attachedBindings []*workv1alpha2.ResourceBinding) error {
	if len(attachedBindings) == 0 {
		return nil
	}

	bindingLabelKey := generateBindingDependedByLabelKey(bindingNamespace, bindingName)

	var errs []error
	for index, binding := range attachedBindings {
		delete(attachedBindings[index].Labels, bindingLabelKey)
		updatedSnapshot := deleteBindingFromSnapshot(bindingNamespace, bindingName, attachedBindings[index].Spec.RequiredBy)
		attachedBindings[index].Spec.RequiredBy = updatedSnapshot
		if err := d.Client.Update(context.TODO(), attachedBindings[index]); err != nil {
			klog.Errorf("Failed to update binding(%s/%s): %v", binding.Namespace, binding.Name, err)
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (d *DependenciesDistributor) createOrUpdateAttachedBinding(attachedBinding *workv1alpha2.ResourceBinding) error {
	if err := d.Client.Create(context.TODO(), attachedBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Infof("Failed to create resource binding(%s/%s): %v", attachedBinding.Namespace, attachedBinding.Name, err)
			return err
		}

		existBinding := &workv1alpha2.ResourceBinding{}
		key := client.ObjectKeyFromObject(attachedBinding)
		if err := d.Client.Get(context.TODO(), key, existBinding); err != nil {
			klog.Infof("Failed to get resource binding(%s/%s): %v", attachedBinding.Namespace, attachedBinding.Name, err)
			return err
		}

		updatedBindingSnapshot := mergeBindingSnapshot(existBinding.Spec.RequiredBy, attachedBinding.Spec.RequiredBy)
		existBinding.Spec.RequiredBy = updatedBindingSnapshot
		existBinding.Labels = util.DedupeAndMergeLabels(existBinding.Labels, attachedBinding.Labels)
		existBinding.Spec.Resource = attachedBinding.Spec.Resource

		if err := d.Client.Update(context.TODO(), existBinding); err != nil {
			klog.Errorf("Failed to update resource binding(%s/%s): %v", existBinding.Namespace, existBinding.Name, err)
			return err
		}
	}
	return nil
}

// Start runs the distributor, never stop until stopCh closed.
func (d *DependenciesDistributor) Start(ctx context.Context) error {
	klog.Infof("Starting dependencies distributor.")
	d.stopCh = ctx.Done()
	resourceWorkerOptions := util.Options{
		Name:          "resource detector",
		KeyFunc:       detector.ClusterWideKeyFunc,
		ReconcileFunc: d.reconcile,
	}
	d.EventHandler = fedinformer.NewHandlerOnEvents(d.OnAdd, d.OnUpdate, d.OnDelete)
	d.Processor = util.NewAsyncWorker(resourceWorkerOptions)
	d.Processor.Run(2, d.stopCh)
	<-d.stopCh

	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (d *DependenciesDistributor) SetupWithManager(mgr controllerruntime.Manager) error {
	return utilerrors.NewAggregate([]error{
		mgr.Add(d),
		controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha2.ResourceBinding{}).
			WithEventFilter(predicate.Funcs{
				CreateFunc: func(event event.CreateEvent) bool {
					bindingObject := event.Object.(*workv1alpha2.ResourceBinding)
					if !bindingObject.Spec.PropagateDeps {
						return false
					}
					if len(bindingObject.Spec.Clusters) == 0 {
						klog.V(4).Infof("Dropping resource binding(%s/%s) as it is not scheduled yet.", bindingObject.Namespace, bindingObject.Name)
						return false
					}
					return true
				},
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
					bindingObject := deleteEvent.Object.(*workv1alpha2.ResourceBinding)
					return bindingObject.Spec.PropagateDeps
				},
				UpdateFunc: func(updateEvent event.UpdateEvent) bool {
					oldBindingObject := updateEvent.ObjectOld.(*workv1alpha2.ResourceBinding)
					newBindingObject := updateEvent.ObjectNew.(*workv1alpha2.ResourceBinding)
					if oldBindingObject.Generation == newBindingObject.Generation {
						klog.V(4).Infof("Dropping resource binding(%s/%s) as the Generation is not changed.", newBindingObject.Namespace, newBindingObject.Name)
						return false
					}

					// prevent newBindingObject from the queue if it's not scheduled yet.
					if len(oldBindingObject.Spec.Clusters) == 0 && len(newBindingObject.Spec.Clusters) == 0 {
						klog.V(4).Infof("Dropping resource binding(%s/%s) as it is not scheduled yet.", newBindingObject.Namespace, newBindingObject.Name)
						return false
					}
					return oldBindingObject.Spec.PropagateDeps || newBindingObject.Spec.PropagateDeps
				},
			}).
			WithOptions(controller.Options{
				RateLimiter:             ratelimiterflag.DefaultControllerRateLimiter(d.RateLimiterOptions),
				MaxConcurrentReconciles: 2,
			}).
			Watches(&source.Channel{Source: d.GenericEvent}, &handler.EnqueueRequestForObject{}).
			Complete(d),
	})
}

func generateBindingDependedByLabel(bindingNamespace, bindingName string) map[string]string {
	labelKey := generateBindingDependedByLabelKey(bindingNamespace, bindingName)
	labelValue := fmt.Sprintf(bindingNamespace + "_" + bindingName)
	return map[string]string{labelKey: labelValue}
}

func generateBindingDependedByLabelKey(bindingNamespace, bindingName string) string {
	bindHashKey := names.GenerateBindingReferenceKey(bindingNamespace, bindingName)
	return fmt.Sprintf(bindingDependedByLabelKeyPrefix + bindHashKey)
}

func generateDependencyKey(kind, apiVersion, name, namespace string) string {
	if len(namespace) == 0 {
		return kind + "-" + apiVersion + "-" + name
	}

	return kind + "-" + apiVersion + "-" + namespace + "-" + name
}

func buildAttachedBinding(binding *workv1alpha2.ResourceBinding, object *unstructured.Unstructured) *workv1alpha2.ResourceBinding {
	dependedByLabels := generateBindingDependedByLabel(binding.Namespace, binding.Name)

	var result []workv1alpha2.BindingSnapshot
	result = append(result, workv1alpha2.BindingSnapshot{
		Namespace: binding.Namespace,
		Name:      binding.Name,
		Clusters:  binding.Spec.Clusters,
	})

	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.GenerateBindingName(object.GetKind(), object.GetName()),
			Namespace: binding.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, object.GroupVersionKind()),
			},
			Labels:     dependedByLabels,
			Finalizers: []string{util.BindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      object.GetAPIVersion(),
				Kind:            object.GetKind(),
				Namespace:       object.GetNamespace(),
				Name:            object.GetName(),
				ResourceVersion: object.GetResourceVersion(),
			},
			RequiredBy: result,
		},
	}
}

func mergeBindingSnapshot(existSnapshot, newSnapshot []workv1alpha2.BindingSnapshot) []workv1alpha2.BindingSnapshot {
	if len(existSnapshot) == 0 {
		return newSnapshot
	}

	for _, newBinding := range newSnapshot {
		existInOldSnapshot := false
		for i := range existSnapshot {
			if existSnapshot[i].Namespace == newBinding.Namespace &&
				existSnapshot[i].Name == newBinding.Name {
				existSnapshot[i].Clusters = newBinding.Clusters
				existInOldSnapshot = true
			}
		}
		if !existInOldSnapshot {
			existSnapshot = append(existSnapshot, newBinding)
		}
	}

	return existSnapshot
}

func deleteBindingFromSnapshot(bindingNamespace, bindingName string, existSnapshot []workv1alpha2.BindingSnapshot) []workv1alpha2.BindingSnapshot {
	for i := 0; i < len(existSnapshot); i++ {
		if existSnapshot[i].Namespace == bindingNamespace &&
			existSnapshot[i].Name == bindingName {
			existSnapshot = append(existSnapshot[:i], existSnapshot[i+1:]...)
			i--
		}
	}
	return existSnapshot
}
