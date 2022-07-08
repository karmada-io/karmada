package util

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	karmadaclientsetfake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
)

func newCluster(name string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec:   clusterv1alpha1.ClusterSpec{},
		Status: clusterv1alpha1.ClusterStatus{},
	}
}

func withAPIEndPoint(cluster *clusterv1alpha1.Cluster, apiEndPoint string) *clusterv1alpha1.Cluster {
	cluster.Spec.APIEndpoint = apiEndPoint
	return cluster
}

func withSyncMode(cluster *clusterv1alpha1.Cluster, syncMode clusterv1alpha1.ClusterSyncMode) *clusterv1alpha1.Cluster {
	cluster.Spec.SyncMode = syncMode
	return cluster
}

func TestCreateOrUpdateClusterObject(t *testing.T) {
	fakeClient := karmadaclientsetfake.NewSimpleClientset()
	type args struct {
		controlPlaneClient karmadaclientset.Interface
		clusterObj         *clusterv1alpha1.Cluster
		mutate             func(*clusterv1alpha1.Cluster)
		aop                func() func()
	}
	tests := []struct {
		name    string
		args    args
		want    *clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "cluster exist, and update cluster",
			args: args{
				controlPlaneClient: fakeClient,
				clusterObj:         newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
				aop: func() func() {
					cluster := withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443")
					_, _ = fakeClient.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
					return func() {
						_ = fakeClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), cluster.Name, metav1.DeleteOptions{})
					}
				},
			},
			want: withSyncMode(withAPIEndPoint(newCluster(ClusterMember1), "https://127.0.0.1:6443"), clusterv1alpha1.Pull),
		},
		{
			name: "cluster not exist, and create cluster",
			args: args{
				controlPlaneClient: fakeClient,
				clusterObj:         newCluster(ClusterMember1),
				mutate: func(cluster *clusterv1alpha1.Cluster) {
					cluster.Spec.SyncMode = clusterv1alpha1.Pull
				},
				aop: func() func() {
					cluster := withSyncMode(newCluster(ClusterMember1), clusterv1alpha1.Pull)
					return func() {
						_ = fakeClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), cluster.Name, metav1.DeleteOptions{})
					}
				},
			},
			want: withSyncMode(newCluster(ClusterMember1), clusterv1alpha1.Pull),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.aop != nil {
				cancel := tt.args.aop()
				defer cancel()
			}
			got, err := CreateOrUpdateClusterObject(tt.args.controlPlaneClient, tt.args.clusterObj, tt.args.mutate)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateClusterObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateOrUpdateClusterObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}
