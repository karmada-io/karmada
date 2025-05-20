/*
Copyright 2022 The Karmada Authors.

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

package helper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

func TestGetWorksByLabelsSet(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, workv1alpha1.Install(scheme))

	work1 := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work1",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}
	work2 := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work2",
			Labels: map[string]string{
				"app": "other",
			},
		},
	}

	tests := []struct {
		name      string
		works     []client.Object
		labels    labels.Set
		wantCount int
	}{
		{
			name:      "no works exist",
			works:     []client.Object{},
			labels:    labels.Set{"app": "test"},
			wantCount: 0,
		},
		{
			name:      "find work by label",
			works:     []client.Object{work1, work2},
			labels:    labels.Set{"app": "test"},
			wantCount: 1,
		},
		{
			name:      "find multiple works",
			works:     []client.Object{work1, work2},
			labels:    labels.Set{},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.works...).
				Build()

			workList, err := GetWorksByLabelsSet(context.TODO(), client, tt.labels)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(workList.Items))
		})
	}
}

func TestGetWorksByBindingID(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, workv1alpha1.Install(scheme))

	bindingID := "test-binding-id"
	workWithRBID := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workWithRBID",
			Labels: map[string]string{
				workv1alpha2.ResourceBindingPermanentIDLabel: bindingID,
			},
		},
	}
	workWithCRBID := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workWithCRBID",
			Labels: map[string]string{
				workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
			},
		},
	}

	tests := []struct {
		name                string
		works               []client.Object
		bindingID           string
		indexName           string
		permanentIDLabelKey string
		namespaced          bool
		wantWorks           []string
	}{
		{
			name:                "find namespaced binding works",
			works:               []client.Object{workWithRBID, workWithCRBID},
			bindingID:           bindingID,
			indexName:           indexregistry.WorkIndexByLabelResourceBindingID,
			permanentIDLabelKey: workv1alpha2.ResourceBindingPermanentIDLabel,
			namespaced:          true,
			wantWorks:           []string{"workWithRBID"},
		},
		{
			name:                "find cluster binding works",
			works:               []client.Object{workWithRBID, workWithCRBID},
			bindingID:           bindingID,
			indexName:           indexregistry.WorkIndexByLabelClusterResourceBindingID,
			permanentIDLabelKey: workv1alpha2.ClusterResourceBindingPermanentIDLabel,
			namespaced:          false,
			wantWorks:           []string{"workWithCRBID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(
					&workv1alpha1.Work{},
					tt.indexName,
					indexregistry.GenLabelIndexerFunc(tt.permanentIDLabelKey),
				).
				WithObjects(tt.works...).
				Build()

			workList, err := GetWorksByBindingID(context.TODO(), fakeClient, tt.bindingID, tt.namespaced)

			assert.NoError(t, err)
			assert.Equal(t, len(tt.wantWorks), len(workList.Items))

			// Verify the correct works were returned
			foundWorkNames := make([]string, len(workList.Items))
			for i, work := range workList.Items {
				foundWorkNames[i] = work.Name
			}
			assert.ElementsMatch(t, tt.wantWorks, foundWorkNames)
		})
	}
}
