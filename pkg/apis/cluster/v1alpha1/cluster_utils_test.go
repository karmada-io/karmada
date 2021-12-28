package v1alpha1

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestString(t *testing.T) {
	clusterName := "cluster1"
	cluster1 := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName},
	}

	tests := []struct {
		name    string
		fmtFunc func() string
		want    string
	}{
		{
			name: "%s pointer test",
			fmtFunc: func() string {
				return fmt.Sprintf("%s", cluster1) // nolint
			},
			want: clusterName,
		},
		{
			name: "%v pointer test",
			fmtFunc: func() string {
				return fmt.Sprintf("%v", cluster1)
			},
			want: clusterName,
		},
		{
			name: "%v pointer array test",
			fmtFunc: func() string {
				return fmt.Sprintf("%v", []*Cluster{cluster1})
			},
			want: fmt.Sprintf("[%s]", clusterName),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fmtFunc(); got != tt.want {
				t.Errorf("%s String() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
