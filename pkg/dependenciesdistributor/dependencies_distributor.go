/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dependenciesdistributor

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/eventfilter"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "dependencies-distributor"
)

// well-know labels
const (
	// dependedByLabelKeyPrefix is added to the attached binding, it is the
	// prefix of the label key which specifying the current attached binding
	// referred by which independent binding.
	// the key in the labels of the current attached binding should be unique,
	// because resource like secret can be referred by multiple deployments.
	dependedByLabelKeyPrefix = "resourcebinding.karmada.io/depended-by-"
)

// well-know annotations
const (
	// dependenciesAnnotationKey is added to the independent binding,
	// it describes the names of dependencies (json serialized).
	dependenciesAnnotationKey = "resourcebinding.karmada.io/dependencies"
)

// LabelsKey is the object key which is a unique identifier under a cluster, across all resources.
type LabelsKey struct {
	keys.ClusterWideKey
	// Labels is the labels of the referencing object.
	Labels map[string]string
}

// DependenciesDistributor is to automatically propagate relevant resources.
// ResourceBinding will be created when a resource(e.g. deployment) is matched by a propagation policy,
// we call it independent binding in DependenciesDistributor.
// And when DependenciesDistributor works, it will create or update reference resourceBindings of
// relevant resources(e.g. secret), which we call them attached bindings.
type DependenciesDistributor struct {
	// Client is used to retrieve objects, it is often more convenient than lister.
	Client client.Client
	// DynamicClient used to fetch arbitrary resources.
	DynamicClient       dynamic.Interface
	InformerManager     genericmanager.SingleClusterInformerManager
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	RateLimiterOptions  ratelimiterflag.Options

	eventHandler      cache.ResourceEventHandler
	resourceProcessor util.AsyncWorker
	genericEvent      chan event.TypedGenericEvent[*workv1alpha2.ResourceBinding]
	// ConcurrentDependentResourceSyncs is the number of dependent resource that are allowed to sync concurrently.
	ConcurrentDependentResourceSyncs int
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
	d.resourceProcessor.Enqueue(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (d *DependenciesDistributor) OnUpdate(oldObj, newObj interface{}) {
	unstructuredOldObj, err := helper.ToUnstructured(oldObj)
	if err != nil {
		klog.Errorf("Failed to transform oldObj, error: %v", err)
		return
	}

	unstructuredNewObj, err := helper.ToUnstructured(newObj)
	if err != nil {
		klog.Errorf("Failed to transform newObj, error: %v", err)
		return
	}

	if !eventfilter.SpecificationChanged(unstructuredOldObj, unstructuredNewObj) {
		klog.V(4).Infof("Ignore update event of object (%s, kind=%s, %s) as specification no change", unstructuredOldObj.GetAPIVersion(), unstructuredOldObj.GetKind(), names.NamespacedKey(unstructuredOldObj.GetNamespace(), unstructuredOldObj.GetName()))
		return
	}
	if !equality.Semantic.DeepEqual(unstructuredOldObj.GetLabels(), unstructuredNewObj.GetLabels()) {
		d.OnAdd(oldObj)
	}
	d.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (d *DependenciesDistributor) OnDelete(obj interface{}) {
	d.OnAdd(obj)
}

// reconcileResourceTemplate coordinates resources that may need to be distributed, such as Configmap, Service, etc.
// When the resource is confirmed to need to be distributed, it will be processed by DependenciesDistributor.Reconcile.
// The key will be re-queued if an error is non-nil.
func (d *DependenciesDistributor) reconcileResourceTemplate(key util.QueueKey) error {
	resourceTemplateKey, ok := key.(*LabelsKey)
	if !ok {
		klog.Error("Invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.V(4).Infof("DependenciesDistributor start to reconcile object: %s", resourceTemplateKey)
	readonlyBindingList := &workv1alpha2.ResourceBindingList{}
	err := d.Client.List(context.TODO(), readonlyBindingList, &client.ListOptions{
		Namespace:             resourceTemplateKey.Namespace,
		LabelSelector:         labels.Everything(),
		UnsafeDisableDeepCopy: ptr.To(true),
	})
	if err != nil {
		return err
	}

	for i := range readonlyBindingList.Items {
		binding := &readonlyBindingList.Items[i]
		if !binding.DeletionTimestamp.IsZero() {
			continue
		}

		matched := matchesWithBindingDependencies(resourceTemplateKey, binding)
		if !matched {
			continue
		}

		klog.V(4).Infof("ResourceBinding(%s/%s) is matched for resource(%s/%s)", binding.Namespace, binding.Name, resourceTemplateKey.Namespace, resourceTemplateKey.Name)
		d.genericEvent <- event.TypedGenericEvent[*workv1alpha2.ResourceBinding]{Object: binding}
	}

	return nil
}

// matchesWithBindingDependencies tells if the given object(resource template) is matched
// with the dependencies of independent resourceBinding.
func matchesWithBindingDependencies(resourceTemplateKey *LabelsKey, independentBinding *workv1alpha2.ResourceBinding) bool {
	dependencies, exist := independentBinding.Annotations[dependenciesAnnotationKey]
	if !exist {
		return false
	}

	var dependenciesSlice []configv1alpha1.DependentObjectReference
	err := json.Unmarshal([]byte(dependencies), &dependenciesSlice)
	if err != nil {
		// If unmarshal fails, retrying with an error return will not solve the problem.
		// It will only increase the consumption by repeatedly listing the binding.
		// Therefore, it is better to print this error and ignore it.
		klog.Errorf("Failed to unmarshal binding(%s/%s) dependencies(%s): %v",
			independentBinding.Namespace, independentBinding.Name, dependencies, err)
		return false
	}
	if len(dependenciesSlice) == 0 {
		return false
	}

	for _, dependency := range dependenciesSlice {
		if resourceTemplateKey.GroupVersion().String() == dependency.APIVersion &&
			resourceTemplateKey.Kind == dependency.Kind &&
			resourceTemplateKey.Namespace == dependency.Namespace {
			if len(dependency.Name) != 0 {
				return dependency.Name == resourceTemplateKey.Name
			}
			var selector labels.Selector
			if selector, err = metav1.LabelSelectorAsSelector(dependency.LabelSelector); err != nil {
				klog.Errorf("Failed to converts the LabelSelector of binding(%s/%s) dependencies(%s): %v",
					independentBinding.Namespace, independentBinding.Name, dependencies, err)
				return false
			}
			return selector.Matches(labels.Set(resourceTemplateKey.Labels))
		}
	}
	return false
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (d *DependenciesDistributor) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("Start to reconcile ResourceBinding(%s)", request.NamespacedName)
	bindingObject := &workv1alpha2.ResourceBinding{}
	err := d.Client.Get(ctx, request.NamespacedName, bindingObject)
	if err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.Errorf("Failed to get ResourceBinding(%s): %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// in case users set PropagateDeps field from "true" to "false"
	if !bindingObject.Spec.PropagateDeps || !bindingObject.DeletionTimestamp.IsZero() {
		err = d.handleIndependentBindingDeletion(bindingObject.Labels[workv1alpha2.ResourceBindingPermanentIDLabel], request.Namespace, request.Name)
		if err != nil {
			klog.Errorf("Failed to cleanup attached bindings for independent binding(%s): %v", request.NamespacedName, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, d.removeFinalizer(ctx, bindingObject)
	}

	workload, err := helper.FetchResourceTemplate(ctx, d.DynamicClient, d.InformerManager, d.RESTMapper, bindingObject.Spec.Resource)
	if err != nil {
		klog.Errorf("Failed to fetch workload for resourceBinding(%s): %v.", request.NamespacedName, err)
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

	if err = d.addFinalizer(ctx, bindingObject); err != nil {
		klog.Errorf("Failed to add finalizer(%s) for ResourceBinding(%s): %v", util.BindingDependenciesDistributorFinalizer, request.NamespacedName, err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, d.syncScheduleResultToAttachedBindings(ctx, bindingObject, dependencies)
}

func (d *DependenciesDistributor) addFinalizer(ctx context.Context, independentBinding *workv1alpha2.ResourceBinding) error {
	if controllerutil.AddFinalizer(independentBinding, util.BindingDependenciesDistributorFinalizer) {
		return d.Client.Update(ctx, independentBinding)
	}
	return nil
}

func (d *DependenciesDistributor) removeFinalizer(ctx context.Context, independentBinding *workv1alpha2.ResourceBinding) error {
	if controllerutil.RemoveFinalizer(independentBinding, util.BindingDependenciesDistributorFinalizer) {
		return d.Client.Update(ctx, independentBinding)
	}
	return nil
}

func (d *DependenciesDistributor) handleIndependentBindingDeletion(id, namespace, name string) error {
	attachedBindings, err := d.listAttachedBindings(id, namespace, name)
	if err != nil {
		return err
	}

	return d.removeScheduleResultFromAttachedBindings(namespace, name, attachedBindings)
}

func (d *DependenciesDistributor) removeOrphanAttachedBindings(ctx context.Context, independentBinding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) error {
	// remove orphan attached bindings
	orphanBindings, err := d.findOrphanAttachedBindings(ctx, independentBinding, dependencies)
	if err != nil {
		klog.Errorf("Failed to find orphan attached bindings for resourceBinding(%s/%s). Error: %v.",
			independentBinding.GetNamespace(), independentBinding.GetName(), err)
		return err
	}
	err = d.removeScheduleResultFromAttachedBindings(independentBinding.Namespace, independentBinding.Name, orphanBindings)
	if err != nil {
		klog.Errorf("Failed to remove orphan attached bindings by resourceBinding(%s/%s). Error: %v.",
			independentBinding.GetNamespace(), independentBinding.GetName(), err)
		return err
	}
	return nil
}

func (d *DependenciesDistributor) handleDependentResource(
	ctx context.Context,
	independentBinding *workv1alpha2.ResourceBinding,
	dependent configv1alpha1.DependentObjectReference) error {
	objRef := workv1alpha2.ObjectReference{
		APIVersion: dependent.APIVersion,
		Kind:       dependent.Kind,
		Namespace:  dependent.Namespace,
		Name:       dependent.Name,
	}

	switch {
	case len(dependent.Name) != 0:
		rawObject, err := helper.FetchResourceTemplate(ctx, d.DynamicClient, d.InformerManager, d.RESTMapper, objRef)
		if err != nil {
			// do nothing if resource template not exist.
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		attachedBinding := buildAttachedBinding(independentBinding, rawObject)
		return d.createOrUpdateAttachedBinding(attachedBinding)
	case dependent.LabelSelector != nil:
		var selector labels.Selector
		var err error
		if selector, err = metav1.LabelSelectorAsSelector(dependent.LabelSelector); err != nil {
			return err
		}
		rawObjects, err := helper.FetchResourceTemplatesByLabelSelector(d.DynamicClient, d.InformerManager, d.RESTMapper, objRef, selector)
		if err != nil {
			return err
		}
		for _, rawObject := range rawObjects {
			attachedBinding := buildAttachedBinding(independentBinding, rawObject)
			if err := d.createOrUpdateAttachedBinding(attachedBinding); err != nil {
				return err
			}
		}
		return nil
	}
	// can not reach here
	return fmt.Errorf("the Name and LabelSelector in the DependentObjectReference cannot be empty at the same time")
}

func (d *DependenciesDistributor) syncScheduleResultToAttachedBindings(ctx context.Context, independentBinding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) (err error) {
	defer func() {
		if err != nil {
			d.EventRecorder.Eventf(independentBinding, corev1.EventTypeWarning, events.EventReasonSyncScheduleResultToDependenciesFailed, err.Error())
		} else {
			d.EventRecorder.Eventf(independentBinding, corev1.EventTypeNormal, events.EventReasonSyncScheduleResultToDependenciesSucceed, "Sync schedule results to dependencies succeed.")
		}
	}()

	if err = d.recordDependencies(ctx, independentBinding, dependencies); err != nil {
		return err
	}
	if err = d.removeOrphanAttachedBindings(ctx, independentBinding, dependencies); err != nil {
		return err
	}

	// create or update attached bindings
	var errs []error
	var startInformerManager bool
	for _, dependent := range dependencies {
		gvr, err := restmapper.GetGroupVersionResource(d.RESTMapper, schema.FromAPIVersionAndKind(dependent.APIVersion, dependent.Kind))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !d.InformerManager.IsHandlerExist(gvr, d.eventHandler) {
			d.InformerManager.ForResource(gvr, d.eventHandler)
			startInformerManager = true
		}
		errs = append(errs, d.handleDependentResource(ctx, independentBinding, dependent))
	}
	if startInformerManager {
		d.InformerManager.Start()
		d.InformerManager.WaitForCacheSync()
	}
	return utilerrors.NewAggregate(errs)
}

func (d *DependenciesDistributor) recordDependencies(ctx context.Context, independentBinding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) error {
	bindingKey := client.ObjectKey{Namespace: independentBinding.Namespace, Name: independentBinding.Name}

	dependenciesBytes, err := json.Marshal(dependencies)
	if err != nil {
		klog.Errorf("Failed to marshal dependencies of binding(%s): %v", bindingKey, err)
		return err
	}
	dependenciesStr := string(dependenciesBytes)

	objectAnnotation := independentBinding.GetAnnotations()
	if objectAnnotation == nil {
		objectAnnotation = make(map[string]string, 1)
	}

	// dependencies are not updated, no need to update annotation.
	if oldDependencies, exist := objectAnnotation[dependenciesAnnotationKey]; exist && oldDependencies == dependenciesStr {
		return nil
	}
	objectAnnotation[dependenciesAnnotationKey] = dependenciesStr

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		independentBinding.SetAnnotations(objectAnnotation)
		updateErr := d.Client.Update(ctx, independentBinding)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha2.ResourceBinding{}
		if err = d.Client.Get(ctx, bindingKey, updated); err == nil {
			independentBinding = updated
		} else {
			klog.Errorf("Failed to get updated binding(%s): %v", bindingKey, err)
		}
		return updateErr
	})
}

func (d *DependenciesDistributor) findOrphanAttachedBindings(ctx context.Context, independentBinding *workv1alpha2.ResourceBinding, dependencies []configv1alpha1.DependentObjectReference) ([]*workv1alpha2.ResourceBinding, error) {
	attachedBindings, err := d.listAttachedBindings(independentBinding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel],
		independentBinding.Namespace, independentBinding.Name)
	if err != nil {
		return nil, err
	}

	dependenciesMaps := make(map[string][]int, 0)
	for index, dependency := range dependencies {
		key := generateDependencyKey(dependency.Kind, dependency.APIVersion, dependency.Namespace)
		dependenciesMaps[key] = append(dependenciesMaps[key], index)
	}

	var orphanAttachedBindings []*workv1alpha2.ResourceBinding
	for _, attachedBinding := range attachedBindings {
		key := generateDependencyKey(attachedBinding.Spec.Resource.Kind, attachedBinding.Spec.Resource.APIVersion, attachedBinding.Spec.Resource.Namespace)
		dependencyIndexes, exist := dependenciesMaps[key]
		if !exist {
			orphanAttachedBindings = append(orphanAttachedBindings, attachedBinding)
			continue
		}
		isOrphanAttachedBinding, err := d.isOrphanAttachedBindings(ctx, dependencies, dependencyIndexes, attachedBinding)
		if err != nil {
			return nil, err
		}
		if isOrphanAttachedBinding {
			orphanAttachedBindings = append(orphanAttachedBindings, attachedBinding)
		}
	}
	return orphanAttachedBindings, nil
}

func (d *DependenciesDistributor) isOrphanAttachedBindings(
	ctx context.Context,
	dependencies []configv1alpha1.DependentObjectReference,
	dependencyIndexes []int,
	attachedBinding *workv1alpha2.ResourceBinding) (bool, error) {
	var resource = attachedBinding.Spec.Resource
	for _, idx := range dependencyIndexes {
		dependency := dependencies[idx]
		switch {
		case len(dependency.Name) != 0:
			if dependency.Name == resource.Name {
				return false, nil
			}
		case dependency.LabelSelector != nil:
			var selector labels.Selector
			var err error
			if selector, err = metav1.LabelSelectorAsSelector(dependency.LabelSelector); err != nil {
				return false, err
			}
			rawObject, err := helper.FetchResourceTemplate(ctx, d.DynamicClient, d.InformerManager, d.RESTMapper, workv1alpha2.ObjectReference{
				APIVersion: resource.APIVersion,
				Kind:       resource.Kind,
				Namespace:  resource.Namespace,
				Name:       resource.Name,
			})
			if err != nil {
				// do nothing if resource template not exist.
				if apierrors.IsNotFound(err) {
					continue
				}
				return false, err
			}
			if selector.Matches(labels.Set(rawObject.GetLabels())) {
				return false, nil
			}
		default:
			// can not reach here
		}
	}
	return true, nil
}

func (d *DependenciesDistributor) listAttachedBindings(bindingID, bindingNamespace, bindingName string) (res []*workv1alpha2.ResourceBinding, err error) {
	labelSet := generateBindingDependedLabels(bindingID, bindingNamespace, bindingName)
	selector := labels.SelectorFromSet(labelSet)
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

	bindingLabelKey := generateBindingDependedLabelKey(bindingNamespace, bindingName)

	var errs []error
	for index, binding := range attachedBindings {
		delete(attachedBindings[index].Labels, bindingLabelKey)
		updatedSnapshot := deleteBindingFromSnapshot(bindingNamespace, bindingName, attachedBindings[index].Spec.RequiredBy)
		attachedBindings[index].Spec.RequiredBy = updatedSnapshot
		attachedBindings[index].Spec.PreserveResourcesOnDeletion = nil
		if err := d.Client.Update(context.TODO(), attachedBindings[index]); err != nil {
			klog.Errorf("Failed to update binding(%s/%s): %v", binding.Namespace, binding.Name, err)
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (d *DependenciesDistributor) createOrUpdateAttachedBinding(attachedBinding *workv1alpha2.ResourceBinding) error {
	existBinding := &workv1alpha2.ResourceBinding{}
	bindingKey := client.ObjectKeyFromObject(attachedBinding)
	err := d.Client.Get(context.TODO(), bindingKey, existBinding)
	if err == nil {
		// If this binding exists and its owner is not the input object, return error and let garbage collector
		// delete this binding and try again later. See https://github.com/karmada-io/karmada/issues/6034.
		if ownerRef := metav1.GetControllerOfNoCopy(existBinding); ownerRef != nil && ownerRef.UID != attachedBinding.OwnerReferences[0].UID {
			return fmt.Errorf("failed to update resourceBinding(%s) due to different owner reference UID, will "+
				"try again later after binding is garbage collected, see https://github.com/karmada-io/karmada/issues/6034", bindingKey)
		}

		// If the spec.Placement is nil, this means that existBinding is generated by the dependency mechanism.
		// If the spec.Placement is not nil, then it must be generated by PropagationPolicy.
		if existBinding.Spec.Placement == nil {
			existBinding.Spec.ConflictResolution = attachedBinding.Spec.ConflictResolution
		}
		existBinding.Spec.RequiredBy = mergeBindingSnapshot(existBinding.Spec.RequiredBy, attachedBinding.Spec.RequiredBy)
		existBinding.Labels = util.DedupeAndMergeLabels(existBinding.Labels, attachedBinding.Labels)
		existBinding.Spec.Resource = attachedBinding.Spec.Resource
		existBinding.Spec.PreserveResourcesOnDeletion = attachedBinding.Spec.PreserveResourcesOnDeletion

		if err := d.Client.Update(context.TODO(), existBinding); err != nil {
			klog.Errorf("Failed to update resourceBinding(%s): %v", bindingKey, err)
			return err
		}
		return nil
	}

	if !apierrors.IsNotFound(err) {
		klog.Infof("Failed to get resourceBinding(%s): %v", bindingKey, err)
		return err
	}

	return d.Client.Create(context.TODO(), attachedBinding)
}

// Start runs the distributor, never stop until context canceled.
func (d *DependenciesDistributor) Start(ctx context.Context) error {
	klog.Infof("Starting dependencies distributor.")
	resourceWorkerOptions := util.Options{
		Name: "dependencies resource detector",
		KeyFunc: func(obj interface{}) (util.QueueKey, error) {
			key, err := keys.ClusterWideKeyFunc(obj)
			if err != nil {
				return nil, err
			}
			metaInfo, err := meta.Accessor(obj)
			if err != nil { // should not happen
				return nil, fmt.Errorf("object has no meta: %v", err)
			}
			return &LabelsKey{
				ClusterWideKey: key,
				Labels:         metaInfo.GetLabels(),
			}, nil
		},
		ReconcileFunc:      d.reconcileResourceTemplate,
		RateLimiterOptions: d.RateLimiterOptions,
	}
	d.eventHandler = fedinformer.NewHandlerOnEvents(d.OnAdd, d.OnUpdate, d.OnDelete)
	d.resourceProcessor = util.NewAsyncWorker(resourceWorkerOptions)
	d.resourceProcessor.Run(ctx, d.ConcurrentDependentResourceSyncs)
	<-ctx.Done()

	klog.Infof("Stopped as context canceled.")
	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (d *DependenciesDistributor) SetupWithManager(mgr controllerruntime.Manager) error {
	d.genericEvent = make(chan event.TypedGenericEvent[*workv1alpha2.ResourceBinding])
	return utilerrors.NewAggregate([]error{
		mgr.Add(d),
		controllerruntime.NewControllerManagedBy(mgr).
			Named(ControllerName).
			For(&workv1alpha2.ResourceBinding{}).
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

					return oldBindingObject.Spec.PropagateDeps || newBindingObject.Spec.PropagateDeps
				},
			}).
			WithOptions(controller.Options{
				RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](d.RateLimiterOptions),
			}).
			WatchesRawSource(source.Channel(d.genericEvent, &handler.TypedEnqueueRequestForObject[*workv1alpha2.ResourceBinding]{})).
			Complete(d),
	})
}

func generateBindingDependedLabels(bindingID, bindingNamespace, bindingName string) map[string]string {
	labelKey := generateBindingDependedLabelKey(bindingNamespace, bindingName)
	return map[string]string{labelKey: bindingID}
}

func generateBindingDependedLabelKey(bindingNamespace, bindingName string) string {
	return dependedByLabelKeyPrefix + names.GenerateBindingReferenceKey(bindingNamespace, bindingName)
}

func generateDependencyKey(kind, apiVersion, namespace string) string {
	if len(namespace) == 0 {
		return kind + "-" + apiVersion
	}

	return kind + "-" + apiVersion + "-" + namespace
}

func buildAttachedBinding(independentBinding *workv1alpha2.ResourceBinding, object *unstructured.Unstructured) *workv1alpha2.ResourceBinding {
	dependedLabels := generateBindingDependedLabels(independentBinding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel],
		independentBinding.Namespace, independentBinding.Name)

	var result []workv1alpha2.BindingSnapshot
	result = append(result, workv1alpha2.BindingSnapshot{
		Namespace: independentBinding.Namespace,
		Name:      independentBinding.Name,
		Clusters:  independentBinding.Spec.Clusters,
	})

	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.GenerateBindingName(object.GetKind(), object.GetName()),
			Namespace: independentBinding.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, object.GroupVersionKind()),
			},
			Labels:     dependedLabels,
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
			RequiredBy:                  result,
			PreserveResourcesOnDeletion: independentBinding.Spec.PreserveResourcesOnDeletion,
			ConflictResolution:          independentBinding.Spec.ConflictResolution,
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
