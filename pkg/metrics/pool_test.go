package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordPoolGet(t *testing.T) {
	type args struct {
		name    string
		created bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "get from pool",
			args: args{
				name:    "foo",
				created: false,
			},
			want: `
# HELP pool_get_operation_total Total times of getting from pool
# TYPE pool_get_operation_total counter
pool_get_operation_total{from="pool",name="foo"} 1
`,
		},
		{
			name: "get from new",
			args: args{
				name:    "foo",
				created: true,
			},
			want: `
# HELP pool_get_operation_total Total times of getting from pool
# TYPE pool_get_operation_total counter
pool_get_operation_total{from="new",name="foo"} 1
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poolGetCounter.Reset()
			RecordPoolGet(tt.args.name, tt.args.created)
			if err := testutil.CollectAndCompare(poolGetCounter, strings.NewReader(tt.want), poolGetCounterMetricsName); err != nil {
				t.Errorf("unexpected collecting result:\n%s", err)
			}
		})
	}
}

func TestRecordPoolPut(t *testing.T) {
	type args struct {
		name      string
		destroyed bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "put to pool",
			args: args{
				name:      "foo",
				destroyed: false,
			},
			want: `
# HELP pool_put_operation_total Total times of putting from pool
# TYPE pool_put_operation_total counter
pool_put_operation_total{name="foo",to="pool"} 1
`,
		},
		{
			name: "put to destroyed",
			args: args{
				name:      "foo",
				destroyed: true,
			},
			want: `
# HELP pool_put_operation_total Total times of putting from pool
# TYPE pool_put_operation_total counter
pool_put_operation_total{name="foo",to="destroyed"} 1
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poolPutCounter.Reset()
			RecordPoolPut(tt.args.name, tt.args.destroyed)
			if err := testutil.CollectAndCompare(poolPutCounter, strings.NewReader(tt.want), poolPutCounterMetricsName); err != nil {
				t.Errorf("unexpected collecting result:\n%s", err)
			}
		})
	}
}
