package runtime

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	filterPluginName         = "filterPlugin"
	scorePluginName          = "scorePlugin"
	filterAndScorePluginName = "filterAndScorePlugin"
)

type filterPlugin struct {
	code framework.Code
}

var _ framework.FilterPlugin = (*filterPlugin)(nil)

func newJustFilterPlugin() (framework.Plugin, error) {
	return &filterPlugin{}, nil
}

func (j *filterPlugin) Name() string {
	return filterPluginName
}

func (j *filterPlugin) Filter(ctx context.Context, placement *policyv1alpha1.Placement, bindingSpec *workv1alpha2.ResourceBindingSpec, clusterv1alpha1 *clusterv1alpha1.Cluster) *framework.Result {
	return framework.NewResult(j.code)
}

type scorePlugin struct {
	name               string
	scoreCode          framework.Code
	normalizeScoreCode framework.Code
	clusterToScore     map[*clusterv1alpha1.Cluster]int64
}

var _ framework.ScorePlugin = (*scorePlugin)(nil)
var _ framework.ScoreExtensions = (*scorePlugin)(nil)

func newJustScorePlugin() (framework.Plugin, error) {
	return &scorePlugin{}, nil
}

func (j *scorePlugin) Name() string {
	if j.name == "" {
		return scorePluginName
	}

	return j.name
}

func (j *scorePlugin) Score(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	score, ok := j.clusterToScore[cluster]
	if !ok {
		return framework.MinClusterScore, framework.NewResult(j.scoreCode)
	}

	return score, framework.NewResult(j.scoreCode)
}

func (j *scorePlugin) ScoreExtensions() framework.ScoreExtensions {
	return j
}

func (j *scorePlugin) NormalizeScore(ctx context.Context, scores framework.ClusterScoreList) *framework.Result {
	return framework.NewResult(j.normalizeScoreCode)
}

type filterAndScorePlugin struct{}

var _ framework.FilterPlugin = (*filterAndScorePlugin)(nil)
var _ framework.ScorePlugin = (*filterAndScorePlugin)(nil)

func newBothPlugin() (framework.Plugin, error) {
	return &filterAndScorePlugin{}, nil
}

func (b *filterAndScorePlugin) Name() string {
	return filterAndScorePluginName
}

func (b *filterAndScorePlugin) Filter(ctx context.Context, placement *policyv1alpha1.Placement, bindingSpec *workv1alpha2.ResourceBindingSpec, clusterv1alpha1 *clusterv1alpha1.Cluster) *framework.Result {
	return framework.NewResult(framework.Success)
}

func (b *filterAndScorePlugin) Score(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	return framework.MinClusterScore, framework.NewResult(framework.Success)
}

func (b *filterAndScorePlugin) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

func TestNewFramework(t *testing.T) {
	// 3 different cases covered
	r := Registry{
		filterPluginName:         newJustFilterPlugin,
		scorePluginName:          newJustScorePlugin,
		filterAndScorePluginName: newBothPlugin,
	}

	fr, err := NewFramework(r)
	if err != nil {
		t.Errorf("NewFramework() returned error: %v", err)
		return
	}

	impl := fr.(*frameworkImpl)

	expectFilterPlugin(t, impl)
	expectScorePlugin(t, impl)
}

func expectFilterPlugin(t *testing.T, impl *frameworkImpl) {
	expectedFilterPlugins := sets.NewString(filterPluginName, filterAndScorePluginName)
	gotFilterPlugins := sets.String{}
	for _, filterPlugin := range impl.filterPlugins {
		gotFilterPlugins.Insert(filterPlugin.Name())
	}

	if !expectedFilterPlugins.Equal(gotFilterPlugins) {
		t.Errorf("FilterPlugin not equal, got: %v, expected: %v", gotFilterPlugins, expectedFilterPlugins)
	}
}

