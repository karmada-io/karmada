/*
Copyright 2024 The Karmada Authors.

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
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
)

type estimateReplicaResult struct {
	replica int32
	ret     *framework.Result
}

type estimateComponentsResult struct {
	sets int32
	ret  *framework.Result
}

type injectedResult struct {
	estimateReplicaResult    estimateReplicaResult
	estimateComponentsResult estimateComponentsResult
}

// TestPlugin implements all Plugin interfaces.
type TestPlugin struct {
	name string
	inj  injectedResult
}

func (pl *TestPlugin) Name() string {
	return pl.name
}

func (pl *TestPlugin) Estimate(_ context.Context, _ *schedcache.Snapshot, _ *pb.ReplicaRequirements) (int32, *framework.Result) {
	return pl.inj.estimateReplicaResult.replica, pl.inj.estimateReplicaResult.ret
}

func (pl *TestPlugin) EstimateComponents(_ context.Context, _ *schedcache.Snapshot, _ []pb.Component, _ string) (int32, *framework.Result) {
	return pl.inj.estimateComponentsResult.sets, pl.inj.estimateComponentsResult.ret
}

func Test_frameworkImpl_RunEstimateReplicasPlugins(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		plugins  []*TestPlugin
		expected estimateReplicaResult
	}{
		{
			name:    "no EstimateReplicasPlugins",
			plugins: []*TestPlugin{},
			expected: estimateReplicaResult{
				replica: math.MaxInt32,
				ret:     framework.NewResult(framework.Noopperation, "plugin results are empty"),
			},
		},
		{
			name: "one EstimateReplicasPlugin plugin returned success, but another EstimateReplicasPlugin plugin returned error",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 1,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "error",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							ret: framework.AsResult(fmt.Errorf("plugin 2 failed")),
						},
					},
				},
			},
			expected: estimateReplicaResult{
				replica: 0,
				ret:     framework.AsResult(fmt.Errorf("plugin 2 failed")),
			},
		},
		{
			name: "one EstimateReplicasPlugin plugin returned success, but another EstimateReplicasPlugin plugin returned unschedulable",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 1,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "unschedulable",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 0,
							ret:     framework.NewResult(framework.Unschedulable, "plugin 2 is unschedulable"),
						},
					},
				},
			},
			expected: estimateReplicaResult{
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "plugin 2 is unschedulable"),
			},
		},
		{
			name: "one EstimateReplicasPlugin plugin returned unschedulable, but another EstimateReplicasPlugin plugin returned noop",
			plugins: []*TestPlugin{
				{
					name: "unschedulable",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 0,
							ret:     framework.NewResult(framework.Unschedulable, "plugin 1 is unschedulable"),
						},
					},
				},
				{
					name: "noop",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: math.MaxInt32,
							ret:     framework.NewResult(framework.Noopperation, "plugin 2 is no operation"),
						},
					},
				},
			},
			expected: estimateReplicaResult{
				replica: 0,
				ret:     framework.NewResult(framework.Unschedulable, "plugin 1 is unschedulable", "plugin 2 is no operation"),
			},
		},
		{
			name: "one EstimateReplicasPlugin plugin returned success, but another EstimateReplicasPlugin plugin return no operation",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 1,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "noop",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							ret: framework.NewResult(framework.Noopperation, "plugin 2 is disabled"),
						},
					},
				},
			},
			expected: estimateReplicaResult{
				replica: 1,
				ret:     framework.NewResult(framework.Success, "plugin 2 is disabled"),
			},
		},
		{
			name: "all EstimateReplicasPlugins returned success and 1 replica",
			plugins: []*TestPlugin{
				{
					name: "success1",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 1,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "success2",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 1,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
			},
			expected: estimateReplicaResult{
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
		{
			name: "all EstimateReplicasPlugins returned success and but different replica",
			plugins: []*TestPlugin{
				{
					name: "success1",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 1,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "success2",
					inj: injectedResult{
						estimateReplicaResult: estimateReplicaResult{
							replica: 2,
							ret:     framework.NewResult(framework.Success),
						},
					},
				},
			},
			expected: estimateReplicaResult{
				replica: 1,
				ret:     framework.NewResult(framework.Success),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := make(Registry)
			for _, p := range tt.plugins {
				if err := r.Register(p.name, func(framework.Handle) (framework.Plugin, error) {
					return p, nil
				}); err != nil {
					t.Fatalf("fail to register PreScorePlugins plugin (%s)", p.Name())
				}
			}
			f, err := NewFramework(r)
			if err != nil {
				t.Errorf("create frame work error:%v", err)
			}
			replica, ret := f.RunEstimateReplicasPlugins(ctx, nil, &pb.ReplicaRequirements{})

			require.Equal(t, tt.expected.ret.Code(), ret.Code())
			assert.ElementsMatch(t, tt.expected.ret.Reasons(), ret.Reasons())
			require.Equal(t, tt.expected.replica, replica)
		})
	}
}

func Test_frameworkImpl_RunEstimateComponentsPlugins(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		plugins  []*TestPlugin
		expected estimateComponentsResult
	}{
		{
			name:    "no EstimateComponentsPlugins",
			plugins: []*TestPlugin{},
			expected: estimateComponentsResult{
				sets: math.MaxInt32,
				ret:  framework.NewResult(framework.Noopperation, "plugin results are empty"),
			},
		},
		{
			name: "one EstimateComponentsPlugin plugin returned success, but another EstimateComponentsPlugin plugin returned error",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 5,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "error",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							ret: framework.AsResult(fmt.Errorf("plugin 2 failed")),
						},
					},
				},
			},
			expected: estimateComponentsResult{
				sets: 5,
				ret:  framework.AsResult(fmt.Errorf("plugin 2 failed")),
			},
		},
		{
			name: "one EstimateComponentsPlugin plugin returned success, but another EstimateComponentsPlugin plugin returned unschedulable",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 10,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "unschedulable",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 0,
							ret:  framework.NewResult(framework.Unschedulable, "plugin 2 is unschedulable"),
						},
					},
				},
			},
			expected: estimateComponentsResult{
				sets: 0,
				ret:  framework.NewResult(framework.Unschedulable, "plugin 2 is unschedulable"),
			},
		},
		{
			name: "one EstimateComponentsPlugin plugin returned unschedulable, but another EstimateComponentsPlugin plugin returned noop",
			plugins: []*TestPlugin{
				{
					name: "unschedulable",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 0,
							ret:  framework.NewResult(framework.Unschedulable, "plugin 1 is unschedulable"),
						},
					},
				},
				{
					name: "noop",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: math.MaxInt32,
							ret:  framework.NewResult(framework.Noopperation, "plugin 2 is no operation"),
						},
					},
				},
			},
			expected: estimateComponentsResult{
				sets: 0,
				ret:  framework.NewResult(framework.Unschedulable, "plugin 1 is unschedulable", "plugin 2 is no operation"),
			},
		},
		{
			name: "one EstimateComponentsPlugin plugin returned success, but another EstimateComponentsPlugin plugin return no operation",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 3,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "noop",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							ret: framework.NewResult(framework.Noopperation, "plugin 2 is disabled"),
						},
					},
				},
			},
			expected: estimateComponentsResult{
				sets: 3,
				ret:  framework.NewResult(framework.Success, "plugin 2 is disabled"),
			},
		},
		{
			name: "all EstimateComponentsPlugins returned success with same sets",
			plugins: []*TestPlugin{
				{
					name: "success1",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 7,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "success2",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 7,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
			},
			expected: estimateComponentsResult{
				sets: 7,
				ret:  framework.NewResult(framework.Success),
			},
		},
		{
			name: "all EstimateComponentsPlugins returned success but different sets - should return minimum",
			plugins: []*TestPlugin{
				{
					name: "success1",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 5,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
				{
					name: "success2",
					inj: injectedResult{
						estimateComponentsResult: estimateComponentsResult{
							sets: 10,
							ret:  framework.NewResult(framework.Success),
						},
					},
				},
			},
			expected: estimateComponentsResult{
				sets: 5,
				ret:  framework.NewResult(framework.Success),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := make(Registry)
			for _, p := range tt.plugins {
				if err := r.Register(p.name, func(framework.Handle) (framework.Plugin, error) {
					return p, nil
				}); err != nil {
					t.Fatalf("fail to register EstimateComponentsPlugin plugin (%s)", p.Name())
				}
			}
			f, err := NewFramework(r)
			if err != nil {
				t.Errorf("create frame work error:%v", err)
			}
			sets, ret := f.RunEstimateComponentsPlugins(ctx, nil, []pb.Component{}, "")

			require.Equal(t, tt.expected.ret.Code(), ret.Code())
			assert.ElementsMatch(t, tt.expected.ret.Reasons(), ret.Reasons())
			require.Equal(t, tt.expected.sets, sets)
		})
	}
}
