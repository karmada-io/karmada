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

package ratelimiterflag

import (
	"reflect"
	"testing"
	"time"
)

func TestOptions_SetDefaults(t *testing.T) {
	type fields struct {
		RateLimiterBaseDelay  time.Duration
		RateLimiterMaxDelay   time.Duration
		RateLimiterQPS        int
		RateLimiterBucketSize int
	}
	tests := []struct {
		name   string
		fields fields
		want   *Options
	}{
		{name: "all zero -> defaults", fields: fields{}, want: &Options{RateLimiterBaseDelay: 5 * time.Millisecond, RateLimiterMaxDelay: 1000 * time.Second, RateLimiterQPS: 10, RateLimiterBucketSize: 100}},
		{name: "all negative -> defaults", fields: fields{RateLimiterBaseDelay: -1, RateLimiterMaxDelay: -1, RateLimiterQPS: -1, RateLimiterBucketSize: -1}, want: &Options{RateLimiterBaseDelay: 5 * time.Millisecond, RateLimiterMaxDelay: 1000 * time.Second, RateLimiterQPS: 10, RateLimiterBucketSize: 100}},
		{name: "only base delay invalid", fields: fields{RateLimiterBaseDelay: 0, RateLimiterMaxDelay: 2 * time.Second, RateLimiterQPS: 20, RateLimiterBucketSize: 200}, want: &Options{RateLimiterBaseDelay: 5 * time.Millisecond, RateLimiterMaxDelay: 2 * time.Second, RateLimiterQPS: 20, RateLimiterBucketSize: 200}},
		{name: "custom values unchanged", fields: fields{RateLimiterBaseDelay: 1 * time.Second, RateLimiterMaxDelay: 2 * time.Second, RateLimiterQPS: 50, RateLimiterBucketSize: 500}, want: &Options{RateLimiterBaseDelay: 1 * time.Second, RateLimiterMaxDelay: 2 * time.Second, RateLimiterQPS: 50, RateLimiterBucketSize: 500}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Options{
				RateLimiterBaseDelay:  tt.fields.RateLimiterBaseDelay,
				RateLimiterMaxDelay:   tt.fields.RateLimiterMaxDelay,
				RateLimiterQPS:        tt.fields.RateLimiterQPS,
				RateLimiterBucketSize: tt.fields.RateLimiterBucketSize,
			}
			if got := o.SetDefaults(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetDefaults() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("receiver is changed", func(t *testing.T) {
		o := &Options{}
		o.SetDefaults()
		want := &Options{RateLimiterBaseDelay: 5 * time.Millisecond, RateLimiterMaxDelay: 1000 * time.Second, RateLimiterQPS: 10, RateLimiterBucketSize: 100}
		if !reflect.DeepEqual(o, want) {
			t.Errorf("SetDefaults() changed self to %v, want %v", o, want)
		}
	})
}
