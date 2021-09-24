package runtime

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	plugins2 "github.com/karmada-io/karmada/pkg/scheduler/framework/plugins"
)

// frameworkImpl implements the Framework interface and is responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	filterPlugins []framework.FilterPlugin
	scorePlugins  []framework.ScorePlugin
}

var _ framework.Framework = &frameworkImpl{}

// NewFramework creates a scheduling framework.
func NewFramework(plugins []string) framework.Framework {
	pluginsMap := plugins2.NewPlugins()
	out := &frameworkImpl{}
	filterPluginsList := reflect.ValueOf(&out.filterPlugins).Elem()
	scorePluginsList := reflect.ValueOf(&out.scorePlugins).Elem()

	filterType := filterPluginsList.Type().Elem()
	scoreType := scorePluginsList.Type().Elem()

	for _, p := range plugins {
		plugin := pluginsMap[p]
		if plugin == nil {
			klog.Warningf("scheduling plugin %s not exists", p)
			continue
		}
		addPluginToList(plugin, filterType, &filterPluginsList)
		addPluginToList(plugin, scoreType, &scorePluginsList)
	}

	return out
}

// RunFilterPlugins runs the set of configured Filter plugins for resources on the cluster.
// If any of the result is not success, the cluster is not suited for the resource.
func (frw *frameworkImpl) RunFilterPlugins(ctx context.Context, placement *policyv1alpha1.Placement, resource *workv1alpha2.ObjectReference, cluster *clusterv1alpha1.Cluster) framework.PluginToResult {
	result := make(framework.PluginToResult, len(frw.filterPlugins))
	for _, p := range frw.filterPlugins {
		pluginResult := p.Filter(ctx, placement, resource, cluster)
		result[p.Name()] = pluginResult
	}

	return result
}

// RunScorePlugins runs the set of configured Filter plugins for resources on the cluster.
// If any of the result is not success, the cluster is not suited for the resource.
func (frw *frameworkImpl) RunScorePlugins(ctx context.Context, placement *policyv1alpha1.Placement, clusters []*clusterv1alpha1.Cluster) (framework.PluginToClusterScores, error) {
	result := make(framework.PluginToClusterScores, len(frw.filterPlugins))
	for _, p := range frw.scorePlugins {
		var scoreList framework.ClusterScoreList
		for _, cluster := range clusters {
			score, res := p.Score(ctx, placement, cluster)
			if !res.IsSuccess() {
				return nil, fmt.Errorf("plugin %q failed with: %w", p.Name(), res.AsError())
			}
			scoreList = append(scoreList, framework.ClusterScore{
				Name:  cluster.Name,
				Score: score,
			})
		}
		result[p.Name()] = scoreList
	}

	return result, nil
}

func addPluginToList(plugin framework.Plugin, pluginType reflect.Type, pluginList *reflect.Value) {
	if reflect.TypeOf(plugin).Implements(pluginType) {
		newPlugins := reflect.Append(*pluginList, reflect.ValueOf(plugin))
		pluginList.Set(newPlugins)
	}
}
