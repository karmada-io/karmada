package objectwatcher

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/crdexplorer"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	generationPrefix      = "gen:"
	resourceVersionPrefix = "rv:"
)

// ObjectWatcher manages operations for object dispatched to member clusters.
type ObjectWatcher interface {
	Create(clusterName string, desireObj *unstructured.Unstructured) error
	Update(clusterName string, desireObj, clusterObj *unstructured.Unstructured) error
	Delete(clusterName string, desireObj *unstructured.Unstructured) error
	NeedsUpdate(clusterName string, desiredObj, clusterObj *unstructured.Unstructured) (bool, error)
}

// ClientSetFunc is used to generate client set of member cluster
type ClientSetFunc func(c string, client client.Client) (*util.DynamicClusterClient, error)

type objectWatcherImpl struct {
	Lock                 sync.RWMutex
	RESTMapper           meta.RESTMapper
	KubeClientSet        client.Client
	VersionRecord        map[string]map[string]string
	ClusterClientSetFunc ClientSetFunc
	resourceExplorer     crdexplorer.CustomResourceExplorer
}

// NewObjectWatcher returns an instance of ObjectWatcher
func NewObjectWatcher(kubeClientSet client.Client, restMapper meta.RESTMapper, clusterClientSetFunc ClientSetFunc, explorer crdexplorer.CustomResourceExplorer) ObjectWatcher {
	return &objectWatcherImpl{
		KubeClientSet:        kubeClientSet,
		VersionRecord:        make(map[string]map[string]string),
		RESTMapper:           restMapper,
		ClusterClientSetFunc: clusterClientSetFunc,
		resourceExplorer:     explorer,
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
		klog.Errorf("Failed to create resource(%s/%s) in cluster %s as mapping GVK to GVR failed: %v", desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	// Karmada will adopt creating resource due to an existing resource in member cluster, because we don't want to force update or delete the resource created by users.
	// users should resolve the conflict in person.
	clusterObj, err := dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Create(context.TODO(), desireObj, metav1.CreateOptions{})
	if err != nil {
		// The 'IsAlreadyExists' conflict may happen in following known scenarios:
		// - 1. In a reconcile process, the execution controller successfully applied resource to member cluster but failed to update the work conditions(Applied=True),
		//   when reconcile again, the controller will try to apply(by create) the resource again.
		// - 2. The resource already exist in the member cluster but it's not created by karmada.
		if apierrors.IsAlreadyExists(err) {
			existObj, err := dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Get(context.TODO(), desireObj.GetName(), metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get exist resource(kind=%s, %s/%s) in cluster %v: %v", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
			}

			// Avoid updating resources that not managed by karmada.
			if util.GetLabelValue(desireObj.GetLabels(), workv1alpha1.WorkNameLabel) != util.GetLabelValue(existObj.GetLabels(), workv1alpha1.WorkNameLabel) {
				return fmt.Errorf("resource(kind=%s, %s/%s) already exist in cluster %v but not managed by karamda", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)
			}

			return o.Update(clusterName, desireObj, existObj)
		}
		klog.Errorf("Failed to create resource(kind=%s, %s/%s) in cluster %s, err is %v ", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}
	klog.Infof("Created resource(kind=%s, %s/%s) on cluster: %s", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)

	// record version
	o.recordVersion(clusterObj, dynamicClusterClient.ClusterName)
	return nil
}

func (o *objectWatcherImpl) retainClusterFields(desired, observed *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Pass the same ResourceVersion as in the cluster object for update operation, otherwise operation will fail.
	desired.SetResourceVersion(observed.GetResourceVersion())

	// Retain finalizers since they will typically be set by
	// controllers in a member cluster.  It is still possible to set the fields
	// via overrides.
	desired.SetFinalizers(observed.GetFinalizers())
	// Merge annotations since they will typically be set by controllers in a member cluster
	// and be set by user in karmada-controller-plane.
	util.MergeAnnotations(desired, observed)

	if o.resourceExplorer.HookEnabled(desired, configv1alpha1.ExploreRetaining) {
		return o.resourceExplorer.Retain(desired, observed)
	}

	return desired, nil
}

func (o *objectWatcherImpl) Update(clusterName string, desireObj, clusterObj *unstructured.Unstructured) error {
	dynamicClusterClient, err := o.ClusterClientSetFunc(clusterName, o.KubeClientSet)
	if err != nil {
		klog.Errorf("Failed to build dynamic cluster client for cluster %s.", clusterName)
		return err
	}

	gvr, err := restmapper.GetGroupVersionResource(o.RESTMapper, desireObj.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to update resource(%s/%s) in cluster %s as mapping GVK to GVR failed: %v", desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	desireObj, err = o.retainClusterFields(desireObj, clusterObj)
	if err != nil {
		klog.Errorf("Failed to retain fields for resource(kind=%s, %s/%s) in cluster %s: %v", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	resource, err := dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(desireObj.GetNamespace()).Update(context.TODO(), desireObj, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update resource(kind=%s, %s/%s) in cluster %s, err is %v ", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
		return err
	}

	klog.Infof("Updated resource(kind=%s, %s/%s) on cluster: %s", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)

	// record version
	o.recordVersion(resource, clusterName)
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
		klog.Errorf("Failed to delete resource(%s/%s) in cluster %s as mapping GVK to GVR failed: %v", desireObj.GetNamespace(), desireObj.GetName(), clusterName, err)
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
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		klog.Errorf("Failed to delete resource %v in cluster %s, err is %v ", desireObj.GetName(), clusterName, err)
		return err
	}
	klog.Infof("Deleted resource(kind=%s, %s/%s) on cluster: %s", desireObj.GetKind(), desireObj.GetNamespace(), desireObj.GetName(), clusterName)

	objectKey := o.genObjectKey(desireObj)
	o.deleteVersionRecord(dynamicClusterClient.ClusterName, objectKey)

	return nil
}

func (o *objectWatcherImpl) genObjectKey(obj *unstructured.Unstructured) string {
	return obj.GroupVersionKind().String() + "/" + obj.GetNamespace() + "/" + obj.GetName()
}

// recordVersion will add or update resource version records
func (o *objectWatcherImpl) recordVersion(clusterObj *unstructured.Unstructured, clusterName string) {
	objVersion := objectVersion(clusterObj)
	objectKey := o.genObjectKey(clusterObj)
	if o.isClusterVersionRecordExist(clusterName) {
		o.updateVersionRecord(clusterName, objectKey, objVersion)
	} else {
		o.addVersionRecord(clusterName, objectKey, objVersion)
	}
}

// isClusterVersionRecordExist checks if the version record map of given member cluster exist
func (o *objectWatcherImpl) isClusterVersionRecordExist(clusterName string) bool {
	o.Lock.RLock()
	defer o.Lock.RUnlock()

	_, exist := o.VersionRecord[clusterName]

	return exist
}

// getVersionRecord will return the recorded version of given resource(if exist)
func (o *objectWatcherImpl) getVersionRecord(clusterName, resourceName string) (string, bool) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()

	version, exist := o.VersionRecord[clusterName][resourceName]
	return version, exist
}

// addVersionRecord will add new version record of given resource
func (o *objectWatcherImpl) addVersionRecord(clusterName, resourceName, version string) {
	o.Lock.Lock()
	defer o.Lock.Unlock()
	o.VersionRecord[clusterName] = map[string]string{resourceName: version}
}

// updateVersionRecord will update the recorded version of given resource
func (o *objectWatcherImpl) updateVersionRecord(clusterName, resourceName, version string) {
	o.Lock.Lock()
	defer o.Lock.Unlock()
	o.VersionRecord[clusterName][resourceName] = version
}

// deleteVersionRecord will delete the recorded version of given resource
func (o *objectWatcherImpl) deleteVersionRecord(clusterName, resourceName string) {
	o.Lock.Lock()
	defer o.Lock.Unlock()
	delete(o.VersionRecord[clusterName], resourceName)
}

func (o *objectWatcherImpl) NeedsUpdate(clusterName string, desiredObj, clusterObj *unstructured.Unstructured) (bool, error) {
	// get resource version
	version, exist := o.getVersionRecord(clusterName, desiredObj.GroupVersionKind().String()+"/"+desiredObj.GetNamespace()+"/"+desiredObj.GetName())
	if !exist {
		klog.Errorf("Failed to update resource %v/%v in cluster %s for the version record does not exist", desiredObj.GetNamespace(), desiredObj.GetName(), clusterName)
		return false, fmt.Errorf("failed to update resource %v/%v in cluster %s for the version record does not exist", desiredObj.GetNamespace(), desiredObj.GetName(), clusterName)
	}

	return objectNeedsUpdate(desiredObj, clusterObj, version), nil
}

/*
This code is lifted from the kubefed codebase. It's a list of functions to determine whether the provided cluster
object needs to be updated according to the desired object and the recorded version.
For reference: https://github.com/kubernetes-sigs/kubefed/blob/master/pkg/controller/util/propagatedversion.go#L30-L59
*/

// objectVersion retrieves the field type-prefixed value used for
// determining currency of the given cluster object.
func objectVersion(clusterObj *unstructured.Unstructured) string {
	generation := clusterObj.GetGeneration()
	if generation != 0 {
		return fmt.Sprintf("%s%d", generationPrefix, generation)
	}
	return fmt.Sprintf("%s%s", resourceVersionPrefix, clusterObj.GetResourceVersion())
}

// objectNeedsUpdate determines whether the 2 objects provided cluster
// object needs to be updated according to the desired object and the
// recorded version.
func objectNeedsUpdate(desiredObj, clusterObj *unstructured.Unstructured, recordedVersion string) bool {
	targetVersion := objectVersion(clusterObj)

	if recordedVersion != targetVersion {
		return true
	}

	// If versions match and the version is sourced from the
	// generation field, a further check of metadata equivalency is
	// required.
	return strings.HasPrefix(targetVersion, generationPrefix) && !objectMetaObjEquivalent(desiredObj, clusterObj)
}

// objectMetaObjEquivalent checks if cluster-independent, user provided data in two given ObjectMeta are equal. If in
// the future the ObjectMeta structure is expanded then any field that is not populated
// by the api server should be included here.
func objectMetaObjEquivalent(a, b metav1.Object) bool {
	if a.GetName() != b.GetName() {
		return false
	}
	if a.GetNamespace() != b.GetNamespace() {
		return false
	}
	aLabels := a.GetLabels()
	bLabels := b.GetLabels()
	if !reflect.DeepEqual(aLabels, bLabels) && (len(aLabels) != 0 || len(bLabels) != 0) {
		return false
	}
	return true
}
