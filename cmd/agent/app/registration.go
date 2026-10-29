/*
Copyright 2026 The Karmada Authors.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/agent/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
)

// validateExternallyRegisteredCluster verifies that a cluster registered outside of the agent
// (i.e. with --enable-cluster-registration=false) is in a state the agent can run against: it
// exists, uses Pull sync mode, and has a spec.id matching the member cluster the agent runs for.
// The namespace holding the cluster lease must also exist; without it the agent cannot renew the
// lease and the cluster is repeatedly marked Unknown.
// A cluster that is being deleted is still accepted, so a restarted agent can finish cleaning up
// its Works, as it does when registration is enabled.
func validateExternallyRegisteredCluster(ctx context.Context, opts *options.Options, karmadaClient karmadaclientset.Interface, controlPlaneKubeClient, memberKubeClient kubeclientset.Interface) error {
	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(ctx, opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get cluster %q from control plane: %w", opts.ClusterName, err)
	}

	if cluster.Spec.SyncMode != clusterv1alpha1.Pull {
		return fmt.Errorf("cluster %q has SyncMode %q, expected %q for an externally registered cluster", opts.ClusterName, cluster.Spec.SyncMode, clusterv1alpha1.Pull)
	}

	if _, err := controlPlaneKubeClient.CoreV1().Namespaces().Get(ctx, util.NamespaceClusterLease, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("namespace %q, which holds the cluster lease, must exist in the control plane", util.NamespaceClusterLease)
		}
		// Being unable to check (e.g. no permission to get namespaces) shouldn't block startup.
		klog.Warningf("Unable to verify that namespace %q exists in the control plane: %v", util.NamespaceClusterLease, err)
	}

	clusterID, err := util.ObtainClusterID(memberKubeClient)
	if err != nil {
		return fmt.Errorf("failed to obtain cluster ID from member cluster: %w", err)
	}

	if cluster.Spec.ID == "" {
		return fmt.Errorf("cluster %q has no ID set in the control plane; set spec.id to %q", opts.ClusterName, clusterID)
	}
	if cluster.Spec.ID != clusterID {
		return fmt.Errorf("cluster ID mismatch: control plane has %q but member cluster reports %q", cluster.Spec.ID, clusterID)
	}

	klog.Infof("Successfully validated externally registered cluster %q", opts.ClusterName)
	return nil
}
