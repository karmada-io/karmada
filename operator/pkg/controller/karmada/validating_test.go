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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

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

func TestValidateCommonSettings(t *testing.T) {
	tests := []struct {
		name           string
		commonSettings *operatorv1alpha1.CommonSettings
		expectedErrs   int
	}{
		{
			name:           "nil common settings",
			commonSettings: nil,
			expectedErrs:   0,
		},
		{
			name: "nil PDB config",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
			},
			expectedErrs: 0,
		},
		{
			name: "valid minAvailable config",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
				},
			},
			expectedErrs: 0,
		},
		{
			name: "valid maxUnavailable config",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			expectedErrs: 0,
		},
		{
			name: "valid percentage minAvailable config",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.String, StrVal: "50%"},
				},
			},
			expectedErrs: 0,
		},
		{
			name: "both minAvailable and maxUnavailable set",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable:   &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			expectedErrs: 1,
		},
		{
			name: "neither minAvailable nor maxUnavailable set",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas:                  ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{},
			},
			expectedErrs: 1,
		},
		{
			name: "minAvailable greater than replicas",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 4},
				},
			},
			expectedErrs: 1,
		},
		{
			name: "minAvailable equal to replicas",
			commonSettings: &operatorv1alpha1.CommonSettings{
				Replicas: ptr.To[int32](3),
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 3},
				},
			},
			expectedErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCommonSettings(tt.commonSettings, field.NewPath("test"))
			if len(errs) != tt.expectedErrs {
				t.Errorf("expected %d errors, got %d: %v", tt.expectedErrs, len(errs), errs)
			}
		})
	}
}
