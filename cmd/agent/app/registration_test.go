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
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"

	"github.com/karmada-io/karmada/cmd/agent/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util"
)

func newTestOpts(clusterName string) *options.Options {
	return &options.Options{
		ClusterName: clusterName,
	}
}

func newKubeSystemNamespace(uid string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metav1.NamespaceSystem,
			UID:  types.UID(uid),
		},
	}
}

func newCluster(name string, syncMode clusterv1alpha1.ClusterSyncMode, id string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterv1alpha1.ClusterSpec{
			SyncMode: syncMode,
			ID:       id,
		},
	}
}

func TestValidateExternallyRegisteredCluster(t *testing.T) {
	const (
		clusterName = "member1"
		clusterUID  = "test-uid-12345"
	)

	tests := []struct {
		name                 string
		cluster              *clusterv1alpha1.Cluster
		nsUID                string
		noKubeSystemNS       bool
		noLeaseNamespace     bool
		leaseNamespaceGetErr error
		wantErr              bool
		errContains          string
	}{
		{
			name:    "happy path with matching ID",
			cluster: newCluster(clusterName, clusterv1alpha1.Pull, clusterUID),
			nsUID:   clusterUID,
			wantErr: false,
		},
		{
			name:        "empty cluster ID",
			cluster:     newCluster(clusterName, clusterv1alpha1.Pull, ""),
			nsUID:       clusterUID,
			wantErr:     true,
			errContains: "has no ID set",
		},
		{
			name:        "cluster not found",
			cluster:     nil,
			nsUID:       clusterUID,
			wantErr:     true,
			errContains: "failed to get cluster",
		},
		{
			name: "cluster being deleted is still accepted",
			cluster: func() *clusterv1alpha1.Cluster {
				c := newCluster(clusterName, clusterv1alpha1.Pull, clusterUID)
				now := metav1.NewTime(time.Now())
				c.DeletionTimestamp = &now
				c.Finalizers = []string{"test-finalizer"}
				return c
			}(),
			nsUID:   clusterUID,
			wantErr: false,
		},
		{
			name:        "wrong sync mode (Push)",
			cluster:     newCluster(clusterName, clusterv1alpha1.Push, clusterUID),
			nsUID:       clusterUID,
			wantErr:     true,
			errContains: "SyncMode",
		},
		{
			name:        "cluster ID mismatch",
			cluster:     newCluster(clusterName, clusterv1alpha1.Pull, "different-uid"),
			nsUID:       clusterUID,
			wantErr:     true,
			errContains: "cluster ID mismatch",
		},
		{
			name:             "lease namespace missing in control plane",
			cluster:          newCluster(clusterName, clusterv1alpha1.Pull, clusterUID),
			nsUID:            clusterUID,
			noLeaseNamespace: true,
			wantErr:          true,
			errContains:      "which holds the cluster lease",
		},
		{
			name:                 "lease namespace check not permitted is not fatal",
			cluster:              newCluster(clusterName, clusterv1alpha1.Pull, clusterUID),
			nsUID:                clusterUID,
			leaseNamespaceGetErr: apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, util.NamespaceClusterLease, errors.New("access denied")),
			wantErr:              false,
		},
		{
			name:           "member cluster missing kube-system namespace",
			cluster:        newCluster(clusterName, clusterv1alpha1.Pull, clusterUID),
			noKubeSystemNS: true,
			wantErr:        true,
			errContains:    "failed to obtain cluster ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var karmadaClient *karmadafake.Clientset
			if tt.cluster != nil {
				karmadaClient = karmadafake.NewSimpleClientset(tt.cluster)
			} else {
				karmadaClient = karmadafake.NewSimpleClientset()
			}

			var memberKubeClient *kubefake.Clientset
			if tt.noKubeSystemNS {
				memberKubeClient = kubefake.NewSimpleClientset()
			} else {
				memberKubeClient = kubefake.NewSimpleClientset(newKubeSystemNamespace(tt.nsUID))
			}
			controlPlaneKubeClient := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: util.NamespaceClusterLease}})
			if tt.noLeaseNamespace {
				controlPlaneKubeClient = kubefake.NewSimpleClientset()
			}
			if tt.leaseNamespaceGetErr != nil {
				controlPlaneKubeClient.PrependReactor("get", "namespaces", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.leaseNamespaceGetErr
				})
			}
			opts := newTestOpts(clusterName)

			err := validateExternallyRegisteredCluster(context.Background(), opts, karmadaClient, controlPlaneKubeClient, memberKubeClient)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
