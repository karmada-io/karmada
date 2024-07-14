/*
Copyright 2024 The Karmada Authors.

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

package storage

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/karmada-io/karmada/pkg/printers"
	printersinternal "github.com/karmada-io/karmada/pkg/util/lifted"
)

func TestPrintHandlerStorage(t *testing.T) {
	convert := NewTableConvertor(
		rest.NewDefaultTableConvertor(schema.GroupResource{}),
		printers.NewTableGenerator().With(printersinternal.AddCoreV1Handlers))
	testCases := []struct {
		object runtime.Object
	}{
		{
			object: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test1"},
				Spec:       v1.PodSpec{},
				Status: v1.PodStatus{
					Phase:  v1.PodPending,
					HostIP: "1.2.3.4",
					PodIP:  "2.3.4.5",
				},
			},
		},
		{
			object: &v1.Node{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Spec:       v1.NodeSpec{},
				Status: v1.NodeStatus{
					Phase: v1.NodeTerminated,
					Addresses: []v1.NodeAddress{
						{
							Type:    v1.NodeInternalIP,
							Address: "1.2.3.4",
						},
					},
				},
			},
		},
		{
			object: &v1.Service{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{Name: "svc1"},
				Spec: v1.ServiceSpec{
					Selector:  nil,
					ClusterIP: "4.4.4.4",
					Type:      v1.ServiceTypeClusterIP,
				},
			},
		},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("object=%#v", tc.object), func(t *testing.T) {
			_, err := convert.ConvertToTable(context.Background(), tc.object, nil)
			if err != nil {
				t.Errorf("index %v ConvertToTable error: %#v", i, err)
			}
		})
	}
}
