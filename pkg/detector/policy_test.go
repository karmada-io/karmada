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

package detector

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

func Test_removeResourceMarksIfNotMatched(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
	deploymentGVK := appsv1.SchemeGroupVersion.WithKind("Deployment")
	restMapper.Add(deploymentGVK, meta.RESTScopeNamespace)

	tests := []struct {
		name            string
		objectReference workv1alpha2.ObjectReference
		selectors       []policyv1alpha1.ResourceSelector
		labels          []string
		annotations     []string
		existingObject  *unstructured.Unstructured
		wantUpdated     bool
		wantErr         bool
		setupClient     func() *fake.ClientBuilder
	}{
		{
			name: "update with non matching selectors",
			objectReference: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "test",
				Name:       "deployment",
			},
			selectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1",
					Kind:       "Pod",
					Namespace:  "default",
				},
			},
			labels:      []string{"app"},
			annotations: []string{"foo"},
			existingObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "deployment",
						"namespace":   "test",
						"labels":      map[string]interface{}{"app": "nginx"},
						"annotations": map[string]interface{}{"foo": "bar"},
					},
				},
			},
			wantUpdated: true,
			wantErr:     false,
			setupClient: func() *fake.ClientBuilder {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":        "deployment",
							"namespace":   "test",
							"labels":      map[string]interface{}{"app": "nginx"},
							"annotations": map[string]interface{}{"foo": "bar"},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).WithRESTMapper(restMapper)
			},
		},
		{
			name: "cannot update with matching selectors",
			objectReference: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "test",
				Name:       "deployment",
			},
			selectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  "test",
				},
			},
			labels:      []string{"app"},
			annotations: []string{"foo"},
			existingObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "deployment",
						"namespace":   "test",
						"labels":      map[string]interface{}{"app": "nginx"},
						"annotations": map[string]interface{}{"foo": "bar"},
					},
				},
			},
			wantUpdated: false,
			wantErr:     false,
			setupClient: func() *fake.ClientBuilder {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":        "deployment",
							"namespace":   "test",
							"labels":      map[string]interface{}{"app": "nginx"},
							"annotations": map[string]interface{}{"foo": "bar"},
						},
					},
				}
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(obj).WithRESTMapper(restMapper)
			},
		},
		{
			name: "failed to update with non matching selectors",
			objectReference: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "test",
				Name:       "deployment",
			},
			selectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1",
					Kind:       "Pod",
					Namespace:  "default",
				},
			},
			labels:      []string{"app"},
			annotations: []string{"foo"},
			existingObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "deployment",
						"namespace":   "test",
						"labels":      map[string]interface{}{"app": "nginx"},
						"annotations": map[string]interface{}{"foo": "bar"},
					},
				},
			},
			wantUpdated: false,
			wantErr:     true,
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithRESTMapper(restMapper)
			},
		},
		{
			name: "restmapper does not contain required scheme",
			objectReference: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "test",
				Name:       "deployment",
			},
			selectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  "test",
				},
			},
			labels:      []string{"app"},
			annotations: []string{"foo"},
			existingObject: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":        "pod",
						"namespace":   "test",
						"labels":      map[string]interface{}{"app": "nginx"},
						"annotations": map[string]interface{}{"foo": "bar"},
					},
				},
			},
			wantUpdated: false,
			wantErr:     false,
			setupClient: func() *fake.ClientBuilder {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name":        "pod",
							"namespace":   "test",
							"labels":      map[string]interface{}{"app": "nginx"},
							"annotations": map[string]interface{}{"foo": "bar"},
						},
					},
				}
				return fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(obj).WithRESTMapper(restMapper)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient().Build()
			stopCh := make(chan struct{})
			defer close(stopCh)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, tt.existingObject)
			genMgr := genericmanager.NewSingleClusterInformerManager(fakeDynamicClient, 0, stopCh)
			resourceDetector := &ResourceDetector{
				Client:          fakeClient,
				DynamicClient:   fakeDynamicClient,
				RESTMapper:      fakeClient.RESTMapper(),
				InformerManager: genMgr,
			}

			updated, err := resourceDetector.removeResourceMarksIfNotMatched(tt.objectReference, tt.selectors, tt.labels, tt.annotations)

			if (err != nil) != tt.wantErr {
				t.Errorf("removeResourceMarksIfNotMatched() error = %v, wantErr %v", err, tt.wantErr)
			}
			if updated != tt.wantUpdated {
				t.Errorf("removeResourceMarksIfNotMatched() = %v, want %v", updated, tt.wantUpdated)
			}
		})
	}
}