func expectScorePlugin(t *testing.T, impl *frameworkImpl) {
	expectedScorePlugins := sets.NewString(scorePluginName, filterAndScorePluginName)

	gotScorePlugins := sets.String{}
	for _, scorePlugin := range impl.scorePlugins {
		gotScorePlugins.Insert(scorePlugin.Name())
	}

	if !expectedScorePlugins.Equal(gotScorePlugins) {
		t.Errorf("ScorePlugin not equal, got: %v, expected: %v", gotScorePlugins, expectedScorePlugins)
	}
}

func Test_frameworkImpl_RunFilterPlugins(t *testing.T) {
	tests := []struct {
		name          string
		filterPlugins []framework.FilterPlugin
		wantSuccess   bool
	}{
		{
			name: "Success",
			filterPlugins: []framework.FilterPlugin{
				&filterPlugin{
					framework.Success,
				},
				&filterPlugin{
					framework.Success,
				},
			},
			wantSuccess: true,
		},
		{
			name: "FAIL",
			filterPlugins: []framework.FilterPlugin{
				&filterPlugin{
					framework.Success,
				},
				&filterPlugin{
					framework.Unschedulable,
				},
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frw := &frameworkImpl{
				filterPlugins: tt.filterPlugins,
			}

			got := frw.RunFilterPlugins(context.TODO(), nil, nil, nil)

			if got.IsSuccess() != tt.wantSuccess {
				t.Errorf("RunFilterPlugins() = %v, want %v", got, tt.wantSuccess)
			}
		})
	}
}

func Test_frameworkImpl_RunScorePlugins(t *testing.T) {
	cluster1 := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}}
	cluster2 := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}}

	tests := []struct {
		name         string
		clusters     []*clusterv1alpha1.Cluster
		scorePlugins []framework.ScorePlugin
		wantErr      bool
		want         framework.PluginToClusterScores
	}{
		{
			name:     "Score Fail",
			clusters: []*clusterv1alpha1.Cluster{cluster1},
			scorePlugins: []framework.ScorePlugin{
				&scorePlugin{
					scoreCode: framework.Error,
				},
			},
			wantErr: true,
		},
		{
			name:     "ScoreExtension Fail",
			clusters: []*clusterv1alpha1.Cluster{cluster1},
			scorePlugins: []framework.ScorePlugin{
				&scorePlugin{
					scoreCode:          framework.Success,
					normalizeScoreCode: framework.Error,
				},
			},
			wantErr: true,
		},
		{
			name:     "Success",
			clusters: []*clusterv1alpha1.Cluster{cluster1, cluster2},
			scorePlugins: []framework.ScorePlugin{
				&scorePlugin{
					name:               "scorePlugin1",
					scoreCode:          framework.Success,
					normalizeScoreCode: framework.Success,
					clusterToScore: map[*clusterv1alpha1.Cluster]int64{
						cluster1: 1,
						cluster2: 2,
					},
				},
				&scorePlugin{
					name:               "scorePlugin2",
					scoreCode:          framework.Success,
					normalizeScoreCode: framework.Success,
					clusterToScore: map[*clusterv1alpha1.Cluster]int64{
						cluster1: 3,
						cluster2: 4,
					},
				},
			},
			wantErr: false,
			want: framework.PluginToClusterScores{
				"scorePlugin1": framework.ClusterScoreList{
					framework.ClusterScore{
						Cluster: cluster1,
						Score:   1,
					},
					framework.ClusterScore{
						Cluster: cluster2,
						Score:   2,
					},
				},
				"scorePlugin2": framework.ClusterScoreList{
					framework.ClusterScore{
						Cluster: cluster1,
						Score:   3,
					},
					framework.ClusterScore{
						Cluster: cluster2,
						Score:   4,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frw := &frameworkImpl{
				scorePlugins: tt.scorePlugins,
			}

			got, err := frw.RunScorePlugins(context.TODO(), nil, nil, tt.clusters)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunScorePlugins() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunScorePlugins() got = %v, want %v", got, tt.want)
			}
		})
	}
}
