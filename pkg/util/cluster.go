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

package util

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

const (
	// NamespaceClusterLease is the namespace which cluster lease are stored.
	NamespaceClusterLease = "karmada-cluster"
)

// IsClusterReady tells whether the cluster status in 'Ready' condition.
func IsClusterReady(clusterStatus *clusterv1alpha1.ClusterStatus) bool {
	return meta.IsStatusConditionTrue(clusterStatus.Conditions, clusterv1alpha1.ClusterConditionReady)
}

// GetCluster returns the given Cluster resource
func GetCluster(hostClient client.Client, clusterName string) (*clusterv1alpha1.Cluster, error) {
	cluster := &clusterv1alpha1.Cluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

// CreateClusterIfNotExist try to create the cluster if it does not exist.
func CreateClusterIfNotExist(client client.Client, clusterObj *clusterv1alpha1.Cluster) error {
	cluster := &clusterv1alpha1.Cluster{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: clusterObj.Name}, cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := client.Create(context.TODO(), clusterObj); err != nil {
			return err
		}
	}
	return nil
}
