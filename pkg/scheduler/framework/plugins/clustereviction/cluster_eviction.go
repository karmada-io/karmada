package clustereviction

import (
	"context"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "ClusterEviction"
)

// ClusterEviction is a plugin that checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
type ClusterEviction struct{}

var _ framework.FilterPlugin = &ClusterEviction{}

// New instantiates the ClusterEviction plugin.
func New() (framework.Plugin, error) {
	return &ClusterEviction{}, nil
}

// Name returns the plugin name.
func (p *ClusterEviction) Name() string {
	return Name
}

// Filter checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
func (p *ClusterEviction) Filter(_ context.Context, bindingSpec *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, cluster *clusterv1alpha1.Cluster) *framework.Result {
	if helper.ClusterInGracefulEvictionTasks(bindingSpec.GracefulEvictionTasks, cluster.Name) {
		klog.V(2).Infof("Cluster(%s) is in the process of eviction.", cluster.Name)
		return framework.NewResult(framework.Unschedulable, "cluster(s) is in the process of eviction")
	}

	return framework.NewResult(framework.Success)
}
