/*
Copyright 2021 The Karmada Authors.

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
					ProxyURL:    "https://anp-server.default.svc.cluster.local:443",
					Zones:       []string{"foo", "bar"},
				},
				Status: clusterapis.ClusterStatus{
					KubernetesVersion: "1.24.2",
					Conditions: []metav1.Condition{
						{Type: clusterapis.ClusterConditionReady, Status: metav1.ConditionTrue},
					},
				},
			},
			printers.GenerateOptions{Wide: true},
			[]metav1.TableRow{{Cells: []interface{}{"test2", "1.24.2", clusterapis.ClusterSyncMode("Push"), "True", "<unknown>",
				"foo,bar",
				"<none>",
				"<none>",
				"https://kubernetes.default.svc.cluster.local:6443",
				"https://anp-server.default.svc.cluster.local:443"}}},
		},
	}

	for i := range tests {
		rows, err := printCluster(&tests[i].cluster, tests[i].generateOptions)
		if err != nil {
			t.Fatal(err)
		}
		for i := range rows {
			rows[i].Object.Object = nil
		}
		if !reflect.DeepEqual(tests[i].expect, rows) {
			t.Errorf("%d mismatch: %s", i, diff.ObjectReflectDiff(tests[i].expect, rows))
		}
	}
}
