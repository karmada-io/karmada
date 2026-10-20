/*
Copyright 2026 The Karmada Authors.

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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestDeleteWorks(t *testing.T) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = workv1alpha1.Install(s)
	_ = workv1alpha2.Install(s)
	_ = clusterv1alpha1.Install(s)

	bindingID := "binding-id-123"
	expectedWorkName := names.GenerateWorkName("ConfigMap", "test-cm", "default")
	work1Namespace := names.GenerateExecutionSpaceName(ClusterMember1)
	work2Namespace := names.GenerateExecutionSpaceName(ClusterMember2)

	newWork := func(namespace, uid, labelKey, id string) *workv1alpha1.Work {
		return &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:      expectedWorkName,
				Namespace: namespace,
				Labels:    map[string]string{labelKey: id},
				UID:       types.UID(uid),
			},
		}
	}
	work1 := newWork(work1Namespace, "work1-uid", workv1alpha2.ResourceBindingPermanentIDLabel, bindingID)
	work2 := newWork(work2Namespace, "work2-uid", workv1alpha2.ResourceBindingPermanentIDLabel, bindingID)
	allClusters := sets.New(ClusterMember1, ClusterMember2)
	// Every case runs against these registered clusters, so a fleet-wide scan would show up as extra lookups.
	registeredClusters := []client.Object{
		&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: ClusterMember1}},
		&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: ClusterMember2}},
		&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: ClusterMember3}},
	}

	tests := []struct {
		name              string
		isCRB             bool
		candidateClusters sets.Set[string]
		cachedWorks       []*workv1alpha1.Work
		apiWorks          []*workv1alpha1.Work
		apiReaderError    error
		workDeletionError error
		expectedError     bool
		// namespaces of Works expected to remain in the API server after DeleteWorks
		expectedRemaining []string
		// namespaces expected to be looked up via the APIReader
		expectedLookups []string
	}{
		{
			name:              "cache finds all Works -> deleted without APIReader lookups",
			candidateClusters: allClusters,
			cachedWorks:       []*workv1alpha1.Work{work1, work2},
			apiWorks:          []*workv1alpha1.Work{work1, work2},
		},
		{
			name:              "cache misses member2 -> APIReader fallback deletes it",
			candidateClusters: allClusters,
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks:          []*workv1alpha1.Work{work1, work2},
			expectedLookups:   []string{work2Namespace},
		},
		{
			name:              "cache finds none -> APIReader fallback deletes both",
			candidateClusters: allClusters,
			apiWorks:          []*workv1alpha1.Work{work1, work2},
			expectedLookups:   []string{work1Namespace, work2Namespace},
		},
		{
			name:              "APIReader returns NotFound -> cleanup continues",
			candidateClusters: allClusters,
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks:          []*workv1alpha1.Work{work1},
			expectedLookups:   []string{work2Namespace},
		},
		{
			name:              "APIReader returns an actual error -> finalizer is NOT removed",
			candidateClusters: allClusters,
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks:          []*workv1alpha1.Work{work1, work2},
			apiReaderError:    errors.New("apiserver unavailable"),
			expectedError:     true,
			expectedRemaining: []string{work2Namespace},
			expectedLookups:   []string{work2Namespace},
		},
		{
			name:              "Work deletion fails -> finalizer is NOT removed",
			candidateClusters: allClusters,
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks:          []*workv1alpha1.Work{work1, work2},
			workDeletionError: errors.New("webhook denied"),
			expectedError:     true,
			expectedRemaining: []string{work1Namespace, work2Namespace},
			expectedLookups:   []string{work2Namespace},
		},
		{
			name:              "APIReader finds Work with different permanent ID -> it is NOT deleted",
			candidateClusters: allClusters,
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks: []*workv1alpha1.Work{work1,
				newWork(work2Namespace, "other-uid", workv1alpha2.ResourceBindingPermanentIDLabel, "some-other-id")},
			expectedRemaining: []string{work2Namespace},
			expectedLookups:   []string{work2Namespace},
		},
		{
			name:              "cluster not held by the binding -> APIReader lookup skipped",
			candidateClusters: sets.New(ClusterMember1),
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks:          []*workv1alpha1.Work{work1, work2},
			expectedRemaining: []string{work2Namespace},
		},
		{
			name:              "no candidate clusters -> no APIReader lookups, only cached Works are deleted",
			cachedWorks:       []*workv1alpha1.Work{work1},
			apiWorks:          []*workv1alpha1.Work{work1, work2},
			expectedRemaining: []string{work2Namespace},
		},
		{
			name:              "ClusterResourceBinding checks its own label",
			isCRB:             true,
			candidateClusters: allClusters,
			cachedWorks: []*workv1alpha1.Work{
				newWork(work1Namespace, "work1-uid", workv1alpha2.ClusterResourceBindingPermanentIDLabel, bindingID),
			},
			apiWorks: []*workv1alpha1.Work{
				newWork(work1Namespace, "work1-uid", workv1alpha2.ClusterResourceBindingPermanentIDLabel, bindingID),
				newWork(work2Namespace, "work2-uid", workv1alpha2.ClusterResourceBindingPermanentIDLabel, bindingID),
			},
			expectedLookups: []string{work2Namespace},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lookups []string

			var apiObjs []client.Object
			for _, work := range tt.apiWorks {
				apiObjs = append(apiObjs, work.DeepCopy())
			}
			apiClient := fake.NewClientBuilder().WithScheme(s).WithObjects(apiObjs...).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						lookups = append(lookups, key.Namespace)
						if tt.apiReaderError != nil {
							return tt.apiReaderError
						}
						return c.Get(ctx, key, obj, opts...)
					},
				}).Build()

			cacheObjs := append([]client.Object{}, registeredClusters...)
			for _, work := range tt.cachedWorks {
				cacheObjs = append(cacheObjs, work.DeepCopy())
			}
			cacheClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cacheObjs...).
				WithIndex(&workv1alpha1.Work{}, indexregistry.WorkIndexByLabelResourceBindingID, func(o client.Object) []string {
					val, ok := o.GetLabels()[workv1alpha2.ResourceBindingPermanentIDLabel]
					if !ok {
						return nil
					}
					return []string{val}
				}).
				WithIndex(&workv1alpha1.Work{}, indexregistry.WorkIndexByLabelClusterResourceBindingID, func(o client.Object) []string {
					val, ok := o.GetLabels()[workv1alpha2.ClusterResourceBindingPermanentIDLabel]
					if !ok {
						return nil
					}
					return []string{val}
				}).
				WithInterceptorFuncs(interceptor.Funcs{
					// Reads are served from the stale cache, but writes go to the API server.
					Delete: func(ctx context.Context, _ client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						if tt.workDeletionError != nil {
							return tt.workDeletionError
						}
						return apiClient.Delete(ctx, obj, opts...)
					},
				}).Build()

			namespace := "default"
			if tt.isCRB {
				namespace = ""
			}

			err := DeleteWorks(context.TODO(), cacheClient, apiClient, namespace, "binding-test", bindingID, expectedWorkName, tt.candidateClusters)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			remaining := &workv1alpha1.WorkList{}
			assert.NoError(t, apiClient.List(context.TODO(), remaining))
			var remainingNamespaces []string
			for _, work := range remaining.Items {
				remainingNamespaces = append(remainingNamespaces, work.Namespace)
			}
			assert.ElementsMatch(t, tt.expectedRemaining, remainingNamespaces)
			assert.ElementsMatch(t, tt.expectedLookups, lookups)
		})
	}
}
