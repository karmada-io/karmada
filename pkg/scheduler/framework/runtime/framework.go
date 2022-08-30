package runtime

import (
	"context"
	"fmt"
	"reflect"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// frameworkImpl implements the Framework interface and is responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	scorePluginsWeightMap map[string]int
	filterPlugins         []framework.FilterPlugin
	scorePlugins          []framework.ScorePlugin
}

var _ framework.Framework = &frameworkImpl{}

// NewFramework creates a scheduling framework by registry.
func NewFramework(r Registry) (framework.Framework, error) {
	f := &frameworkImpl{}
	filterPluginsList := reflect.ValueOf(&f.filterPlugins).Elem()
	scorePluginsList := reflect.ValueOf(&f.scorePlugins).Elem()
	filterType := filterPluginsList.Type().Elem()
	scoreType := scorePluginsList.Type().Elem()

	for name, factory := range r {
		p, err := factory()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize plugin %q: %w", name, err)
		}

		addPluginToList(p, filterType, &filterPluginsList)
		addPluginToList(p, scoreType, &scorePluginsList)
	}

	return f, nil
}

// RunFilterPlugins runs the set of configured Filter plugins for resources on the cluster.
// If any of the result is not success, the cluster is not suited for the resource.
func (frw *frameworkImpl) RunFilterPlugins(ctx context.Context, placement *policyv1alpha1.Placement, bindingSpec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) *framework.Result {
	for _, p := range frw.filterPlugins {
		if result := p.Filter(ctx, placement, bindingSpec, cluster); !result.IsSuccess() {
			return result
		}
	}
	return framework.NewResult(framework.Success)
}

// RunScorePlugins runs the set of configured Filter plugins for resources on the cluster.
// If any of the result is not success, the cluster is not suited for the resource.
func (frw *frameworkImpl) RunScorePlugins(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, clusters []*clusterv1alpha1.Cluster) (framework.PluginToClusterScores, error) {
	result := make(framework.PluginToClusterScores, len(frw.filterPlugins))
	for _, p := range frw.scorePlugins {
		var scoreList framework.ClusterScoreList
		for _, cluster := range clusters {
			score, res := p.Score(ctx, placement, spec, cluster)
			if !res.IsSuccess() {
				return nil, fmt.Errorf("plugin %q failed with: %w", p.Name(), res.AsError())
			}
			scoreList = append(scoreList, framework.ClusterScore{
				Cluster: cluster,
				Score:   score,
			})
		}

		if p.ScoreExtensions() != nil {
			res := p.ScoreExtensions().NormalizeScore(ctx, scoreList)
			if !res.IsSuccess() {
				return nil, fmt.Errorf("plugin %q normalizeScore failed with: %w", p.Name(), res.AsError())
			}
		}

		weight, ok := frw.scorePluginsWeightMap[p.Name()]
		if !ok {
			result[p.Name()] = scoreList
			continue
		}

		for i := range scoreList {
			scoreList[i].Score = scoreList[i].Score * int64(weight)
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
