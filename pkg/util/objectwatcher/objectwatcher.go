package objectwatcher

import (
	"context"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ObjectWatcher manages operations for object dispatched to member clusters.
type ObjectWatcher interface {
	Create(clusterName string, desireObj *unstructured.Unstructured) error
	Update(clusterName string, desireObj, clusterObj *unstructured.Unstructured) error
	Delete(clusterName string, desireObj *unstructured.Unstructured) error
	NeedsUpdate(clusterName string, oldObj, currentObj *unstructured.Unstructured) bool
}

// ClientSetFunc is used to generate client set of member cluster
type ClientSetFunc func(c string, client client.Client) (*util.DynamicClusterClient, error)

type versionFunc func() (objectVersion string, err error)

type versionWithLock struct {
	lock    sync.RWMutex
	version string
}

type objectWatcherImpl struct {
	Lock                 sync.RWMutex
	RESTMapper           meta.RESTMapper
	KubeClientSet        client.Client
	VersionRecord        map[string]map[string]*versionWithLock
	ClusterClientSetFunc ClientSetFunc
	resourceInterpreter  resourceinterpreter.ResourceInterpreter
}

// NewObjectWatcher returns an instance of ObjectWatcher
func NewObjectWatcher(kubeClientSet client.Client, restMapper meta.RESTMapper, clusterClientSetFunc ClientSetFunc, interpreter resourceinterpreter.ResourceInterpreter) ObjectWatcher {
	return &objectWatcherImpl{
		KubeClientSet:        kubeClientSet,
		VersionRecord:        make(map[string]map[string]*versionWithLock),
		RESTMapper:           restMapper,
		ClusterClientSetFunc: clusterClientSetFunc,
		resourceInterpreter:  interpreter,
	}
}

func (o *objectWatcherImpl) Create(clusterName string, desireObj *unstructured.Unstructured) error {
	dynamicClusterClient, err := o.ClusterClientSetFunc(clusterName, o.KubeClientSet)
	if err != nil {
		klog.Errorf("Failed to build dynamic cluster client for cluster %s.", clusterName)
		return err
	}

	gvr, err := restmapper.GetGroupVersionResource(o.RESTMapper, desireObj.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to create resource(kind=%s, %s/%s) in cluster %s as mapping GVK to GVR failed: %v", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	clusterObj, err := dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Create(context.TODO(), desireObj, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Failed to create resource(kind=%s, %s/%s) in cluster %s, err is %v ", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	klog.Infof("Created resource(kind=%s, %s/%s) on cluster: %s", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)

	// record version
	return o.recordVersionWithVersionFunc(clusterObj, dynamicClusterClient.ClusterName, func() (string, error) { return lifted.ObjectVersion(clusterObj), nil })
}

func (o *objectWatcherImpl) retainClusterFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Pass the same ResourceVersion as in the cluster object for update operation, otherwise operation will fail.
	desired.SetResourceVersion(observed.GetResourceVersion())

	// Retain finalizers since they will typically be set by
	// controllers in a member cluster.  It is still possible to set the fields
	// via overrides.
	desired.SetFinalizers(observed.GetFinalizers())

	// Retain ownerReferences since they will typically be set by controllers in a member cluster.
	desired.SetOwnerReferences(observed.GetOwnerReferences())

	// Retain annotations since they will typically be set by controllers in a member cluster
	// and be set by user in karmada-controller-plane.
	util.RetainAnnotations(desired, observed)

	// Retain labels since they will typically be set by controllers in a member cluster
	// and be set by user in karmada-controller-plane.
	util.RetainLabels(desired, observed)

	if o.resourceInterpreter.HookEnabled(desired.GroupVersionKind(), configv1alpha1.InterpreterOperationRetain) {
		return o.resourceInterpreter.Retain(desired, observed)
	}

	return desired, nil
}

func (o *objectWatcherImpl) Update(clusterName string, desireObj, clusterObj *unstructured.Unstructured) error {
	updateAllowed := o.allowUpdate(clusterName, desireObj, clusterObj)
	if !updateAllowed {
		// The existing resource is not managed by Karmada, and no conflict resolution found, avoid updating the existing resource by default.
		return fmt.Errorf("resource(kind=%s, %s/%s) already exist in cluster %v and the %s strategy value is empty, karmada will not manage this resource",
			desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, workv1alpha2.ResourceConflictResolutionAnnotation,
		)
	}

	dynamicClusterClient, err := o.ClusterClientSetFunc(clusterName, o.KubeClientSet)
	if err != nil {
		klog.Errorf("Failed to build dynamic cluster client for cluster %s.", clusterName)
		return err
	}

	gvr, err := restmapper.GetGroupVersionResource(o.RESTMapper, desireObj.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to update resource(kind=%s, %s/%s) in cluster %s as mapping GVK to GVR failed: %v", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	var errMsg string
	var desireCopy, resource *unstructured.Unstructured
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		desireCopy = desireObj.DeepCopy()
		if err != nil {
			clusterObj, err = dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Get(context.TODO(), desireObj.GetName(), metav1.GetOptions{})
			if err != nil {
				errMsg = fmt.Sprintf("Failed to get resource(kind=%s, %s/%s) in cluster %s, err is %v ", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
				return err
			}
		}

		desireCopy, err = o.retainClusterFields(desireCopy, clusterObj)
		if err != nil {
			errMsg = fmt.Sprintf("Failed to retain fields for resource(kind=%s, %s/%s) in cluster %s: %v", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), clusterName, err)
			return err
		}

		versionFuncWithUpdate := func() (string, error) {
			resource, err = dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Update(context.TODO(), desireCopy, metav1.UpdateOptions{})
			if err != nil {
				return "", err
			}

			return lifted.ObjectVersion(resource), nil
		}
		err = o.recordVersionWithVersionFunc(desireCopy, clusterName, versionFuncWithUpdate)
		if err == nil {
			return nil
		}

		errMsg = fmt.Sprintf("Failed to update resource(kind=%s, %s/%s) in cluster %s, err is %v ", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	})

	if err != nil {
		klog.Errorf(errMsg)
		return err
	}

	klog.Infof("Updated resource(kind=%s, %s/%s) on cluster: %s", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)

	return nil
}

func (o *objectWatcherImpl) Delete(clusterName string, desireObj *unstructured.Unstructured) error {
	dynamicClusterClient, err := o.ClusterClientSetFunc(clusterName, o.KubeClientSet)
	if err != nil {
		klog.Errorf("Failed to build dynamic cluster client for cluster %s.", clusterName)
		return err
	}

	gvr, err := restmapper.GetGroupVersionResource(o.RESTMapper, desireObj.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to delete resource(kind=%s, %s/%s) in cluster %s as mapping GVK to GVR failed: %v", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	// Set deletion strategy to background explicitly even though it's the default strategy for most of the resources.
	// The reason for this is to fix the exception case that Kubernetes does on Job(batch/v1).
	// In kubernetes, the Job's default deletion strategy is "Orphan", that will cause the "Pods" created by "Job"
	// still exist after "Job" has been deleted.
	// Refer to https://github.com/karmada-io/karmada/issues/969 for more details.
	deleteBackground := metav1.DeletePropagationBackground
	deleteOption := metav1.DeleteOptions{
		PropagationPolicy: &deleteBackground,
	}

	err = dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Delete(context.TODO(), desireObj.GetName(), deleteOption)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Failed to delete resource %v in cluster %s, err is %v ", desireObj.GetName(), clusterName, err)
		return err
	}
	klog.Infof("Deleted resource(kind=%s, %s/%s) on cluster: %s", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)

	o.deleteVersionRecord(desireObj, dynamicClusterClient.ClusterName)

	return nil
}

func (o *objectWatcherImpl) genObjectKey(obj *unstructured.Unstructured) string {
	return obj.GroupVersionKind().String() + "/" + obj.GetNamespace() + "/" + obj.GetName()
}

// recordVersion will add or update resource version records with the version returned by versionFunc
func (o *objectWatcherImpl) recordVersionWithVersionFunc(obj *unstructured.Unstructured, clusterName string, fn versionFunc) error {
	objectKey := o.genObjectKey(obj)
	return o.addOrUpdateVersionRecordWithVersionFunc(clusterName, objectKey, fn)
}

// getVersionRecord will return the recorded version of given resource(if exist)
func (o *objectWatcherImpl) getVersionRecord(clusterName, resourceName string) (string, bool) {
	versionLock, exist := o.getVersionWithLockRecord(clusterName, resourceName)
	if !exist {
		return "", false
	}

	versionLock.lock.RLock()
	defer versionLock.lock.RUnlock()

	return versionLock.version, true
}

// getVersionRecordWithLock will return the recorded versionWithLock of given resource(if exist)
func (o *objectWatcherImpl) getVersionWithLockRecord(clusterName, resourceName string) (*versionWithLock, bool) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()

	versionLock, exist := o.VersionRecord[clusterName][resourceName]
	return versionLock, exist
}