func Test_listPPDerivedRBs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workv1alpha2.Install(scheme))

	tests := []struct {
		name             string
		policyID         string
		policyName       string
		policyNamespace  string
		wantErr          bool
		setupClient      func() *fake.ClientBuilder
		expectedBindings *workv1alpha2.ResourceBindingList
	}{
		{
			name:            "list resource binding with policy and namespace",
			policyID:        "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
			policyName:      "test-policy-1",
			policyNamespace: "fake-namespace-1",
			wantErr:         false,
			setupClient: func() *fake.ClientBuilder {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "binding-1",
						Namespace:       "fake-namespace-1",
						ResourceVersion: "999",
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(rb)
			},
			expectedBindings: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "binding-1",
							Namespace:       "fake-namespace-1",
							ResourceVersion: "999",
							Labels: map[string]string{
								policyv1alpha1.PropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
							},
						},
					},
				},
			},
		},
		{
			name:            "cannot list resource binding with policy and namespace",
			policyID:        "g5609cgb-f3f3-4a4b-b289-4512a4fef979",
			policyName:      "test-policy-2",
			policyNamespace: "fake-namespace-2",
			wantErr:         true,
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder()
			},
			expectedBindings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient().Build()

			resourceDetector := &ResourceDetector{
				Client: fakeClient,
			}

			bindings, err := resourceDetector.listPPDerivedRBs(tt.policyID, tt.policyNamespace, tt.policyName)

			if (err != nil) != tt.wantErr {
				t.Errorf("listPPDerivedRBs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.expectedBindings, bindings) {
				t.Errorf("listPPDerivedRBs() = %v, want %v", bindings, tt.expectedBindings)
			}
		})
	}
}

func Test_listCPPDerivedRBs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workv1alpha2.Install(scheme))

	tests := []struct {
		name             string
		policyID         string
		policyName       string
		wantErr          bool
		setupClient      func() *fake.ClientBuilder
		expectedBindings *workv1alpha2.ResourceBindingList
	}{
		{
			name:       "list resource binding with policy",
			policyID:   "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
			policyName: "test-policy-1",
			wantErr:    false,
			setupClient: func() *fake.ClientBuilder {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "binding-1",
						ResourceVersion: "999",
						Labels: map[string]string{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(rb)
			},
			expectedBindings: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "binding-1",
							ResourceVersion: "999",
							Labels: map[string]string{
								policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
							},
						},
					},
				},
			},
		},
		{
			name:       "cannot list resource binding with policy",
			policyID:   "g5609cgb-f3f3-4a4b-b289-4512a4fef979",
			policyName: "test-policy-2",
			wantErr:    true,
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder()
			},
			expectedBindings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient().Build()

			resourceDetector := &ResourceDetector{
				Client: fakeClient,
			}

			bindings, err := resourceDetector.listCPPDerivedRBs(tt.policyID, tt.policyName)

			if (err != nil) != tt.wantErr {
				t.Errorf("listCPPDerivedRBs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.expectedBindings, bindings) {
				t.Errorf("listCPPDerivedRBs() = %v, want %v", bindings, tt.expectedBindings)
			}
		})
	}
}

func Test_listCPPDerivedCRBs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(workv1alpha2.Install(scheme))

	tests := []struct {
		name             string
		policyID         string
		policyName       string
		wantErr          bool
		setupClient      func() *fake.ClientBuilder
		expectedBindings *workv1alpha2.ClusterResourceBindingList
	}{
		{
			name:       "list cluster binding with policy",
			policyID:   "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
			policyName: "test-policy-1",
			wantErr:    false,
			setupClient: func() *fake.ClientBuilder {
				rb := &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "binding-1",
						ResourceVersion: "999",
						Labels: map[string]string{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(rb)
			},
			expectedBindings: &workv1alpha2.ClusterResourceBindingList{
				Items: []workv1alpha2.ClusterResourceBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "binding-1",
							ResourceVersion: "999",
							Labels: map[string]string{
								policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
							},
						},
					},
				},
			},
		},
		{
			name:       "cannot list cluster binding with policy",
			policyID:   "g5609cgb-f3f3-4a4b-b289-4512a4fef979",
			policyName: "test-policy-2",
			wantErr:    true,
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder()
			},
			expectedBindings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient().Build()

			resourceDetector := &ResourceDetector{
				Client: fakeClient,
			}

			bindings, err := resourceDetector.listCPPDerivedCRBs(tt.policyID, tt.policyName)

			if (err != nil) != tt.wantErr {
				t.Errorf("listCPPDerivedCRBs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.expectedBindings, bindings) {
				t.Errorf("listCPPDerivedCRBs() = %v, want %v", bindings, tt.expectedBindings)
			}
		})
	}
}

func Test_excludeClusterPolicy(t *testing.T) {
	tests := []struct {
		name      string
		objLabels map[string]string
		want      bool
	}{
		{
			name:      "propagation policy was claimed",
			objLabels: map[string]string{},
			want:      false,
		}, {
			name: "propagation policy was not claimed",
			objLabels: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := excludeClusterPolicy(tt.objLabels)
			assert.Equal(t, tt.want, got)
		})
	}
}
