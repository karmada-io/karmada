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

package helper

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	resourceKind := "ConfigMap"
	resourceName := "test-cm"
	resourceNamespace := "default"

	expectedWorkName := names.GenerateWorkName(resourceKind, resourceName, resourceNamespace)
	work1Namespace := names.GenerateExecutionSpaceName(ClusterMember1)
	work2Namespace := names.GenerateExecutionSpaceName(ClusterMember2)

	work1 := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedWorkName,
			Namespace: work1Namespace,
			Labels: map[string]string{
				workv1alpha2.ResourceBindingPermanentIDLabel: bindingID,
			},
			UID: "work1-uid",
		},
	}

	work2 := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedWorkName,
			Namespace: work2Namespace,
			Labels: map[string]string{
				workv1alpha2.ResourceBindingPermanentIDLabel: bindingID,
			},
			UID: "work2-uid",
		},
	}

	cluster1 := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: ClusterMember1},
	}
	cluster2 := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: ClusterMember2},
	}

	tests := []struct {
		name               string
		isCRB              bool
		cachedClusters     []client.Object
		cachedWorks        []client.Object
		apiWorks           []client.Object
		apiReaderError     error
		workDeletionError  error
		expectedError      bool
		expectWork2Deleted bool
	}{
		{
			name:               "cache finds all Works -> APIReader lists and finds nothing extra",
			cachedClusters:     []client.Object{cluster1, cluster2},
			cachedWorks:        []client.Object{work1, work2},
			apiWorks:           []client.Object{work1, work2},
			expectedError:      false,
			expectWork2Deleted: true,
		},
		{
			name:               "cache misses member2 -> APIReader fallback finds and deletes it",
			cachedClusters:     []client.Object{cluster1, cluster2},
			cachedWorks:        []client.Object{work1},
			apiWorks:           []client.Object{work1, work2},
			expectedError:      false,
			expectWork2Deleted: true,
		},
		{
			name:               "cache finds none -> APIReader fallback finds and deletes both",
			cachedClusters:     []client.Object{cluster1, cluster2},
			cachedWorks:        []client.Object{},
			apiWorks:           []client.Object{work1, work2},
			expectedError:      false,
			expectWork2Deleted: true,
		},
		{
			name:               "APIReader returns NotFound -> cleanup continues",
			cachedClusters:     []client.Object{cluster1, cluster2},
			cachedWorks:        []client.Object{work1},
			apiWorks:           []client.Object{work1},
			expectedError:      false,
			expectWork2Deleted: false,
		},
		{
			name:               "APIReader returns an actual error -> finalizer is NOT removed",
			cachedClusters:     []client.Object{cluster1, cluster2},
			cachedWorks:        []client.Object{work1},
			apiWorks:           []client.Object{work1, work2},
			apiReaderError:     errors.New("apiserver unavailable"),
			expectedError:      true,
			expectWork2Deleted: false,
		},
		{
			name:               "Work deletion fails -> finalizer is NOT removed",
			cachedClusters:     []client.Object{cluster1, cluster2},
			cachedWorks:        []client.Object{work1},
			apiWorks:           []client.Object{work1, work2},
			workDeletionError:  errors.New("webhook denied"),
			expectedError:      true,
			expectWork2Deleted: false,
		},
		{
			name:           "APIReader finds Work with different permanent ID -> it is NOT deleted",
			cachedClusters: []client.Object{cluster1, cluster2},
			cachedWorks:    []client.Object{work1},
			apiWorks: []client.Object{work1, &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedWorkName,
					Namespace: work2Namespace,
					Labels: map[string]string{
						workv1alpha2.ResourceBindingPermanentIDLabel: "some-other-id",
					},
					UID: "wrong-id-uid",
				},
			}},
			expectedError:      false,
			expectWork2Deleted: false,
		},
		{
			name:               "Cluster missing from cache -> APIReader Get skipped",
			cachedClusters:     []client.Object{cluster1},
			cachedWorks:        []client.Object{work1},
			apiWorks:           []client.Object{work1, work2},
			expectedError:      false,
			expectWork2Deleted: false, // member2 is not in cache, so we skip reading it
		},
		{
			name:           "ClusterResourceBinding checks its own label",
			isCRB:          true,
			cachedClusters: []client.Object{cluster1, cluster2},
			cachedWorks: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedWorkName,
						Namespace: work1Namespace,
						Labels: map[string]string{
							workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
						},
						UID: "work1-uid",
					},
				},
			},
			apiWorks: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedWorkName,
						Namespace: work1Namespace,
						Labels: map[string]string{
							workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
						},
						UID: "work1-uid",
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedWorkName,
						Namespace: work2Namespace,
						Labels: map[string]string{
							workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
						},
						UID: "work2-uid",
					},
				},
			},
			expectedError:      false,
			expectWork2Deleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deletedWorks []string

			// Cache Client setup
			var cacheObjs []client.Object
			cacheObjs = append(cacheObjs, tt.cachedClusters...)
			cacheObjs = append(cacheObjs, tt.cachedWorks...)

			cacheClient := fake.NewClientBuilder().WithScheme(s).WithObjects(cacheObjs...).
				WithIndex(&workv1alpha1.Work{}, indexregistry.WorkIndexByLabelResourceBindingID, func(o client.Object) []string {
					work := o.(*workv1alpha1.Work)
					val, ok := work.Labels[workv1alpha2.ResourceBindingPermanentIDLabel]
					if !ok {
						return nil
					}
					return []string{val}
				}).
				WithIndex(&workv1alpha1.Work{}, indexregistry.WorkIndexByLabelClusterResourceBindingID, func(o client.Object) []string {
					work := o.(*workv1alpha1.Work)
					val, ok := work.Labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel]
					if !ok {
						return nil
					}
					return []string{val}
				}).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						if tt.workDeletionError != nil {
							return tt.workDeletionError
						}
						deletedWorks = append(deletedWorks, obj.GetNamespace()+"/"+obj.GetName())
						return c.Delete(ctx, obj, opts...)
					},
				}).Build()

			// API Reader setup
			apiReaderBuilder := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.apiWorks...)
			funcs := interceptor.Funcs{
				Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if tt.workDeletionError != nil {
						return tt.workDeletionError
					}
					deletedWorks = append(deletedWorks, obj.GetNamespace()+"/"+obj.GetName())
					return c.Delete(ctx, obj, opts...)
				},
			}
			if tt.apiReaderError != nil {
				funcs.Get = func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return tt.apiReaderError
				}
			}
			apiReaderBuilder.WithInterceptorFuncs(funcs)
			apiReader := apiReaderBuilder.Build()

			namespace := "default"
			if tt.isCRB {
				namespace = ""
			}

			err := DeleteWorks(context.TODO(), cacheClient, apiReader, namespace, "binding-test", bindingID, expectedWorkName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, deletedWorks, work1Namespace+"/"+expectedWorkName)
				if tt.expectWork2Deleted {
					assert.Contains(t, deletedWorks, work2Namespace+"/"+expectedWorkName)
				} else {
					assert.NotContains(t, deletedWorks, work2Namespace+"/"+expectedWorkName)
				}
			}
		})
	}
}
