/*
Copyright The Karmada Authors.

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

// estimatorServicePrefix is the prefix of scheduler estimator service name.
const estimatorServicePrefix = "karmada-scheduler-estimator"

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

// GenerateBindingReferenceKey will generate the key of binding object with the hash of its namespace and name.
func GenerateBindingReferenceKey(namespace, name string) string {
	var bindingName string
	if len(namespace) == 0 {
		bindingName = namespace + "-" + name
	} else {
		bindingName = name
	}
	hash := fnv.New32a()
	hashutil.DeepHashObject(hash, bindingName)
	return rand.SafeEncodeString(fmt.Sprint(hash.Sum32()))
}

// GenerateWorkName will generate work name by its name and the hash of its namespace, kind and name.
func GenerateWorkName(kind, name, namespace string) string {
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
func GenerateEstimatorServiceName(clusterName string) string {
	return fmt.Sprintf("%s-%s", estimatorServicePrefix, clusterName)
}

// GenerateClusterAPISecretName generates the secret name of cluster authentication in cluster-api.
func GenerateClusterAPISecretName(clusterName string) string {
	return fmt.Sprintf("%s-kubeconfig", clusterName)
}
