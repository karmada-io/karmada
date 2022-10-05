package cache

import (
	"testing"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSnapshotGetCluster(t *testing.T) {
	clusters := []*v1alpha1.Cluster{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "test",
			},
		},
	}
	snapshot := NewSnapshotFromClusters(clusters)

	tests := []struct {
		expected string
	}{
		{
			"test",
		},
	}

	for i := 0; i < len(tests); i++ {
		got := snapshot.GetCluster(clusters[i].Name).Cluster().Name
		if got != tests[i].expected {
			t.Errorf("wrong cluster snapshot,expected:%v, but got %v", tests[i].expected, got)
		}
	}
}
