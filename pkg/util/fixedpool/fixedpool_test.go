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

package fixedpool

import (
	"testing"
)

func TestFixedPool_Get(t *testing.T) {
	type fields struct {
		pool     []any
		capacity int
	}
	type want struct {
		len int
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "poll is empty",
			fields: fields{
				pool:     []any{},
				capacity: 3,
			},
			want: want{
				len: 0,
			},
		},
		{
			name: "poll is not empty",
			fields: fields{
				pool:     []any{1},
				capacity: 3,
			},
			want: want{
				len: 0,
			},
		},
		{
			name: "poll is full",
			fields: fields{
				pool:     []any{1, 2, 3},
				capacity: 3,
			},
			want: want{
				len: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &FixedPool{
				pool:        tt.fields.pool,
				capacity:    tt.fields.capacity,
				newFunc:     func() (any, error) { return &struct{}{}, nil },
				destroyFunc: func(any) {},
			}
			g, err := p.Get()
			if err != nil {
				t.Errorf("Get() returns error: %v", err)
				return
			}
			if g == nil {
				t.Errorf("Get() returns nil")
				return
			}
			if got := len(p.pool); got != tt.want.len {
				t.Errorf("Get() got = %v, want %v", got, tt.want.len)
			}
		})
	}
}

func TestFixedPool_Put(t *testing.T) {
	type fields struct {
		pool     []any
		capacity int
	}
	type want struct {
		len       int
		destroyed bool
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "pool is empty",
			fields: fields{
				pool:     nil,
				capacity: 3,
			},
			want: want{
				len:       1,
				destroyed: false,
			},
		},
		{
			name: "pool is not empty",
			fields: fields{
				pool:     []any{1},
				capacity: 3,
			},
			want: want{
				len:       2,
				destroyed: false,
			},
		},
		{
			name: "pool is not full",
			fields: fields{
				pool:     []any{1, 2, 3},
				capacity: 3,
			},
			want: want{
				len:       3,
				destroyed: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destroyed := false
			p := &FixedPool{
				pool:        tt.fields.pool,
				capacity:    tt.fields.capacity,
				newFunc:     func() (any, error) { return &struct{}{}, nil },
				destroyFunc: func(any) { destroyed = true },
			}
			p.Put(&struct{}{})
			if got := len(p.pool); got != tt.want.len {
				t.Errorf("pool len got %v, want %v", got, tt.want)
			}
			if destroyed != tt.want.destroyed {
				t.Errorf("destroyed got %v, want %v", destroyed, tt.want.destroyed)
			}
		})
	}
}

func TestNewFixedPool(t *testing.T) {
	tests := []struct {
		name        string
		newFunc     func() (any, error)
		destroyFunc func(any)
		capacity    int
		wantName    string
		wantCap     int
	}{
		{
			name:        "create pool with capacity 3",
			newFunc:     func() (any, error) { return &struct{}{}, nil },
			destroyFunc: func(any) {},
			capacity:    3,
			wantName:    "testPool",
			wantCap:     3,
		},
		{
			name:        "create pool with capacity 0",
			newFunc:     func() (any, error) { return &struct{}{}, nil },
			destroyFunc: func(any) {},
			capacity:    0,
			wantName:    "emptyPool",
			wantCap:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.wantName, tt.newFunc, tt.destroyFunc, tt.capacity)

			if p.name != tt.wantName {
				t.Errorf("New() name got = %v, want %v", p.name, tt.wantName)
			}
			if p.capacity != tt.wantCap {
				t.Errorf("New() capacity got = %v, want %v", p.capacity, tt.wantCap)
			}
			if got := len(p.pool); got != 0 {
				t.Errorf("New() pool length got = %v, want 0", got)
			}
		})
	}
}
