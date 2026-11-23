/*
Copyright 2020 The Karmada Authors.

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

package names

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/rand"

	hashutil "github.com/karmada-io/karmada/pkg/util/hash"
)

const (
	// NamespaceKarmadaSystem is reserved namespace
	NamespaceKarmadaSystem = "karmada-system"
	// NamespaceKarmadaCluster is reserved namespace
	NamespaceKarmadaCluster = "karmada-cluster"
	// NamespaceDefault is reserved namespace
	NamespaceDefault = "default"
)

// The following constants define standard names for various Karmada components.
// These names are used consistently across the project to ensure uniformity and clarity.
// Using these constants helps avoid typos and ensures that all components are referenced with the correct names.
const (
	// KarmadaDeschedulerComponentName is the name of the Karmada Descheduler component.
	KarmadaDeschedulerComponentName = "karmada-descheduler"

	// KarmadaSchedulerEstimatorComponentName is the name of the Karmada Scheduler Estimator component.
	KarmadaSchedulerEstimatorComponentName = "karmada-scheduler-estimator"

	// KarmadaSearchComponentName is the name of the Karmada Search addon.
	KarmadaSearchComponentName = "karmada-search"

	// KarmadaMetricsAdapterComponentName is the name of the Karmada Metrics Adapter component.
	KarmadaMetricsAdapterComponentName = "karmada-metrics-adapter"

	// KarmadaAggregatedAPIServerComponentName is the name of the Karmada Aggregated API Server component.
	KarmadaAggregatedAPIServerComponentName = "karmada-aggregated-apiserver"

	// KarmadaAgentComponentName is the name of the Karmada Agent component.
	KarmadaAgentComponentName = "karmada-agent"

	// KarmadaSchedulerComponentName is the name of the Karmada Scheduler component.
	KarmadaSchedulerComponentName = "karmada-scheduler"

	// KarmadaWebhookComponentName is the name of the Karmada Webhook component.
	KarmadaWebhookComponentName = "karmada-webhook"

	// KarmadaControllerManagerComponentName is the name of the Karmada Controller Manager component.
	KarmadaControllerManagerComponentName = "karmada-controller-manager"

	// KubeControllerManagerComponentName is the name of the Kube Controller Manager component.
	KubeControllerManagerComponentName = "kube-controller-manager"

	// KarmadaEtcdComponentName is the name of the Karmada Etcd component.
	KarmadaEtcdComponentName = "karmada-etcd"

	// KarmadaAPIServerComponentName is the name of the Karmada API Server component.
	KarmadaAPIServerComponentName = "karmada-apiserver"
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

var (
	splitterPattern = regexp.MustCompile(`[._\-]`)
)

const (
	maxNameLength = 52 // plus hex suffix -0123456789 goes to 63, value length limit of label is 63
)

func ShortName(name string, splitter *regexp.Regexp, maxLength int) string {
	if len(name) <= maxLength {
		return name
	}
	parts := splitter.Split(name, -1)
	var length, index, trim int
	for i, part := range parts {
		if length+len(part)+i < maxLength {
			length += len(part)
			continue
		}
		if i == 0 {
			index = i
			break
		}
		for j := i; j > 0; j-- {
			trim = maxLength - length - len(parts)
			if trim >= 0 && len(parts[j]) > trim {
				index = j
				break
			}
			length -= len(parts[j-1])
		}
		if index != 0 {
			break
		}
	}
	if index == 0 {
		return name[:maxLength]
	}
	buffer := new(strings.Builder)
	if trim == 0 {
		buffer.WriteString(parts[index][:1])
	}
	for _, part := range parts[index+1:] {
		buffer.WriteString(part[:1])
	}
	if trim > 0 {
		parts = append(parts[:index], parts[index][:trim])
	} else {
		parts = parts[:index]
	}
	if rest := buffer.String(); len(rest) > 0 {
		parts = append(parts, rest)
	}
	return strings.Join(parts, "-")
}

// ShortNameLabelValue will truncate name to let length of name not to reach max length.
// This method is inspired by java spring, for example there is a class full name
// com.company.departure.business.module.abc.SomeClass, this will log as
// ccdbma.SomeClass, keep the last part of class name, the prefix parts only keep
// the first alphabet.
// This method tries best to keep prefix. e.g.
// input: lala.134-ab_abc-adfja2edklfja3rqdkfjasfa-1234567890123-a-b
// output: lala-134-ab-abc-adfja2edklfja3rqdkfjasfa-12345678-ab
func ShortNameLabelValue(name string) string {
	return ShortName(name, splitterPattern, maxNameLength)
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
	return fmt.Sprintf("%s-%s", ShortNameLabelValue(name), rand.SafeEncodeString(fmt.Sprint(hash.Sum32())))
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
		strings.HasPrefix(namespace, ExecutionSpacePrefix)
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

// NamespacedKey generates key with namespace and name.
func NamespacedKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}
