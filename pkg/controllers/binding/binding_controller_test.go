/*
Copyright 2023 The Karmada Authors.

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

package binding

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	testing2 "github.com/karmada-io/karmada/pkg/search/proxy/testing"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
	"github.com/karmada-io/karmada/test/helper"
)

// makeFakeRBCByResource to make a fake ResourceBindingController with ObjectReference.
// Currently support kind: Pod,Node. If you want support more kind, pls add it.
// rs is nil means use default RestMapper, see: github.com/karmada-io/karmada/pkg/search/proxy/testing/constant.go
func makeFakeRBCByResource(rs *workv1alpha2.ObjectReference) (*ResourceBindingController, error) {
	c := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithIndex(
		&workv1alpha1.Work{},
		indexregistry.WorkIndexByLabelResourceBindingID,
		indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
	).Build()

	tempDyClient := fakedynamic.NewSimpleDynamicClient(scheme.Scheme)
	if rs == nil {
		return &ResourceBindingController{
			Client:          c,
			RESTMapper:      testing2.RestMapper,
			InformerManager: genericmanager.NewSingleClusterInformerManager(context.TODO(), tempDyClient, 0),
			DynamicClient:   tempDyClient,
		}, nil
	}

	var obj runtime.Object
	var src string
	switch rs.Kind {
	case "Pod":
		obj = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: rs.Name, Namespace: rs.Namespace}}
		src = "pods"
	case "Node":
		obj = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: rs.Name, Namespace: rs.Namespace}}
		src = "nodes"
	default:
		return nil, fmt.Errorf("%s not support yet, pls add for it", rs.Kind)
	}

	tempDyClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: appsv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: rs.Name, Namespaced: true, Kind: rs.Kind, Version: rs.APIVersion},
			},
		},
	}

	return &ResourceBindingController{
		Client:          c,
		RESTMapper:      helper.NewGroupRESTMapper(rs.Kind, meta.RESTScopeNamespace),
		InformerManager: testingutil.NewSingleClusterInformerManagerByRS(src, obj),
		DynamicClient:   tempDyClient,
		EventRecorder:   record.NewFakeRecorder(1024),
	}, nil
}

func TestResourceBindingController_Reconcile(t *testing.T) {
	tmpReq := controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-rb",
			Namespace: "default",
		},
	}
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
		req     controllerruntime.Request
	}{
		{
			name:    "Err is RB not found",
			want:    controllerruntime.Result{},
			wantErr: false,
			req:     tmpReq,
		},
		{
			name:    "RB found without deleting",
			want:    controllerruntime.Result{},
			wantErr: true,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
			},
			req: tmpReq,
		},
		{
			name:    "Req not found",
			want:    controllerruntime.Result{Requeue: false},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "haha-rb",
					Namespace: "default",
				},
			},
			req: tmpReq,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(nil)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}
			if tt.rb != nil {
				// Add a rb to the fake client.
				if err := c.Client.Create(context.Background(), tt.rb); err != nil {
					t.Fatalf("Failed to create rb: %v", err)
				}
			}
			// Run the reconcile function.
			got, err := c.Reconcile(context.Background(), tt.req)
			// Check the results.
			if tt.wantErr && err == nil {
				t.Errorf("Expected an error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reconcile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceBindingController_syncBinding(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod",
	}
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
	}{
		{
			name:    "syncBinding success test",
			want:    controllerruntime.Result{},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(&rs)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}
			got, err := c.syncBinding(context.Background(), tt.rb)
			if (err != nil) != tt.wantErr {
				t.Errorf("syncBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("syncBinding() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceBindingController_removeOrphanWorks(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod",
	}
	tests := []struct {
		name    string
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
	}{
		{
			name:    "removeOrphanWorks success test",
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(&rs)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}
			if err := c.removeOrphanWorks(context.TODO(), tt.rb); (err != nil) != tt.wantErr {
				t.Errorf("removeOrphanWorks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourceBindingController_newOverridePolicyFunc(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "pod",
	}
	tests := []struct {
		name string
		want []reconcile.Request
		req  client.Object
		rb   *workv1alpha2.ResourceBinding
	}{
		{
			name: "newOverridePolicyFunc success test",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-rb"}}},
			req: &policyv1alpha1.OverridePolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: rs.Namespace},
				Spec: policyv1alpha1.OverrideSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: rs.APIVersion,
						Kind:       rs.Kind,
						Namespace:  rs.Namespace,
						Name:       rs.Name,
					},
				}},
			},
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "namespace not match",
			want: nil,
			req: &policyv1alpha1.OverridePolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: rs.Namespace},
				Spec: policyv1alpha1.OverrideSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{
					{
						APIVersion: rs.APIVersion,
						Kind:       rs.Kind,
						Namespace:  rs.Namespace,
						Name:       rs.Name,
					},
				}},
			},
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "test",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "ResourceSelector is empty",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test-rb"}}},
			req: &policyv1alpha1.OverridePolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: rs.Namespace},
				Spec:       policyv1alpha1.OverrideSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{}},
			},
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "client is nil",
			want: nil,
			req:  nil,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, makeErr := makeFakeRBCByResource(&rs)
			if makeErr != nil {
				t.Errorf("makeFakeRBCByResource %v", makeErr)
				return
			}

			if tt.rb != nil {
				if err := c.Client.Create(context.Background(), tt.rb); err != nil {
					t.Errorf("create rb %v", err)
					return
				}
			}

			got := c.newOverridePolicyFunc()
			result := got(context.Background(), tt.req)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("newOverridePolicyFunc() got() result is %v not same as want: %v", result, tt.want)
			}
		})
	}
}

func TestResourceBindingController_removeFinalizer(t *testing.T) {
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		rb      *workv1alpha2.ResourceBinding
		create  bool
	}{
		{
			name:    "Remove finalizer succeed",
			want:    controllerruntime.Result{},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.BindingControllerFinalizer},
				},
			},
			create: true,
		},
		{
			name:    "finalizers not exist",
			want:    controllerruntime.Result{},
			wantErr: false,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			create: true,
		},
		{
			name:    "rb not found",
			want:    controllerruntime.Result{},
			wantErr: true,
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.BindingControllerFinalizer},
				},
			},
			create: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeRBCByResource(nil)
			if err != nil {
				t.Fatalf("Failed to create ClusterResourceBindingController: %v", err)
			}

			if tt.create && tt.rb != nil {
				if err := c.Client.Create(context.Background(), tt.rb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			result, err := c.removeFinalizer(context.Background(), tt.rb)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.removeFinalizer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.removeFinalizer() = %v, want %v", result, tt.want)
			}
		})
	}
}
