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

package karmada

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

func Test_validate(t *testing.T) {
	karmadaType := metav1.TypeMeta{Kind: "Karmada", APIVersion: "operator.karmada.io/v1alpha1"}
	testObj := metav1.ObjectMeta{Name: "test", Namespace: "test"}

	tests := []struct {
		name    string
		karmada *operatorv1alpha1.Karmada
		wantErr bool
	}{
		{
			name: "KarmadaSpec is empty",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec:       operatorv1alpha1.KarmadaSpec{},
			},
			wantErr: false,
		},
		{
			name: "CRDTarball HTTPSource is invalid",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					CRDTarball: &operatorv1alpha1.CRDTarball{
						HTTPSource: &operatorv1alpha1.HTTPSource{
							URL: "test",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "CRDTarball HTTPSource is valid",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					CRDTarball: &operatorv1alpha1.CRDTarball{
						HTTPSource: &operatorv1alpha1.HTTPSource{
							URL: "http://localhost",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "KarmadaAPIServer ServiceType unsupported",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{
								CommonSettings: operatorv1alpha1.CommonSettings{
									Image: operatorv1alpha1.Image{},
								},
							},
						},
						KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
							ServiceType: "ExternalName",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "KarmadaAPIServer ServiceType is invalid when using remote cluster",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{
								CommonSettings: operatorv1alpha1.CommonSettings{
									Image: operatorv1alpha1.Image{},
								},
							},
						},
						KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
							ServiceType: "ClusterIP",
						},
					},
					HostCluster: &operatorv1alpha1.HostCluster{
						SecretRef: &operatorv1alpha1.LocalSecretReference{
							Name: "test",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "KarmadaAPIServer ServiceType is valid",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{
								CommonSettings: operatorv1alpha1.CommonSettings{
									Image: operatorv1alpha1.Image{},
								},
							},
						},
						KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
							ServiceType: "NodePort",
						},
					},
					HostCluster: &operatorv1alpha1.HostCluster{
						SecretRef: &operatorv1alpha1.LocalSecretReference{
							Name: "test",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ETCD empty configuration",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{},
				},
			},
			wantErr: true,
		},
		{
			name: "ExternalETCD secretref unexpected name",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							External: &operatorv1alpha1.ExternalEtcd{
								SecretRef: operatorv1alpha1.LocalSecretReference{
									Name: "karmada-xx",
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ExternalETCD secretref is valid",
			karmada: &operatorv1alpha1.Karmada{
				TypeMeta:   karmadaType,
				ObjectMeta: testObj,
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							External: &operatorv1alpha1.ExternalEtcd{
								SecretRef: operatorv1alpha1.LocalSecretReference{
									Name: util.EtcdCertSecretName(testObj.Name),
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.karmada)
			if (err != nil && !tt.wantErr) || (err == nil && tt.wantErr) {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestController_validateKarmada(t *testing.T) {
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: operatorv1alpha1.KarmadaSpec{
			CRDTarball: &operatorv1alpha1.CRDTarball{
				HTTPSource: &operatorv1alpha1.HTTPSource{URL: "bad-url"},
			},
		},
	}

	cl, err := buildValidationFakeClient(karmada, true)
	if err != nil {
		t.Fatalf("failed to build fake client: %v", err)
	}
	recorder := record.NewFakeRecorder(10)
	ctrl := &Controller{Client: cl, EventRecorder: recorder}

	err = ctrl.validateKarmada(context.Background(), karmada)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	updated := &operatorv1alpha1.Karmada{}
	if getErr := cl.Get(context.Background(), client.ObjectKeyFromObject(karmada), updated); getErr != nil {
		t.Fatalf("failed to get updated karmada: %v", getErr)
	}

	if len(updated.Status.Conditions) == 0 {
		t.Fatal("expected status conditions to be updated")
	}
	condition := updated.Status.Conditions[0]
	if condition.Type != string(operatorv1alpha1.Ready) ||
		condition.Status != metav1.ConditionFalse ||
		condition.Reason != ValidationErrorReason {
		t.Fatalf("unexpected condition: type=%s, status=%s, reason=%s", condition.Type, condition.Status, condition.Reason)
	}

	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, ValidationErrorReason) {
			t.Fatalf("expected event to contain %q, got %q", ValidationErrorReason, event)
		}
	default:
		t.Fatal("expected validation warning event, got none")
	}
}

func TestController_validateKarmada_StatusUpdateError(t *testing.T) {
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: operatorv1alpha1.KarmadaSpec{
			CRDTarball: &operatorv1alpha1.CRDTarball{
				HTTPSource: &operatorv1alpha1.HTTPSource{URL: "bad-url"},
			},
		},
	}

	cl, err := buildValidationFakeClient(karmada, false)
	if err != nil {
		t.Fatalf("failed to build fake client: %v", err)
	}
	recorder := record.NewFakeRecorder(10)
	ctrl := &Controller{Client: cl, EventRecorder: recorder}

	err = ctrl.validateKarmada(context.Background(), karmada)
	if err == nil {
		t.Fatal("expected status update error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update validate condition") {
		t.Fatalf("expected wrapped update error, got %v", err)
	}
}

func buildValidationFakeClient(karmada *operatorv1alpha1.Karmada, withStatusSubresource bool) (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := operatorv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	b := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada)
	if withStatusSubresource {
		b = b.WithStatusSubresource(karmada)
	}

	return b.Build(), nil
}
