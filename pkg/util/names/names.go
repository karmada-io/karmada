package names

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	hashutil "github.com/karmada-io/karmada/pkg/util/hash"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

const (
	// KubernetesReservedNSPrefix is the prefix of namespace which reserved by Kubernetes system, such as:
	// - kube-system
	// - kube-public
	// - kube-node-lease
	KubernetesReservedNSPrefix = "kube-"
	// NamespaceKarmadaSystem is reserved namespace
	NamespaceKarmadaSystem = "karmada-system"
	// NamespaceKarmadaCluster is reserved namespace
	NamespaceKarmadaCluster = "karmada-cluster"
	// NamespaceDefault is reserved namespace
	NamespaceDefault = "default"
)

// ExecutionSpacePrefix is the prefix of execution space
const ExecutionSpacePrefix = "karmada-es-"

// endpointSlicePrefix is the prefix of collected EndpointSlice from member clusters.
const endpointSlicePrefix = "imported"

// endpointSlicePrefix is the prefix of service derived from ServiceImport.
const derivedServicePrefix = "derived"

// estimatorServicePrefix is the prefix of scheduler estimator service name.
const estimatorServicePrefix = "karmada-scheduler-estimator"

// GenerateExecutionSpaceName generates execution space name for the given member cluster
func GenerateExecutionSpaceName(clusterName string) string {
	return ExecutionSpacePrefix + clusterName
}

// GetClusterName returns member cluster name for the given execution space
func GetClusterName(executionSpaceName string) (string, error) {
	if !strings.HasPrefix(executionSpaceName, ExecutionSpacePrefix) {
		return "", fmt.Errorf("the execution space name is in wrong format")
	}
	return strings.TrimPrefix(executionSpaceName, ExecutionSpacePrefix), nil
}

// GenerateBindingName will generate binding name for the template.
func GenerateBindingName(ctx context.Context, karmadaClient client.Client, dynamicClient dynamic.Interface,
	template *unstructured.Unstructured, restMapper meta.RESTMapper) (string, error) {
	basicName := GenerateBasicBindingName(template.GetKind(), template.GetName())
	namespace := template.GetNamespace()

	var existingBinding client.Object
	var bindingNameAnnotation string
	if len(namespace) == 0 {
		existingBinding = &workv1alpha2.ClusterResourceBinding{}
		bindingNameAnnotation = workv1alpha2.ClusterResourceBindingAnnotationKey
	} else {
		existingBinding = &workv1alpha2.ResourceBinding{}
		bindingNameAnnotation = workv1alpha2.ResourceBindingNameAnnotationKey
	}

	bindingName := template.GetAnnotations()[bindingNameAnnotation]
	if len(bindingName) == 0 {
		bindingName = basicName
	}

	// check name collision
	err := karmadaClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: bindingName}, existingBinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// it's not created yet, no name collision
			return bindingName, nil
		}
		return "", err
	}

	if !BindingNameCollision(existingBinding, template) {
		return bindingName, nil
	}

	// name collision!
	t := template.DeepCopy()
	annotations := t.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	// append a random suffix to the basic binding name
	bindingName = basicName + "-" + rand.String(5)
	// record the binding name in template annotations
	annotations[bindingNameAnnotation] = bindingName
	t.SetAnnotations(annotations)
	gvr, err := restmapper.GetGroupVersionResource(restMapper, t.GroupVersionKind())
	if err != nil {
		return "", err
	}
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, t, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}
	return bindingName, nil
}

// GenerateBasicBindingName will generate binding name by kind and name
func GenerateBasicBindingName(kind, name string) string {
	// The name of resources, like 'Role'/'ClusterRole'/'RoleBinding'/'ClusterRoleBinding',
	// may contain symbols(like ':') that are not allowed by CRD resources which require the
	// name can be used as a DNS subdomain name. So, we need to replace it.
	// These resources may also allow for other characters(like '&','$') that are not allowed
	// by CRD resources, we only handle the most common ones now for performance concerns.
	// For more information about the DNS subdomain name, please refer to
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names.
	if strings.Contains(name, ":") {
		name = strings.ReplaceAll(name, ":", ".")
	}

	return strings.ToLower(name + "-" + kind)
}

