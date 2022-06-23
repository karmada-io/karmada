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
		cluster         clusterapis.Cluster
		generateOptions printers.GenerateOptions
		expect          []metav1.TableRow
	}{
		// Test name, kubernetes version, sync mode, cluster ready status,
		{
			clusterapis.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec: clusterapis.ClusterSpec{
					SyncMode: clusterapis.Push,
				},
				Status: clusterapis.ClusterStatus{
					KubernetesVersion: "1.24.2",
					Conditions: []metav1.Condition{
						{Type: clusterapis.ClusterConditionReady, Status: metav1.ConditionTrue},
					},
				},
			},
			printers.GenerateOptions{Wide: false},
			[]metav1.TableRow{{Cells: []interface{}{"test1", "1.24.2", clusterapis.ClusterSyncMode("Push"), "True", "<unknown>"}}},
		},
		{
			clusterapis.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test2"},
				Spec: clusterapis.ClusterSpec{
					SyncMode:    clusterapis.Push,
					APIEndpoint: "https://kubernetes.default.svc.cluster.local:6443",
				},
				Status: clusterapis.ClusterStatus{
					KubernetesVersion: "1.24.2",
					Conditions: []metav1.Condition{
						{Type: clusterapis.ClusterConditionReady, Status: metav1.ConditionTrue},
					},
				},
			},
			printers.GenerateOptions{Wide: true},
			[]metav1.TableRow{{Cells: []interface{}{"test2", "1.24.2", clusterapis.ClusterSyncMode("Push"), "True", "<unknown>", "https://kubernetes.default.svc.cluster.local:6443"}}},
		},
	}

	for i, test := range tests {
		rows, err := printCluster(&test.cluster, test.generateOptions)
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