// newVersionWithLockRecord will add new versionWithLock record of given resource
func (o *objectWatcherImpl) newVersionWithLockRecord(clusterName, resourceName string) *versionWithLock {
	o.Lock.Lock()
	defer o.Lock.Unlock()

	v, exist := o.VersionRecord[clusterName][resourceName]
	if exist {
		return v
	}

	v = &versionWithLock{}
	if o.VersionRecord[clusterName] == nil {
		o.VersionRecord[clusterName] = map[string]*versionWithLock{}
	}

	o.VersionRecord[clusterName][resourceName] = v

	return v
}

// addOrUpdateVersionRecordWithVersionFunc will add or update the recorded version of given resource with version returned by versionFunc
func (o *objectWatcherImpl) addOrUpdateVersionRecordWithVersionFunc(clusterName, resourceName string, fn versionFunc) error {
	versionLock, exist := o.getVersionWithLockRecord(clusterName, resourceName)

	if !exist {
		versionLock = o.newVersionWithLockRecord(clusterName, resourceName)
	}

	versionLock.lock.Lock()
	defer versionLock.lock.Unlock()

	version, err := fn()
	if err != nil {
		return err
	}

	versionLock.version = version

	return nil
}

// deleteVersionRecord will delete the recorded version of given resource
func (o *objectWatcherImpl) deleteVersionRecord(obj *unstructured.Unstructured, clusterName string) {
	objectKey := o.genObjectKey(obj)

	o.Lock.Lock()
	defer o.Lock.Unlock()
	delete(o.VersionRecord[clusterName], objectKey)
}