// GenerateBindingReferenceKey will generate the key of binding object with the hash of its namespace and name.
func GenerateBindingReferenceKey(namespace, name string) string {
	var bindingName string
	if len(namespace) > 0 {
		bindingName = namespace + "/" + name
	} else {
		bindingName = name
	}
	hash := fnv.New32a()
	hashutil.DeepHashObject(hash, bindingName)
	return rand.SafeEncodeString(fmt.Sprint(hash.Sum32()))
}

// GenerateWorkName generates work name for the workload.
func GenerateWorkName(ctx context.Context, karmadaClient client.Client, dynamicClient dynamic.Interface,
	workNamespace string, workload, owner *unstructured.Unstructured, restMapper meta.RESTMapper) (string, error) {
	basicName := GenerateBasicWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace())

	// key: work namespace, value: work name
	collisionWorks := make(map[string]string)
	_ = json.Unmarshal([]byte(owner.GetAnnotations()[workv1alpha2.WorkNamesAnnotationKey]), &collisionWorks)

	workName := collisionWorks[workNamespace]
	if len(workName) == 0 {
		workName = basicName
	}

	// check name collision
	existingWork := &workv1alpha1.Work{}
	err := karmadaClient.Get(ctx, types.NamespacedName{Namespace: workNamespace, Name: workName}, existingWork)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// it's not created yet, no hash collision
			return workName, nil
		}
		return "", err
	}

	if !WorkNameCollision(existingWork, workload) {
		return workName, nil
	}

	// name collision!
	// append a random suffix to the basic work name
	workName = basicName + "-" + rand.String(5)
	// record the work name in owner annotations
	collisionWorks[workNamespace] = workName
	w, err := json.Marshal(&collisionWorks)
	if err != nil {
		return "", err
	}
	owner = owner.DeepCopy()
	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[workv1alpha2.WorkNamesAnnotationKey] = string(w)
	owner.SetAnnotations(annotations)

	gvr, err := restmapper.GetGroupVersionResource(restMapper, owner.GroupVersionKind())
	if err != nil {
		return "", err
	}
	_, err = dynamicClient.Resource(gvr).Namespace(owner.GetNamespace()).Update(ctx, owner, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}
	return workName, nil
}

// GenerateBasicWorkName will generate work name by its name and the hash of its namespace, kind and name.
func GenerateBasicWorkName(kind, name, namespace string) string {
	// The name of resources, like 'Role'/'ClusterRole'/'RoleBinding'/'ClusterRoleBinding',
	// may contain symbols(like ':' or uppercase upper case) that are not allowed by CRD resources which require the
	// name can be used as a DNS subdomain name. So, we need to replace it.
	// These resources may also allow for other characters(like '&','$') that are not allowed
	// by CRD resources, we only handle the most common ones now for performance concerns.
	// For more information about the DNS subdomain name, please refer to
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names.
	if strings.Contains(name, ":") {
		name = strings.ReplaceAll(name, ":", ".")
	}
	name = strings.ToLower(name)

	var workName string
	if len(namespace) == 0 {
		workName = strings.ToLower(name + "-" + kind)
	} else {
		workName = strings.ToLower(namespace + "-" + name + "-" + kind)
	}
	hash := fnv.New32a()
	hashutil.DeepHashObject(hash, workName)
	return fmt.Sprintf("%s-%s", name, rand.SafeEncodeString(fmt.Sprint(hash.Sum32())))
}

// BindingNameCollision checks if the binding is built for the object
func BindingNameCollision(existingBinding client.Object, object metav1.Object) bool {
	switch existingBinding := existingBinding.(type) {
	case *workv1alpha2.ResourceBinding:
		return existingBinding.Spec.Resource.UID != object.GetUID()
	case *workv1alpha2.ClusterResourceBinding:
		return existingBinding.Spec.Resource.UID != object.GetUID()
	default:
		// should not happen
		return false
	}
}

