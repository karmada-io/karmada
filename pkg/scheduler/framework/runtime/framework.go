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
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework/parallelize"
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

	parallelizer parallelize.Parallelizer
}

var _ framework.Framework = &frameworkImpl{}

type frameworkOptions struct {
	metricsRecorder *metricsRecorder
	parallelizer    parallelize.Parallelizer
}

// Option for the frameworkImpl.
type Option func(*frameworkOptions)

// WithParallelism sets parallelism for the scheduling frameworkImpl.
func WithParallelism(parallelism int) Option {
	return func(o *frameworkOptions) {
		o.parallelizer = parallelize.NewParallelizer(parallelism)
	}
}

func defaultFrameworkOptions() frameworkOptions {
	return frameworkOptions{
		metricsRecorder: newMetricsRecorder(1000, time.Second),
		parallelizer:    parallelize.NewParallelizer(parallelize.DefaultParallelism),
	}
}

// NewFramework creates a scheduling framework by registry.
func NewFramework(r Registry, opts ...Option) (framework.Framework, error) {
	options := defaultFrameworkOptions()
	for _, opt := range opts {
		opt(&options)
	}

	f := &frameworkImpl{
		metricsRecorder: options.metricsRecorder,
		parallelizer:    options.parallelizer,
	}
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
		metrics.FrameworkExtensionPointDuration.WithLabelValues(filter, result.Code().String()).Observe(utilmetrics.DurationInSeconds(startTime))
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
		metrics.FrameworkExtensionPointDuration.WithLabelValues(score, result.Code().String()).Observe(utilmetrics.DurationInSeconds(startTime))
	}()
	pluginToClusterScores := make(framework.PluginToClusterScores, len(frw.scorePlugins))
	for _, pl := range frw.scorePlugins {
		pluginToClusterScores[pl.Name()] = make(framework.ClusterScoreList, len(clusters))
	}
	ctx, cancel := context.WithCancel(ctx)
	errCh := parallelize.NewErrorChannel()

	// Run Score method for each cluster in parallel
	frw.parallelizer.Until(ctx, len(clusters), func(index int) {
		for _, p := range frw.scorePlugins {
			cluster := clusters[index]
			s, res := frw.runScorePlugin(ctx, p, placement, spec, cluster)
			if !res.IsSuccess() {
				err := fmt.Errorf("plugin %q failed with: %w", p.Name(), res.AsError())
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			pluginToClusterScores[p.Name()][index] = framework.ClusterScore{
				Cluster: cluster,
				Score:   s,
			}
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsResult(fmt.Errorf("running Score plugins: %w", err))
	}

	// Run NormalizeScore method for each ScorePlugin in parallel.
	frw.Parallelizer().Until(ctx, len(frw.scorePlugins), func(index int) {
		p := frw.scorePlugins[index]
		clusterScoreList := pluginToClusterScores[p.Name()]
		if p.ScoreExtensions() == nil {
			return
		}
		res := frw.runScoreExtension(ctx, p, clusterScoreList)
		if !res.IsSuccess() {
			err := fmt.Errorf("plugin %q normalizeScore failed with: %w", p.Name(), res.AsError())
			errCh.SendErrorWithCancel(err, cancel)
			return
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsResult(fmt.Errorf("running Normalize on Score plugins: %w", err))
	}

	// Apply score defaultWeights for each ScorePlugin in parallel.
	frw.Parallelizer().Until(ctx, len(frw.scorePlugins), func(index int) {
		p := frw.scorePlugins[index]
		weight, ok := frw.scorePluginsWeightMap[p.Name()]
		if ok {
			clusterScoreList := pluginToClusterScores[p.Name()]
			for i, clusterScore := range clusterScoreList {
				if clusterScore.Score > framework.MaxClusterScore || clusterScore.Score < framework.MinClusterScore {
					err := fmt.Errorf("plugin %q returns an invalid score %v, it should in the range of [%v, %v] after normalizing", p.Name(), clusterScore.Score, framework.MinClusterScore, framework.MaxClusterScore)
					errCh.SendErrorWithCancel(err, cancel)
					return
				}
				clusterScoreList[i].Score = clusterScore.Score * int64(weight)
			}
		}
	})

	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsResult(fmt.Errorf("applying score defaultWeights on Score plugins: %w", err))
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

// Parallelizer returns a parallelizer holding parallelism for scheduler.
func (frw *frameworkImpl) Parallelizer() parallelize.Parallelizer {
	return frw.parallelizer
}

func addPluginToList(plugin framework.Plugin, pluginType reflect.Type, pluginList *reflect.Value) {
	if reflect.TypeOf(plugin).Implements(pluginType) {
		newPlugins := reflect.Append(*pluginList, reflect.ValueOf(plugin))
		pluginList.Set(newPlugins)
	}
}
