package names

import (
	"fmt"
	"strings"
)

const (
	// KubernetesReservedNSPrefix is the prefix of namespace which reserved by Kubernetes system, such as:
	// - kube-system
	// - kube-public
	// - kube-node-lease
	KubernetesReservedNSPrefix = "kube-"

	// KarmadaReservedNSPrefix is the prefix of namespace which reserved by Karmada system, such as:
	// - karmada-system
	// - karmada-cluster
	// - karmada-es-*
	KarmadaReservedNSPrefix = "karmada-"
)

// executionSpacePrefix is the prefix of execution space
const executionSpacePrefix = "karmada-es-"

// endpointSlicePrefix is the prefix of collected EndpointSlice from member clusters.
const endpointSlicePrefix = "imported"

// endpointSlicePrefix is the prefix of service derived from ServiceImport.
const derivedServicePrefix = "derived"

// GenerateExecutionSpaceName generates execution space name for the given member cluster
func GenerateExecutionSpaceName(clusterName string) (string, error) {
	if clusterName == "" {
		return "", fmt.Errorf("the member cluster name is empty")
	}
	executionSpace := executionSpacePrefix + clusterName
	return executionSpace, nil
}

// GetClusterName returns member cluster name for the given execution space
func GetClusterName(executionSpaceName string) (string, error) {
	if !strings.HasPrefix(executionSpaceName, executionSpacePrefix) {
		return "", fmt.Errorf("the execution space name is in wrong format")
	}
	return strings.TrimPrefix(executionSpaceName, executionSpacePrefix), nil
}

// GenerateBindingName will generate binding name by kind and name
func GenerateBindingName(kind, name string) string {
	return strings.ToLower(name + "-" + kind)
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

// GenerateClusterAPISecretName generates the secret name of cluster authentication in cluster-api.
func GenerateClusterAPISecretName(clusterName string) string {
	return fmt.Sprintf("%s-kubeconfig", clusterName)
}
