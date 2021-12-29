package internalversion

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/printers"
)

func TestPrintCluster(t *testing.T) {
	tests := []struct {
		cluster clusterapis.Cluster
		expect  []metav1.TableRow
	}{
		// Test name, kubernetes version, sync mode, cluster ready status,
		{
			clusterapis.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec: clusterapis.ClusterSpec{
					SyncMode: clusterapis.Push,
				},
				Status: clusterapis.ClusterStatus{
					KubernetesVersion: "1.21.7",
					Conditions: []metav1.Condition{
						{Type: clusterapis.ClusterConditionReady, Status: metav1.ConditionTrue},
					},
				},
			},
			[]metav1.TableRow{{Cells: []interface{}{"test1", "1.21.7", clusterapis.ClusterSyncMode("Push"), "True", "<unknown>"}}},
		},
	}

	for i, test := range tests {
		rows, err := printCluster(&test.cluster, printers.GenerateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for i := range rows {
			rows[i].Object.Object = nil
		}
		if !reflect.DeepEqual(test.expect, rows) {
			t.Errorf("%d mismatch: %s", i, diff.ObjectReflectDiff(test.expect, rows))
		}
	}
}
