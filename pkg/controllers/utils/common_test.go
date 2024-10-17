/*
Copyright 2020 The Karmada Authors.

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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestUpdateFailoverStatus(t *testing.T) {
	tests := []struct {
		name         string
		binding      *workv1alpha2.ResourceBinding
		cluster      string
		failoverType workv1alpha2.FailoverReason
		wantErr      bool
	}{
		{
			name: "application failover",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
			},
			cluster:      "cluster1",
			failoverType: workv1alpha2.ApplicationFailover,
			wantErr:      false,
		},
		{
			name: "cluster failover",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
			},
			cluster:      "cluster2",
			failoverType: workv1alpha2.ClusterFailover,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build()

			err := UpdateFailoverStatus(fakeClient, tt.binding, tt.cluster, tt.failoverType)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
