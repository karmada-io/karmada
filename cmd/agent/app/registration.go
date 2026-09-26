/*
Copyright 2021 The Karmada Authors.

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

package app

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/agent/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
)

// validateExternallyRegisteredCluster verifies that a cluster registered outside of the agent
// (i.e. with --register-cluster=false) is in a state the agent can run against: it exists, is not
// being deleted, uses Pull sync mode, and its ID matches the member cluster the agent is running for.
func validateExternallyRegisteredCluster(ctx context.Context, opts *options.Options, karmadaClient karmadaclientset.Interface, memberKubeClient kubeclientset.Interface) error {
	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(ctx, opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get cluster %q from control plane: %w", opts.ClusterName, err)
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return fmt.Errorf("cluster %q is being deleted", opts.ClusterName)
	}

	if cluster.Spec.SyncMode != clusterv1alpha1.Pull {
		return fmt.Errorf("cluster %q has SyncMode %q, expected %q for an externally registered cluster", opts.ClusterName, cluster.Spec.SyncMode, clusterv1alpha1.Pull)
	}

	clusterID, err := util.ObtainClusterID(memberKubeClient)
	if err != nil {
		return fmt.Errorf("failed to obtain cluster ID from member cluster: %w", err)
	}

	if cluster.Spec.ID != "" && cluster.Spec.ID != clusterID {
		return fmt.Errorf("cluster ID mismatch: control plane has %q but member cluster reports %q", cluster.Spec.ID, clusterID)
	}
	if cluster.Spec.ID == "" {
		klog.Warningf("Cluster %q has no ID set in the control plane; consider setting spec.id to %q", opts.ClusterName, clusterID)
	}

	klog.Infof("Successfully validated externally registered cluster %q", opts.ClusterName)
	return nil
}
