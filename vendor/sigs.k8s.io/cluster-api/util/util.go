/*
Copyright 2017 The Kubernetes Authors.

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

// Package util implements utilities.
package util

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sversion "k8s.io/apimachinery/pkg/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/contract"
)

const (
	// CharSet defines the alphanumeric set for random string generation.
	CharSet = "0123456789abcdefghijklmnopqrstuvwxyz"
)

var (
	rnd = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

	// ErrNoCluster is returned when the cluster
	// label could not be found on the object passed in.
	ErrNoCluster = fmt.Errorf("no %q label present", clusterv1.ClusterNameLabel)

	// ErrUnstructuredFieldNotFound determines that a field
	// in an unstructured object could not be found.
	ErrUnstructuredFieldNotFound = fmt.Errorf("field not found")
)

// RandomString returns a random alphanumeric string.
func RandomString(n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = CharSet[rnd.Intn(len(CharSet))]
	}
	return string(result)
}

// Ordinalize takes an int and returns the ordinalized version of it.
// Eg. 1 --> 1st, 103 --> 103rd.
func Ordinalize(n int) string {
	m := map[int]string{
		0: "th",
		1: "st",
		2: "nd",
		3: "rd",
		4: "th",
		5: "th",
		6: "th",
		7: "th",
		8: "th",
		9: "th",
	}

	an := int(math.Abs(float64(n)))
	if an < 10 {
		return fmt.Sprintf("%d%s", n, m[an])
	}
	return fmt.Sprintf("%d%s", n, m[an%10])
}

// IsExternalManagedControlPlane returns a bool indicating whether the control plane referenced
// in the passed Unstructured resource is an externally managed control plane such as AKS, EKS, GKE, etc.
func IsExternalManagedControlPlane(controlPlane *unstructured.Unstructured) bool {
	managed, found, err := unstructured.NestedBool(controlPlane.Object, "status", "externalManagedControlPlane")
	if err != nil || !found {
		return false
	}
	return managed
}

// GetMachineIfExists gets a machine from the API server if it exists.
func GetMachineIfExists(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.Machine, error) {
	if c == nil {
		// Being called before k8s is setup as part of control plane VM creation
		return nil, nil
	}

	// Machines are identified by name
	machine := &clusterv1.Machine{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return machine, nil
}

// IsControlPlaneMachine checks machine is a control plane node.
func IsControlPlaneMachine(machine *clusterv1.Machine) bool {
	_, ok := machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabel]
	return ok
}

// IsNodeReady returns true if a node is ready.
func IsNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// GetClusterFromMetadata returns the Cluster object (if present) using the object metadata.
func GetClusterFromMetadata(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1.Cluster, error) {
	if obj.Labels[clusterv1.ClusterNameLabel] == "" {
		return nil, errors.WithStack(ErrNoCluster)
	}
	return GetClusterByName(ctx, c, obj.Namespace, obj.Labels[clusterv1.ClusterNameLabel])
}

// GetOwnerCluster returns the Cluster object owning the current resource.
func GetOwnerCluster(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1.Cluster, error) {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind != "Cluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return GetClusterByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetClusterByName finds and return a Cluster object using the specified params.
func GetClusterByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.Cluster, error) {
	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, cluster); err != nil {
		return nil, errors.Wrapf(err, "failed to get Cluster/%s", name)
	}

	return cluster, nil
}

// ObjectKey returns client.ObjectKey for the object.
func ObjectKey(object metav1.Object) client.ObjectKey {
	return client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}
}

// ClusterToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Cluster events and returns reconciliation requests for an infrastructure provider object.
func ClusterToInfrastructureMapFunc(ctx context.Context, gvk schema.GroupVersionKind, c client.Client, providerCluster client.Object) handler.MapFunc {
	log := ctrl.LoggerFrom(ctx)
	return func(o client.Object) []reconcile.Request {
		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		// Return early if the InfrastructureRef is nil.
		if cluster.Spec.InfrastructureRef == nil {
			return nil
		}
		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect.
		infraGK := cluster.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}
		providerCluster := providerCluster.DeepCopyObject().(client.Object)
		key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.InfrastructureRef.Name}

		if err := c.Get(ctx, key, providerCluster); err != nil {
			log.V(4).Error(err, fmt.Sprintf("Failed to get %T", providerCluster))
			return nil
		}

		if annotations.IsExternallyManaged(providerCluster) {
			log.V(4).Info(fmt.Sprintf("%T is externally managed, skipping mapping", providerCluster))
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: cluster.Namespace,
					Name:      cluster.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// GetOwnerMachine returns the Machine object owning the current resource.
func GetOwnerMachine(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1.Machine, error) {
	for _, ref := range obj.GetOwnerReferences() {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if ref.Kind == "Machine" && gv.Group == clusterv1.GroupVersion.Group {
			return GetMachineByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetMachineByName finds and return a Machine object using the specified params.
func GetMachineByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.Machine, error) {
	m := &clusterv1.Machine{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// MachineToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Machine events and returns reconciliation requests for an infrastructure provider object.
func MachineToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.MapFunc {
	return func(o client.Object) []reconcile.Request {
		m, ok := o.(*clusterv1.Machine)
		if !ok {
			return nil
		}

		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect.
		infraGK := m.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// HasOwnerRef returns true if the OwnerReference is already in the slice. It matches based on Group, Kind and Name.
func HasOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) bool {
	return indexOwnerRef(ownerReferences, ref) > -1
}

// EnsureOwnerRef makes sure the slice contains the OwnerReference.
// Note: EnsureOwnerRef will update the version of the OwnerReference fi it exists with a different version. It will also update the UID.
func EnsureOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) []metav1.OwnerReference {
	idx := indexOwnerRef(ownerReferences, ref)
	if idx == -1 {
		return append(ownerReferences, ref)
	}
	ownerReferences[idx] = ref
	return ownerReferences
}

// ReplaceOwnerRef re-parents an object from one OwnerReference to another
// It compares strictly based on UID to avoid reparenting across an intentional deletion: if an object is deleted
// and re-created with the same name and namespace, the only way to tell there was an in-progress deletion
// is by comparing the UIDs.
func ReplaceOwnerRef(ownerReferences []metav1.OwnerReference, source metav1.Object, target metav1.OwnerReference) []metav1.OwnerReference {
	fi := -1
	for index, r := range ownerReferences {
		if r.UID == source.GetUID() {
			fi = index
			ownerReferences[index] = target
			break
		}
	}
	if fi < 0 {
		ownerReferences = append(ownerReferences, target)
	}
	return ownerReferences
}

// RemoveOwnerRef returns the slice of owner references after removing the supplied owner ref.
// Note: RemoveOwnerRef ignores apiVersion and UID. It will remove the passed ownerReference where it matches Name, Group and Kind.
func RemoveOwnerRef(ownerReferences []metav1.OwnerReference, inputRef metav1.OwnerReference) []metav1.OwnerReference {
	if index := indexOwnerRef(ownerReferences, inputRef); index != -1 {
		return append(ownerReferences[:index], ownerReferences[index+1:]...)
	}
	return ownerReferences
}

// indexOwnerRef returns the index of the owner reference in the slice if found, or -1.
func indexOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) int {
	for index, r := range ownerReferences {
		if referSameObject(r, ref) {
			return index
		}
	}
	return -1
}

// IsOwnedByObject returns true if any of the owner references point to the given target.
// It matches the object based on the Group, Kind and Name.
func IsOwnedByObject(obj metav1.Object, target client.Object) bool {
	for _, ref := range obj.GetOwnerReferences() {
		ref := ref
		if refersTo(&ref, target) {
			return true
		}
	}
	return false
}

// IsControlledBy differs from metav1.IsControlledBy. This function matches on Group, Kind and Name. The metav1.IsControlledBy function matches on UID only.
func IsControlledBy(obj metav1.Object, owner client.Object) bool {
	controllerRef := metav1.GetControllerOfNoCopy(obj)
	if controllerRef == nil {
		return false
	}
	return refersTo(controllerRef, owner)
}

// Returns true if a and b point to the same object based on Group, Kind and Name.
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}

// Returns true if ref refers to obj based on Group, Kind and Name.
func refersTo(ref *metav1.OwnerReference, obj client.Object) bool {
	refGv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return false
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	return refGv.Group == gvk.Group && ref.Kind == gvk.Kind && ref.Name == obj.GetName()
}

// UnstructuredUnmarshalField is a wrapper around json and unstructured objects to decode and copy a specific field
// value into an object.
func UnstructuredUnmarshalField(obj *unstructured.Unstructured, v interface{}, fields ...string) error {
	if obj == nil || obj.Object == nil {
		return errors.Errorf("failed to unmarshal unstructured object: object is nil")
	}

	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, fields...)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve field %q from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	if !found || value == nil {
		return ErrUnstructuredFieldNotFound
	}
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return errors.Wrapf(err, "failed to json-encode field %q value from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	if err := json.Unmarshal(valueBytes, v); err != nil {
		return errors.Wrapf(err, "failed to json-decode field %q value from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	return nil
}

// HasOwner checks if any of the references in the passed list match the given group from apiVersion and one of the given kinds.
func HasOwner(refList []metav1.OwnerReference, apiVersion string, kinds []string) bool {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false
	}

	kindMap := make(map[string]bool)
	for _, kind := range kinds {
		kindMap[kind] = true
	}

	for _, mr := range refList {
		mrGroupVersion, err := schema.ParseGroupVersion(mr.APIVersion)
		if err != nil {
			return false
		}

		if mrGroupVersion.Group == gv.Group && kindMap[mr.Kind] {
			return true
		}
	}

	return false
}

// GetGVKMetadata retrieves a CustomResourceDefinition metadata from the API server using partial object metadata.
//
// This function is greatly more efficient than GetCRDWithContract and should be preferred in most cases.
func GetGVKMetadata(ctx context.Context, c client.Client, gvk schema.GroupVersionKind) (*metav1.PartialObjectMetadata, error) {
	meta := &metav1.PartialObjectMetadata{}
	meta.SetName(contract.CalculateCRDName(gvk.Group, gvk.Kind))
	meta.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	if err := c.Get(ctx, client.ObjectKeyFromObject(meta), meta); err != nil {
		return meta, errors.Wrap(err, "failed to retrieve metadata from GVK resource")
	}
	return meta, nil
}

// KubeAwareAPIVersions is a sortable slice of kube-like version strings.
//
// Kube-like version strings are starting with a v, followed by a major version,
// optional "alpha" or "beta" strings followed by a minor version (e.g. v1, v2beta1).
// Versions will be sorted based on GA/alpha/beta first and then major and minor
// versions. e.g. v2, v1, v1beta2, v1beta1, v1alpha1.
type KubeAwareAPIVersions []string

func (k KubeAwareAPIVersions) Len() int      { return len(k) }
func (k KubeAwareAPIVersions) Swap(i, j int) { k[i], k[j] = k[j], k[i] }
func (k KubeAwareAPIVersions) Less(i, j int) bool {
	return k8sversion.CompareKubeAwareVersionStrings(k[i], k[j]) < 0
}

// ClusterToObjectsMapper returns a mapper function that gets a cluster and lists all objects for the object passed in
// and returns a list of requests.
// NB: The objects are required to have `clusterv1.ClusterNameLabel` applied.
func ClusterToObjectsMapper(c client.Client, ro client.ObjectList, scheme *runtime.Scheme) (handler.MapFunc, error) {
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return nil, err
	}

	isNamespaced, err := isAPINamespaced(gvk, c.RESTMapper())
	if err != nil {
		return nil, err
	}

	return func(o client.Object) []ctrl.Request {
		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		listOpts := []client.ListOption{
			client.MatchingLabels{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
		}

		if isNamespaced {
			listOpts = append(listOpts, client.InNamespace(cluster.Namespace))
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		if err := c.List(context.TODO(), list, listOpts...); err != nil {
			return nil
		}

		results := []ctrl.Request{}
		for _, obj := range list.Items {
			results = append(results, ctrl.Request{
				NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()},
			})
		}
		return results
	}, nil
}

// isAPINamespaced detects if a GroupVersionKind is namespaced.
func isAPINamespaced(gk schema.GroupVersionKind, restmapper meta.RESTMapper) (bool, error) {
	restMapping, err := restmapper.RESTMapping(schema.GroupKind{Group: gk.Group, Kind: gk.Kind})
	if err != nil {
		return false, fmt.Errorf("failed to get restmapping: %w", err)
	}

	switch restMapping.Scope.Name() {
	case "":
		return false, errors.New("Scope cannot be identified. Empty scope returned")
	case meta.RESTScopeNameRoot:
		return false, nil
	default:
		return true, nil
	}
}

// ObjectReferenceToUnstructured converts an object reference to an unstructured object.
func ObjectReferenceToUnstructured(in corev1.ObjectReference) *unstructured.Unstructured {
	out := &unstructured.Unstructured{}
	out.SetKind(in.Kind)
	out.SetAPIVersion(in.APIVersion)
	out.SetNamespace(in.Namespace)
	out.SetName(in.Name)
	return out
}

// IsSupportedVersionSkew will return true if a and b are no more than one minor version off from each other.
func IsSupportedVersionSkew(a, b semver.Version) bool {
	if a.Major != b.Major {
		return false
	}
	if a.Minor > b.Minor {
		return a.Minor-b.Minor == 1
	}
	return b.Minor-a.Minor <= 1
}

// LowestNonZeroResult compares two reconciliation results
// and returns the one with lowest requeue time.
func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

// LowestNonZeroInt32 returns the lowest non-zero value of the two provided values.
func LowestNonZeroInt32(i, j int32) int32 {
	if i == 0 {
		return j
	}
	if j == 0 {
		return i
	}
	if i < j {
		return i
	}
	return j
}

// IsNil returns an error if the passed interface is equal to nil or if it has an interface value of nil.
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Slice, reflect.Interface, reflect.UnsafePointer, reflect.Func:
		return reflect.ValueOf(i).IsValid() && reflect.ValueOf(i).IsNil()
	}
	return false
}