// WorkNameCollision checks if the work is built for the object
func WorkNameCollision(existingWork *workv1alpha1.Work, object metav1.Object) bool {
	manifest := getWorkManifest(existingWork)
	if manifest == nil || manifest.GetAnnotations() == nil {
		// should not happen
		return false
	}
	return string(object.GetUID()) != manifest.GetAnnotations()[workv1alpha2.ResourceTemplateUIDAnnotation]
}

func getWorkManifest(work *workv1alpha1.Work) *unstructured.Unstructured {
	if len(work.Spec.Workload.Manifests) != 1 {
		// should not happen
		return nil
	}

	workload := &unstructured.Unstructured{}
	if err := workload.UnmarshalJSON(work.Spec.Workload.Manifests[0].Raw); err != nil {
		// should not happen
		return nil
	}
	return workload
}

// GenerateServiceAccountName generates the name of a ServiceAccount.
func GenerateServiceAccountName(clusterName string) string {
	return fmt.Sprintf("%s-%s", "karmada", clusterName)
}

// GenerateRoleName generates the name of a Role or ClusterRole.
func GenerateRoleName(serviceAccountName string) string {
	return fmt.Sprintf("karmada-controller-manager:%s", serviceAccountName)
}

// GenerateEndpointSliceName generates the name of collected EndpointSlice.
func GenerateEndpointSliceName(endpointSliceName string, cluster string) string {
	return fmt.Sprintf("%s-%s-%s", endpointSlicePrefix, cluster, endpointSliceName)
}

// GenerateDerivedServiceName generates the service name derived from ServiceImport.
func GenerateDerivedServiceName(serviceName string) string {
	return fmt.Sprintf("%s-%s", derivedServicePrefix, serviceName)
}

// GenerateEstimatorServiceName generates the gRPC scheduler estimator service name which belongs to a cluster.
func GenerateEstimatorServiceName(estimatorServicePrefix, clusterName string) string {
	return fmt.Sprintf("%s-%s", estimatorServicePrefix, clusterName)
}

// GenerateEstimatorDeploymentName generates the gRPC scheduler estimator deployment name which belongs to a cluster.
func GenerateEstimatorDeploymentName(clusterName string) string {
	return fmt.Sprintf("%s-%s", estimatorServicePrefix, clusterName)
}

// IsReservedNamespace return whether it is a reserved namespace
func IsReservedNamespace(namespace string) bool {
	return namespace == NamespaceKarmadaSystem ||
		namespace == NamespaceKarmadaCluster ||
		strings.HasPrefix(namespace, ExecutionSpacePrefix) ||
		strings.HasPrefix(namespace, KubernetesReservedNSPrefix)
}

// GenerateImpersonationSecretName generates the secret name of impersonation secret.
func GenerateImpersonationSecretName(clusterName string) string {
	return fmt.Sprintf("%s-impersonator", clusterName)
}

// GeneratePolicyName generates the propagationPolicy name
func GeneratePolicyName(namespace, name, gvk string) string {
	hash := fnv.New32a()
	hashutil.DeepHashObject(hash, namespace+gvk)

	// The name of resources, like 'Role'/'ClusterRole'/'RoleBinding'/'ClusterRoleBinding',
	// may contain symbols(like ':') that are not allowed by CRD resources which require the
	// name can be used as a DNS subdomain name. So, we need to replace it.
	// These resources may also allow for other characters(like '&','$') that are not allowed
	// by CRD resources, we only handle the most common ones now for performance concerns.
	// For more information about the DNS subdomain name, please refer to
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names.
	if strings.Contains(name, ":") {
		name = strings.ReplaceAll(name, ":", ".")
	}
	return strings.ToLower(fmt.Sprintf("%s-%s", name, rand.SafeEncodeString(fmt.Sprint(hash.Sum32()))))
}
