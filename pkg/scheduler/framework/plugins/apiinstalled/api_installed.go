package apiinstalled

import (
	"context"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "APIInstalled"
)

// APIInstalled is a plugin that checks if the API(CRD) of the resource is installed in the target cluster.
type APIInstalled struct{}

var _ framework.FilterPlugin = &APIInstalled{}

// New instantiates the APIInstalled plugin.
func New() framework.Plugin {
	return &APIInstalled{}
}

// Name returns the plugin name.
func (p *APIInstalled) Name() string {
	return Name
}

// Filter checks if the API(CRD) of the resource is installed in the target cluster.
func (p *APIInstalled) Filter(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) *framework.Result {
	resource := spec.Resource
	if !helper.IsAPIEnabled(cluster.Status.APIEnablements, resource.APIVersion, resource.Kind) {
		klog.V(2).Infof("Cluster(%s) not fit as missing API(%s, kind=%s)", cluster.Name, resource.APIVersion, resource.Kind)
		return framework.NewResult(framework.Unschedulable, "no such API resource")
	}

	return framework.NewResult(framework.Success)
}
