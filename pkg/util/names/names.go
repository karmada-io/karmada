package names

import (
	"fmt"
	"hash/fnv"
	"strings"

	"k8s.io/apimachinery/pkg/util/rand"

	hashutil "github.com/karmada-io/karmada/pkg/util/hash"
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

// GenerateBindingName will generate binding name by kind and name
func GenerateBindingName(kind, name string) string {
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

// GenerateWorkName will generate work name by its name and the hash of its namespace, kind and name.
func GenerateWorkName(kind, name, namespace string) string {
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
