/*
Copyright 2025 The Karmada Authors.

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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/util/flowcontrol"
)

func TestGetRateLimiterGetter(t *testing.T) {
	tests := []struct {
		name string
		want *ClusterRateLimiterGetter
	}{
		{
			name: "get the default singleton",
			want: defaultRateLimiterGetter,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, GetClusterRateLimiterGetter(), "GetClusterRateLimiterGetter()")
		})
	}
}

func TestRateLimiterGetter_GetRateLimiter(t *testing.T) {
	tests := []struct {
		name   string
		getter *ClusterRateLimiterGetter
		want   flowcontrol.RateLimiter
	}{
		{
			name:   "if qps/burst not set, use default value",
			getter: &ClusterRateLimiterGetter{},
			want:   flowcontrol.NewTokenBucketRateLimiter(40, 60),
		},
		{
			name: "SetDefaultLimits() should able to work",
			getter: func() *ClusterRateLimiterGetter {
				return (&ClusterRateLimiterGetter{}).SetDefaultLimits(100, 200)
			}(),
			want: flowcontrol.NewTokenBucketRateLimiter(100, 200),
		},
		{
			name: "if qps/burst invalid, use default value",
			getter: func() *ClusterRateLimiterGetter {
				return (&ClusterRateLimiterGetter{}).SetDefaultLimits(-1, -1)
			}(),
			want: flowcontrol.NewTokenBucketRateLimiter(40, 60),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.getter.GetRateLimiter("a"), "GetRateLimiter()")
		})
	}
}

func TestRateLimiterGetter_GetSameRateLimiter(t *testing.T) {
	type args struct {
		key1 string
		key2 string
	}
	tests := []struct {
		name            string
		args            args
		wantSameLimiter bool
	}{
		{
			name: "for a single cluster, the same limiter instance is obtained each time",
			args: args{
				key1: "a",
				key2: "a",
			},
			wantSameLimiter: true,
		},
		{
			name: "the rate limiter is independent for each cluster",
			args: args{
				key1: "a",
				key2: "b",
			},
			wantSameLimiter: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter := &ClusterRateLimiterGetter{}
			rl1 := getter.GetRateLimiter(tt.args.key1)
			rl2 := getter.GetRateLimiter(tt.args.key2)
			gotSameLimiter := rl1 == rl2
			assert.Equalf(t, tt.wantSameLimiter, gotSameLimiter,
				"key1: %v, key2: %v, wantSameLimiter: %v, rl1: %p, rl2: %p",
				tt.args.key1, tt.args.key2, tt.wantSameLimiter, rl1, rl2)
		})
	}
}
