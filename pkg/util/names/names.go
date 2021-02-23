package names

import (
	"fmt"
	"strings"
)

// executionSpacePrefix is the prefix of execution space
const executionSpacePrefix = "karmada-es-"

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

// GetNamespaceAndName will get namespace and name from ownerLabel.
// For example: "karmada-es-member-1.default-deployment-nginx"
func GetNamespaceAndName(value string) (string, string, error) {
	splits := strings.Split(value, ".")
	if len(splits) != 2 {
		return "", "", fmt.Errorf("value is not correct")
	}
	return splits[0], splits[1], nil
}

// GenerateBindingName will generate binding name by namespace, kind and name
func GenerateBindingName(namespace, kind, name string) string {
	return strings.ToLower(namespace + "-" + kind + "-" + name)
}

// GenerateClusterResourceBindingName will generate ClusterResourceBinding name by kind and name
func GenerateClusterResourceBindingName(kind, name string) string {
	return strings.ToLower(kind + "-" + name)
}

// GenerateOwnerLabelValue will get owner label value.
func GenerateOwnerLabelValue(namespace, name string) string {
	return namespace + "." + name
}

// GenerateOwnerLabelValueForClusterResourceBinding will get owner label value for ClusterResourceBinding.
func GenerateOwnerLabelValueForClusterResourceBinding(name string) string {
	return name
}

// GenerateServiceAccountName generates the name of a ServiceAccount.
func GenerateServiceAccountName(clusterName string) string {
	return fmt.Sprintf("%s-%s", "karmada", clusterName)
}

// GenerateRoleName generates the name of a Role or ClusterRole.
func GenerateRoleName(serviceAccountName string) string {
	return fmt.Sprintf("karmada-controller-manager:%s", serviceAccountName)
}
