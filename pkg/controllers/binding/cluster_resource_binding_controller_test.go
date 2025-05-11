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

func makeFakeCRBCByResource(rs *workv1alpha2.ObjectReference) (*ClusterResourceBindingController, error) {
	c := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithIndex(
		&workv1alpha1.Work{},
		indexregistry.WorkIndexByLabelClusterResourceBindingID,
		indexregistry.GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel),
	).Build()
	tempDyClient := fakedynamic.NewSimpleDynamicClient(scheme.Scheme)
	if rs == nil {
		return &ClusterResourceBindingController{
			Client:          c,
			RESTMapper:      testing2.RestMapper,
			InformerManager: genericmanager.NewSingleClusterInformerManager(context.TODO(), tempDyClient, 0),
			DynamicClient:   tempDyClient,
		}, nil
	}

	var obj runtime.Object
	var src string
	switch rs.Kind {
	case "Namespace":
		obj = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Namespace: "", Name: rs.Name}}
		src = "namespaces"
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

	return &ClusterResourceBindingController{
		Client:          c,
		RESTMapper:      helper.NewGroupRESTMapper(rs.Kind, meta.RESTScopeNamespace),
		InformerManager: testingutil.NewSingleClusterInformerManagerByRS(src, obj),
		DynamicClient:   tempDyClient,
		EventRecorder:   record.NewFakeRecorder(1024),
	}, nil
}

func TestClusterResourceBindingController_Reconcile(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}
	req := controllerruntime.Request{NamespacedName: client.ObjectKey{Namespace: "", Name: "test"}}

	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
		del     bool
		req     controllerruntime.Request
	}{
		{
			name:    "Reconcile create crb",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
			req: req,
		},
		{
			name:    "Reconcile crb deleted",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
			del: true,
			req: req,
		},
		{
			name:    "Req not found",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-noexist",
				},
			},
			req: req,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("%s", err)
			}

			if tt.crb != nil {
				if err := c.Client.Create(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			if tt.del {
				if err := c.Client.Delete(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to delete ClusterResourceBinding: %v", err)
				}
			}

			result, err := c.Reconcile(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.Reconcile() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestClusterResourceBindingController_removeFinalizer(t *testing.T) {
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
		create  bool
	}{
		{
			name:    "Remove finalizer succeed",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
			},
			create: true,
		},
		{
			name:    "finalizers not exist",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			create: true,
		},
		{
			name:    "crb not found",
			want:    controllerruntime.Result{},
			wantErr: true,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
			},
			create: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(nil)
			if err != nil {
				t.Fatalf("Failed to create ClusterResourceBindingController: %v", err)
			}

			if tt.create && tt.crb != nil {
				if err := c.Client.Create(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			result, err := c.removeFinalizer(context.Background(), tt.crb)
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

func TestClusterResourceBindingController_syncBinding(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}
	tests := []struct {
		name    string
		want    controllerruntime.Result
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
	}{
		{
			name:    "sync binding",
			want:    controllerruntime.Result{},
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("failed to create fake ClusterResourceBindingController: %v", err)
			}

			result, err := c.syncBinding(context.Background(), tt.crb)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.syncBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.syncBinding() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestClusterResourceBindingController_removeOrphanWorks(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}
	tests := []struct {
		name    string
		wantErr bool
		crb     *workv1alpha2.ClusterResourceBinding
	}{
		{
			name:    "removeOrphanWorks test",
			wantErr: false,
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("failed to create fake ClusterResourceBindingController: %v", err)
			}

			err = c.removeOrphanWorks(context.Background(), tt.crb)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterResourceBindingController.syncBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestClusterResourceBindingController_newOverridePolicyFunc(t *testing.T) {
	rs := workv1alpha2.ObjectReference{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test",
	}

	tests := []struct {
		name string
		want []reconcile.Request
		req  client.Object
		crb  *workv1alpha2.ClusterResourceBinding
	}{
		{
			name: "not clusteroverridepolicy",
			want: nil,
			req:  &policyv1alpha1.OverridePolicy{},
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "newOverridePolicyFunc test succeed",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: rs.Name}}},
			req: &policyv1alpha1.ClusterOverridePolicy{
				Spec: policyv1alpha1.OverrideSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: rs.APIVersion,
							Kind:       rs.Kind,
							Name:       rs.Name,
						},
					},
				},
			},
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
		{
			name: "ResourceSelector is empty",
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: rs.Name}}},
			req: &policyv1alpha1.ClusterOverridePolicy{
				Spec: policyv1alpha1.OverrideSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{},
				},
			},
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Labels:     map[string]string{"clusterresourcebinding.karmada.io/permanent-id": "f2603cdb-f3f3-4a4b-b289-3186a4fef979"},
					Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: rs,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := makeFakeCRBCByResource(&rs)
			if err != nil {
				t.Fatalf("failed to create fake ClusterResourceBindingController: %v", err)
			}

			if tt.crb != nil {
				if err := c.Client.Create(context.Background(), tt.crb); err != nil {
					t.Fatalf("Failed to create ClusterResourceBinding: %v", err)
				}
			}

			got := c.newOverridePolicyFunc()
			result := got(context.Background(), tt.req)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("ClusterResourceBindingController.newOverridePolicyFunc() get = %v, want %v", result, tt.want)
				return
			}
		})
	}
}
