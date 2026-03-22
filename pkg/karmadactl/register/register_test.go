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

package register

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	karmadautil "github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestCleanupCreatedClusterInControlPlane(t *testing.T) {
	tests := []struct {
		name            string
		cluster         *clusterv1alpha1.Cluster
		createdUID      types.UID
		wantClusterGone bool
	}{
		{
			name: "delete created cluster with matching uid",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "member1", UID: types.UID("uid-1")},
			},
			createdUID:      types.UID("uid-1"),
			wantClusterGone: true,
		},
		{
			name: "keep pre-existing cluster with different uid",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "member1", UID: types.UID("uid-1")},
			},
			createdUID:      types.UID("uid-2"),
			wantClusterGone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fakekarmadaclient.NewClientset(tt.cluster)
			err := cleanupCreatedClusterInControlPlane(client, tt.cluster.Name, tt.createdUID, 2*time.Second)
			if err != nil {
				t.Fatalf("cleanupCreatedClusterInControlPlane() error = %v", err)
			}

			_, getErr := client.ClusterV1alpha1().Clusters().Get(context.TODO(), tt.cluster.Name, metav1.GetOptions{})
			if tt.wantClusterGone && getErr == nil {
				t.Fatalf("expected cluster %s to be deleted", tt.cluster.Name)
			}
			if !tt.wantClusterGone && getErr != nil {
				t.Fatalf("expected cluster %s to remain, got err=%v", tt.cluster.Name, getErr)
			}
		})
	}
}

func TestCommandRegisterOptionGenerateClusterInControlPlane(t *testing.T) {
	option := &CommandRegisterOption{
		ClusterName:           "member1",
		ClusterNamespace:      "custom-cluster-ns",
		ClusterProvider:       "provider-a",
		ClusterRegion:         "region-a",
		ClusterZones:          []string{"zone-a", "zone-b"},
		ProxyServerAddress:    "https://proxy.internal:8443",
		clusterID:             "cluster-uid",
		memberClusterEndpoint: "https://member.example:6443",
		memberClusterConfig: &rest.Config{
			TLSClientConfig: rest.TLSClientConfig{Insecure: true},
		},
	}

	cluster := option.generateClusterInControlPlane()

	if cluster.Name != option.ClusterName {
		t.Fatalf("cluster name = %s, want %s", cluster.Name, option.ClusterName)
	}
	if cluster.Spec.SyncMode != clusterv1alpha1.Pull {
		t.Fatalf("cluster sync mode = %s, want %s", cluster.Spec.SyncMode, clusterv1alpha1.Pull)
	}
	if cluster.Spec.ID != option.clusterID {
		t.Fatalf("cluster ID = %s, want %s", cluster.Spec.ID, option.clusterID)
	}
	if cluster.Annotations[karmadautil.ClusterNamespaceAnnotation] != option.ClusterNamespace {
		t.Fatalf("cluster namespace annotation = %s, want %s", cluster.Annotations[karmadautil.ClusterNamespaceAnnotation], option.ClusterNamespace)
	}
	if cluster.Spec.SecretRef == nil || cluster.Spec.SecretRef.Namespace != option.ClusterNamespace || cluster.Spec.SecretRef.Name != option.ClusterName {
		t.Fatalf("cluster secretRef = %#v, want namespace=%s name=%s", cluster.Spec.SecretRef, option.ClusterNamespace, option.ClusterName)
	}
	if cluster.Spec.ImpersonatorSecretRef == nil || cluster.Spec.ImpersonatorSecretRef.Namespace != option.ClusterNamespace || cluster.Spec.ImpersonatorSecretRef.Name != names.GenerateImpersonationSecretName(option.ClusterName) {
		t.Fatalf("cluster impersonatorSecretRef = %#v", cluster.Spec.ImpersonatorSecretRef)
	}
	if !cluster.Spec.InsecureSkipTLSVerification {
		t.Fatalf("cluster insecureSkipTLSVerification = false, want true")
	}
	if cluster.Spec.APIEndpoint != option.memberClusterEndpoint {
		t.Fatalf("cluster API endpoint = %s, want %s", cluster.Spec.APIEndpoint, option.memberClusterEndpoint)
	}
	if cluster.Spec.ProxyURL != option.ProxyServerAddress {
		t.Fatalf("cluster proxy URL = %s, want %s", cluster.Spec.ProxyURL, option.ProxyServerAddress)
	}
	if cluster.Spec.Provider != option.ClusterProvider {
		t.Fatalf("cluster provider = %s, want %s", cluster.Spec.Provider, option.ClusterProvider)
	}
	if cluster.Spec.Region != option.ClusterRegion {
		t.Fatalf("cluster region = %s, want %s", cluster.Spec.Region, option.ClusterRegion)
	}
	if len(cluster.Spec.Zones) != len(option.ClusterZones) {
		t.Fatalf("cluster zones = %v, want %v", cluster.Spec.Zones, option.ClusterZones)
	}
}
