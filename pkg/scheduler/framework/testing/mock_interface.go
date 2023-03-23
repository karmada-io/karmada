// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

// Package testing is a generated GoMock package.
package testing

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	v1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	framework "github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// MockFramework is a mock of Framework interface.
type MockFramework struct {
	ctrl     *gomock.Controller
	recorder *MockFrameworkMockRecorder
}

// MockFrameworkMockRecorder is the mock recorder for MockFramework.
type MockFrameworkMockRecorder struct {
	mock *MockFramework
}

// NewMockFramework creates a new mock instance.
func NewMockFramework(ctrl *gomock.Controller) *MockFramework {
	mock := &MockFramework{ctrl: ctrl}
	mock.recorder = &MockFrameworkMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFramework) EXPECT() *MockFrameworkMockRecorder {
	return m.recorder
}

// RunFilterPlugins mocks base method.
func (m *MockFramework) RunFilterPlugins(ctx context.Context, bindingSpec *v1alpha2.ResourceBindingSpec, bindingStatus *v1alpha2.ResourceBindingStatus, cluster *v1alpha1.Cluster) *framework.Result {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunFilterPlugins", ctx, bindingSpec, bindingStatus, cluster)
	ret0, _ := ret[0].(*framework.Result)
	return ret0
}

// RunFilterPlugins indicates an expected call of RunFilterPlugins.
func (mr *MockFrameworkMockRecorder) RunFilterPlugins(ctx, bindingSpec, bindingStatus, cluster interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunFilterPlugins", reflect.TypeOf((*MockFramework)(nil).RunFilterPlugins), ctx, bindingSpec, bindingStatus, cluster)
}

// RunScorePlugins mocks base method.
func (m *MockFramework) RunScorePlugins(ctx context.Context, spec *v1alpha2.ResourceBindingSpec, clusters []*v1alpha1.Cluster) (framework.PluginToClusterScores, *framework.Result) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunScorePlugins", ctx, spec, clusters)
	ret0, _ := ret[0].(framework.PluginToClusterScores)
	ret1, _ := ret[1].(*framework.Result)
	return ret0, ret1
}

// RunScorePlugins indicates an expected call of RunScorePlugins.
func (mr *MockFrameworkMockRecorder) RunScorePlugins(ctx, spec, clusters interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunScorePlugins", reflect.TypeOf((*MockFramework)(nil).RunScorePlugins), ctx, spec, clusters)
}

// MockPlugin is a mock of Plugin interface.
type MockPlugin struct {
	ctrl     *gomock.Controller
	recorder *MockPluginMockRecorder
}

// MockPluginMockRecorder is the mock recorder for MockPlugin.
type MockPluginMockRecorder struct {
	mock *MockPlugin
}

// NewMockPlugin creates a new mock instance.
func NewMockPlugin(ctrl *gomock.Controller) *MockPlugin {
	mock := &MockPlugin{ctrl: ctrl}
	mock.recorder = &MockPluginMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPlugin) EXPECT() *MockPluginMockRecorder {
	return m.recorder
}

// Name mocks base method.
func (m *MockPlugin) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockPluginMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockPlugin)(nil).Name))
}

// MockFilterPlugin is a mock of FilterPlugin interface.
type MockFilterPlugin struct {
	ctrl     *gomock.Controller
	recorder *MockFilterPluginMockRecorder
}

// MockFilterPluginMockRecorder is the mock recorder for MockFilterPlugin.
type MockFilterPluginMockRecorder struct {
	mock *MockFilterPlugin
}

// NewMockFilterPlugin creates a new mock instance.
func NewMockFilterPlugin(ctrl *gomock.Controller) *MockFilterPlugin {
	mock := &MockFilterPlugin{ctrl: ctrl}
	mock.recorder = &MockFilterPluginMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFilterPlugin) EXPECT() *MockFilterPluginMockRecorder {
	return m.recorder
}