func (o *objectWatcherImpl) NeedsUpdate(clusterName string, oldObj, currentObj *unstructured.Unstructured) bool {
	// get resource version, and if no recorded version, that means the object hasn't been processed yet since last restart, it should be processed now.
	version, exist := o.getVersionRecord(clusterName, o.genObjectKey(oldObj))
	if !exist {
		return true
	}

	return lifted.ObjectNeedsUpdate(oldObj, currentObj, version)
}

func (o *objectWatcherImpl) allowUpdate(clusterName string, desiredObj, clusterObj *unstructured.Unstructured) bool {
	// If the existing resource is managed by Karmada, then the updating is allowed.
	if util.GetLabelValue(desiredObj.GetLabels(), workv1alpha1.WorkNameLabel) == util.GetLabelValue(clusterObj.GetLabels(), workv1alpha1.WorkNameLabel) &&
		util.GetLabelValue(desiredObj.GetLabels(), workv1alpha1.WorkNamespaceLabel) == util.GetLabelValue(clusterObj.GetLabels(), workv1alpha1.WorkNamespaceLabel) {
		return true
	}

	// This happens when promoting workload to the Karmada control plane
	conflictResolution := util.GetAnnotationValue(desiredObj.GetAnnotations(), workv1alpha2.ResourceConflictResolutionAnnotation)
	if conflictResolution == workv1alpha2.ResourceConflictResolutionOverwrite {
		return true
	}

	// The existing resource is not managed by Karmada, and no conflict resolution found, avoid updating the existing resource by default.
	klog.Warningf("resource(kind=%s, %s/%s) already exist in cluster %v and the %s strategy value is empty, karmada will not manage this resource",
		desiredObj.GetKind(), desiredObj.GetNamespace(), desiredObj.GetName(), clusterName, workv1alpha2.ResourceConflictResolutionAnnotation,
	)
	return false
}
