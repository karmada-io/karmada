package runtime

import (
	"context"
	"fmt"
	"reflect"
	"time"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

const (
	filter                  = "Filter"
	score                   = "Score"
	scoreExtensionNormalize = "ScoreExtensionNormalize"
)

// frameworkImpl implements the Framework interface and is responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	scorePluginsWeightMap map[string]int
	filterPlugins         []framework.FilterPlugin
	scorePlugins          []framework.ScorePlugin

	metricsRecorder *metricsRecorder
	profileName     string
}

var _ framework.Framework = &frameworkImpl{}

type frameworkOptions struct {
	metricsRecorder *metricsRecorder
}

// Option for the frameworkImpl.
type Option func(*frameworkOptions)

func defaultFrameworkOptions() frameworkOptions {
	return frameworkOptions{
		metricsRecorder: newMetricsRecorder(1000, time.Second),
	}
}

// NewFramework creates a scheduling framework by registry.
func NewFramework(r Registry, profileName string, opts ...Option) (framework.Framework, error) {
	options := defaultFrameworkOptions()
	for _, opt := range opts {
		opt(&options)
	}

	f := &frameworkImpl{
		metricsRecorder: options.metricsRecorder,
	}
	f.profileName = profileName
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
func (frw *frameworkImpl) RunFilterPlugins(ctx context.Context, placement *policyv1alpha1.Placement, bindingSpec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (result *framework.Result) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(filter, result.Code().String(), frw.profileName).Observe(utilmetrics.DurationInSeconds(startTime))
	}()
	for _, p := range frw.filterPlugins {
		if result := frw.runFilterPlugin(ctx, p, placement, bindingSpec, cluster); !result.IsSuccess() {
			return result
		}
	}
	return framework.NewResult(framework.Success)
}

func (frw *frameworkImpl) runFilterPlugin(ctx context.Context, pl framework.FilterPlugin, placement *policyv1alpha1.Placement, bindingSpec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) *framework.Result {
	startTime := time.Now()
	result := pl.Filter(ctx, placement, bindingSpec, cluster)
	frw.metricsRecorder.observePluginDurationAsync(filter, pl.Name(), result, utilmetrics.DurationInSeconds(startTime))
	return result
}

// RunScorePlugins runs the set of configured Filter plugins for resources on the cluster.
// If any of the result is not success, the cluster is not suited for the resource.
func (frw *frameworkImpl) RunScorePlugins(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, clusters []*clusterv1alpha1.Cluster) (ps framework.PluginToClusterScores, result *framework.Result) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(score, result.Code().String(), frw.profileName).Observe(utilmetrics.DurationInSeconds(startTime))
	}()
	pluginToClusterScores := make(framework.PluginToClusterScores, len(frw.filterPlugins))
	for _, p := range frw.scorePlugins {
		var scoreList framework.ClusterScoreList
		for _, cluster := range clusters {
			s, res := frw.runScorePlugin(ctx, p, placement, spec, cluster)
			if !res.IsSuccess() {
				return nil, framework.AsResult(fmt.Errorf("plugin %q failed with: %w", p.Name(), res.AsError()))
			}
			scoreList = append(scoreList, framework.ClusterScore{
				Cluster: cluster,
				Score:   s,
			})
		}

		if p.ScoreExtensions() != nil {
			res := frw.runScoreExtension(ctx, p, scoreList)
			if !res.IsSuccess() {
				return nil, framework.AsResult(fmt.Errorf("plugin %q normalizeScore failed with: %w", p.Name(), res.AsError()))
			}
		}

		weight, ok := frw.scorePluginsWeightMap[p.Name()]
		if !ok {
			pluginToClusterScores[p.Name()] = scoreList
			continue
		}

		for i := range scoreList {
			scoreList[i].Score = scoreList[i].Score * int64(weight)
		}

		pluginToClusterScores[p.Name()] = scoreList
	}

	return pluginToClusterScores, nil
}

func (frw *frameworkImpl) runScorePlugin(ctx context.Context, pl framework.ScorePlugin, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	startTime := time.Now()
	s, result := pl.Score(ctx, placement, spec, cluster)
	frw.metricsRecorder.observePluginDurationAsync(score, pl.Name(), result, utilmetrics.DurationInSeconds(startTime))
	return s, result
}

func (frw *frameworkImpl) runScoreExtension(ctx context.Context, pl framework.ScorePlugin, scores framework.ClusterScoreList) *framework.Result {
	startTime := time.Now()
	result := pl.ScoreExtensions().NormalizeScore(ctx, scores)
	frw.metricsRecorder.observePluginDurationAsync(scoreExtensionNormalize, pl.Name(), result, utilmetrics.DurationInSeconds(startTime))
	return result
}

// ProfileName returns the profile name associated to this framework.
func (frw *frameworkImpl) ProfileName() string {
	return frw.profileName
}

func addPluginToList(plugin framework.Plugin, pluginType reflect.Type, pluginList *reflect.Value) {
	if reflect.TypeOf(plugin).Implements(pluginType) {
		newPlugins := reflect.Append(*pluginList, reflect.ValueOf(plugin))
		pluginList.Set(newPlugins)
	}
}
