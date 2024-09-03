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

package multiclusterservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestMCSController_claimMultiClusterServiceForService(t *testing.T) {
	tests := []struct {
		name    string
		svc     *corev1.Service
		mcs     *networkingv1alpha1.MultiClusterService
		wantErr bool
	}{
		{
			name:    "adds labels",
			wantErr: false,
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				TypeMeta: metav1.TypeMeta{Kind: "MultiClusterService"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "1234",
					},
				},
			},
		},
		{
			name:    "removes policy annotations labels",
			wantErr: false,
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Annotations: map[string]string{
						policyv1alpha1.PropagationPolicyNameAnnotation:      "pp",
						policyv1alpha1.PropagationPolicyNamespaceAnnotation: "pp-ns",
						policyv1alpha1.ClusterPropagationPolicyAnnotation:   "cpp",
					},
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				TypeMeta: metav1.TypeMeta{Kind: "MultiClusterService"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "1234",
					},
				},
			},
		},
		{
			name:    "removes unwanted services labels",
			wantErr: false,
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Labels: map[string]string{
						policyv1alpha1.PropagationPolicyPermanentIDLabel:        "1",
						policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "2",
					},
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				TypeMeta: metav1.TypeMeta{Kind: "MultiClusterService"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "1234",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			fakeClientBuilder := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(tt.svc, tt.mcs)
			fakeClient := fakeClientBuilder.Build()
			fakeRecorder := record.NewFakeRecorder(100)

			c := &MCSController{
				Client:        fakeClient,
				EventRecorder: fakeRecorder}
			if err := c.claimMultiClusterServiceForService(ctx, tt.svc, tt.mcs); (err != nil) != tt.wantErr {
				t.Errorf("claimMultiClusterServiceForService() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := fakeClient.Get(ctx, client.ObjectKeyFromObject(tt.svc), tt.svc); err != nil {
				t.Errorf("failed to get service: %v", err)
			}

			expectedLabels := map[string]string{
				networkingv1alpha1.MultiClusterServicePermanentIDLabel: tt.mcs.Labels[networkingv1alpha1.MultiClusterServicePermanentIDLabel],
				util.ResourceTemplateClaimedByLabel:                    util.MultiClusterServiceKind,
			}
			assert.Equalf(t, expectedLabels, tt.svc.Labels, "expected labels: %v, got: %v", expectedLabels, tt.svc.Labels)

			expectedAnnotations := map[string]string{
				networkingv1alpha1.MultiClusterServiceNameAnnotation:      tt.mcs.Name,
				networkingv1alpha1.MultiClusterServiceNamespaceAnnotation: tt.mcs.Namespace,
			}
			assert.Equalf(t, expectedAnnotations, tt.svc.Annotations, "expected annotations: %v, got: %v", expectedAnnotations, tt.svc.Annotations)
		})
	}
}
