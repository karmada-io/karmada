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

package lifted

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"

	"github.com/karmada-io/karmada/pkg/printers"
)

func TestPrintCoreV1(t *testing.T) {
	testCases := []struct {
		pod             corev1.Pod
		generateOptions printers.GenerateOptions
		expect          []metav1.TableRow
	}{
		// Test name, kubernetes version, sync mode, cluster ready status,
		{
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec:       corev1.PodSpec{},
				Status: corev1.PodStatus{
					Phase:  corev1.PodPending,
					HostIP: "1.2.3.4",
					PodIP:  "2.3.4.5",
				},
			},
			printers.GenerateOptions{Wide: false},
			[]metav1.TableRow{{Cells: []interface{}{"test1", "0/0", "Pending", int64(0), "<unknown>"}}},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("pod=%s, expected value=%v", tc.pod.Name, tc.expect), func(t *testing.T) {
			rows, err := printPod(&tc.pod, tc.generateOptions)
			if err != nil {
				t.Fatal(err)
			}
			for i := range rows {
				rows[i].Object.Object = nil
			}
			if !reflect.DeepEqual(tc.expect, rows) {
				t.Errorf("%d mismatch: %s", i, diff.ObjectReflectDiff(tc.expect, rows))
			}
		})
	}
}