// Filter mocks base method.
func (m *MockFilterPlugin) Filter(ctx context.Context, bindingSpec *v1alpha2.ResourceBindingSpec, bindingStatus *v1alpha2.ResourceBindingStatus, cluster *v1alpha1.Cluster) *framework.Result {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", ctx, bindingSpec, bindingStatus, cluster)
	ret0, _ := ret[0].(*framework.Result)
	return ret0
}

// Filter indicates an expected call of Filter.
func (mr *MockFilterPluginMockRecorder) Filter(ctx, bindingSpec, bindingStatus, cluster interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockFilterPlugin)(nil).Filter), ctx, bindingSpec, bindingStatus, cluster)
}

// Name mocks base method.
func (m *MockFilterPlugin) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockFilterPluginMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockFilterPlugin)(nil).Name))
}

// MockScorePlugin is a mock of ScorePlugin interface.
type MockScorePlugin struct {
	ctrl     *gomock.Controller
	recorder *MockScorePluginMockRecorder
}

// MockScorePluginMockRecorder is the mock recorder for MockScorePlugin.
type MockScorePluginMockRecorder struct {
	mock *MockScorePlugin
}

// NewMockScorePlugin creates a new mock instance.
func NewMockScorePlugin(ctrl *gomock.Controller) *MockScorePlugin {
	mock := &MockScorePlugin{ctrl: ctrl}
	mock.recorder = &MockScorePluginMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockScorePlugin) EXPECT() *MockScorePluginMockRecorder {
	return m.recorder
}

// Name mocks base method.
func (m *MockScorePlugin) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockScorePluginMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockScorePlugin)(nil).Name))
}

// Score mocks base method.
func (m *MockScorePlugin) Score(ctx context.Context, spec *v1alpha2.ResourceBindingSpec, clusters []*v1alpha1.Cluster, name string) (int64, *framework.Result) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Score", ctx, spec, clusters, name)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(*framework.Result)
	return ret0, ret1
}

// Score indicates an expected call of Score.
func (mr *MockScorePluginMockRecorder) Score(ctx, spec, clusters, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Score", reflect.TypeOf((*MockScorePlugin)(nil).Score), ctx, spec, clusters, name)
}

// ScoreExtensions mocks base method.
func (m *MockScorePlugin) ScoreExtensions() framework.ScoreExtensions {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScoreExtensions")
	ret0, _ := ret[0].(framework.ScoreExtensions)
	return ret0
}

// ScoreExtensions indicates an expected call of ScoreExtensions.
func (mr *MockScorePluginMockRecorder) ScoreExtensions() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScoreExtensions", reflect.TypeOf((*MockScorePlugin)(nil).ScoreExtensions))
}

// MockScoreExtensions is a mock of ScoreExtensions interface.
type MockScoreExtensions struct {
	ctrl     *gomock.Controller
	recorder *MockScoreExtensionsMockRecorder
}

// MockScoreExtensionsMockRecorder is the mock recorder for MockScoreExtensions.
type MockScoreExtensionsMockRecorder struct {
	mock *MockScoreExtensions
}

// NewMockScoreExtensions creates a new mock instance.
func NewMockScoreExtensions(ctrl *gomock.Controller) *MockScoreExtensions {
	mock := &MockScoreExtensions{ctrl: ctrl}
	mock.recorder = &MockScoreExtensionsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockScoreExtensions) EXPECT() *MockScoreExtensionsMockRecorder {
	return m.recorder
}

// NormalizeScore mocks base method.
func (m *MockScoreExtensions) NormalizeScore(ctx context.Context, scores framework.ClusterScoreList) *framework.Result {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NormalizeScore", ctx, scores)
	ret0, _ := ret[0].(*framework.Result)
	return ret0
}

// NormalizeScore indicates an expected call of NormalizeScore.
func (mr *MockScoreExtensionsMockRecorder) NormalizeScore(ctx, scores interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NormalizeScore", reflect.TypeOf((*MockScoreExtensions)(nil).NormalizeScore), ctx, scores)
}
