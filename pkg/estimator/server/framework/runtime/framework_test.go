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

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
)

var (
	errEstimateReplica = fmt.Errorf("estimate failed")
)

type estimateReplicaResult struct {
	replica int32
	err     error
}

type injectedResult struct {
	estimateReplicaResult estimateReplicaResult
}

// TestPlugin implements all Plugin interfaces.
type TestPlugin struct {
	name string
	inj  injectedResult
}

func (pl *TestPlugin) Name() string {
	return pl.name
}

func (pl *TestPlugin) Estimate(_ context.Context, _ *pb.ReplicaRequirements) (int32, error) {
	return pl.inj.estimateReplicaResult.replica, pl.inj.estimateReplicaResult.err
}

func Test_frameworkImpl_RunEstimateReplicasPlugins(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		plugins  []*TestPlugin
		expected estimateReplicaResult
	}{
		{
			name:     "no EstimateReplicasPlugins",
			plugins:  []*TestPlugin{},
			expected: estimateReplicaResult{replica: math.MaxInt32},
		},
		{
			name: "all EstimateReplicasPlugins returned success and zero replica",
			plugins: []*TestPlugin{
				{
					name: "success1",
					inj: injectedResult{
						estimateReplicaResult{},
					},
				},
				{
					name: "success2",
					inj: injectedResult{
						estimateReplicaResult{},
					},
				},
			},
			expected: estimateReplicaResult{},
		},
		{
			name: "one PreScore plugin returned success, but another PreScore plugin returned error",
			plugins: []*TestPlugin{
				{
					name: "success",
					inj: injectedResult{
						estimateReplicaResult{},
					},
				},
				{
					name: "error",
					inj: injectedResult{
						estimateReplicaResult{
							err: errEstimateReplica,
						},
					},
				},
			},
			expected: estimateReplicaResult{err: errEstimateReplica},
		},
		{
			name: "all EstimateReplicasPlugins returned success and but different replica",
			plugins: []*TestPlugin{
				{
					name: "success1",
					inj: injectedResult{
						estimateReplicaResult{replica: 0},
					},
				},
				{
					name: "success2",
					inj: injectedResult{
						estimateReplicaResult{replica: math.MaxInt32},
					},
				},
			},
			expected: estimateReplicaResult{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := make(Registry)
			for _, p := range tt.plugins {
				p := p
				if err := r.Register(p.name, func(fh framework.Handle) (framework.Plugin, error) {
					return p, nil
				}); err != nil {
					t.Fatalf("fail to register PreScorePlugins plugin (%s)", p.Name())
				}
			}
			f, err := NewFramework(r)
			if err != nil {
				t.Errorf("create frame work error:%v", err)
			}
			ret, err := f.RunEstimateReplicasPlugins(ctx, &pb.ReplicaRequirements{})
			if err != tt.expected.err {
				t.Errorf("wrong error. got: %v, expect: %v", err, tt.expected.err)
			}

			if ret != tt.expected.replica {
				t.Errorf("wrong estimated replica. got: %v, expect: %v", ret, tt.expected.replica)
			}
		})
	}
}
