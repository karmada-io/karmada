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
				destroyFunc: func(a any) {},
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
				destroyFunc: func(a any) { destroyed = true },
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
