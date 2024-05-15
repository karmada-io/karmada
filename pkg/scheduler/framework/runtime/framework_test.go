/*
Copyright 2022 The Karmada Authors.

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

package runtime

import (
	"context"
	"strconv"
	"testing"

	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	frameworktesting "github.com/karmada-io/karmada/pkg/scheduler/framework/testing"
)

func Test_frameworkImpl_RunFilterPlugins(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	alwaysError := frameworktesting.NewMockFilterPlugin(mockCtrl)
	alwaysError.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
		Return(framework.NewResult(framework.Error, "foo"))
	alwaysError.EXPECT().Name().AnyTimes().Return("foo")

	alwaysSuccess := frameworktesting.NewMockFilterPlugin(mockCtrl)
	alwaysSuccess.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
		Return(framework.NewResult(framework.Success))
	alwaysSuccess.EXPECT().Name().AnyTimes().Return("foo")

	tests := []struct {
		name      string
		plugins   []framework.Plugin
		isSuccess bool
	}{
		{
			name:      "no filter plugin",
			plugins:   []framework.Plugin{},
			isSuccess: true,
		},
		{
			name:      "error filter plugin",
			plugins:   []framework.Plugin{alwaysError},
			isSuccess: false,
		},
		{
			name:      "success filter plugin",
			plugins:   []framework.Plugin{alwaysSuccess},
			isSuccess: true,
		},
		{
			name:      "error and success filter plugins",
			plugins:   []framework.Plugin{alwaysError, alwaysSuccess},
			isSuccess: false,
		},
		{
			name:      "success and error filter plugins",
			plugins:   []framework.Plugin{alwaysSuccess, alwaysError},
			isSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := createAndRegisterFactory(tt.plugins)
			if err != nil {
				t.Errorf("create plugin factory error:%v", err)
			}

			frameWork, err := NewFramework(registry)
			if err != nil {
				t.Errorf("create frame work error:%v", err)
			}

			result := frameWork.RunFilterPlugins(ctx, nil, nil, nil)
			if result.IsSuccess() != tt.isSuccess {
				t.Errorf("want %v, but get:%v", tt.isSuccess, result.IsSuccess())
			}
		})
	}
}

func createAndRegisterFactory(plugins []framework.Plugin) (Registry, error) {
	registry := Registry{}
	for i, plugin := range plugins {
		pluginCopy := plugin
		filterPluginFactory := func() (framework.Plugin, error) {
			return pluginCopy, nil
		}
		if err := registry.Register("foo"+strconv.Itoa(i), filterPluginFactory); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

func Test_frameworkImpl_RunScorePlugins(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clusters := []*clusterv1alpha1.Cluster{
		{ObjectMeta: metav1.ObjectMeta{Name: "c1"}},
	}
	tests := []struct {
		name      string
		mockFunc  func(mockScorePlugin *frameworktesting.MockScorePlugin, mockScoreExtension *frameworktesting.MockScoreExtensions)
		isSuccess bool
	}{
		{
			name: "Test score ok",
			mockFunc: func(mockScorePlugin *frameworktesting.MockScorePlugin, mockScoreExtension *frameworktesting.MockScoreExtensions) {
				mockScorePlugin.EXPECT().Score(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(60), framework.NewResult(framework.Success))
				mockScorePlugin.EXPECT().ScoreExtensions().Times(2).Return(mockScoreExtension)
				mockScorePlugin.EXPECT().Name().AnyTimes().Return("foo")

				mockScoreExtension.EXPECT().NormalizeScore(gomock.Any(), gomock.Any()).
					Return(framework.NewResult(framework.Success))
			},
			isSuccess: true,
		},
		{
			name: "Test score func error",
			mockFunc: func(mockScorePlugin *frameworktesting.MockScorePlugin, _ *frameworktesting.MockScoreExtensions) {
				mockScorePlugin.EXPECT().Score(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(-1), framework.NewResult(framework.Error, "foo"))
				mockScorePlugin.EXPECT().Name().AnyTimes().Return("foo")
			},
			isSuccess: false,
		},
		{
			name: "Test normalize score error",
			mockFunc: func(mockScorePlugin *frameworktesting.MockScorePlugin, mockScoreExtension *frameworktesting.MockScoreExtensions) {
				mockScorePlugin.EXPECT().Score(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(60), framework.NewResult(framework.Success))
				mockScorePlugin.EXPECT().ScoreExtensions().Times(2).Return(mockScoreExtension)
				mockScorePlugin.EXPECT().Name().AnyTimes().Return("foo")

				mockScoreExtension.EXPECT().NormalizeScore(gomock.Any(), gomock.Any()).
					Return(framework.NewResult(framework.Error, "foo"))
			},
			isSuccess: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockScorePlugin := frameworktesting.NewMockScorePlugin(mockCtrl)
			mockScoreExtension := frameworktesting.NewMockScoreExtensions(mockCtrl)

			registry, err := createAndRegisterFactory([]framework.Plugin{mockScorePlugin})
			if err != nil {
				t.Errorf("create plugin factory error:%v", err)
			}

			frameWork, err := NewFramework(registry)
			if err != nil {
				t.Errorf("create frame work error:%v", err)
			}

			tt.mockFunc(mockScorePlugin, mockScoreExtension)
			_, result := frameWork.RunScorePlugins(ctx, nil, clusters)
			if result.IsSuccess() != tt.isSuccess {
				t.Errorf("want %v, but get:%v", tt.isSuccess, result.IsSuccess())
			}
		})
	}
}
