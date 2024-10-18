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
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		initOptions *InitOptions
		wantErr     bool
		errMsg      string
	}{
		{
			name: "Validate_WithoutInitWorkflowNameOpt_UnexpectedEmptyName",
			initOptions: &InitOptions{
				Namespace:      "test",
				Karmada:        &operatorv1alpha1.Karmada{},
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			wantErr: true,
			errMsg:  "unexpected empty name",
		},
		{
			name: "Validate_WithoutInitWorkflowNamespaceOpt_UnexpectedEmptyNamespace",
			initOptions: &InitOptions{
				Name:           "test_init",
				Karmada:        &operatorv1alpha1.Karmada{},
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			wantErr: true,
			errMsg:  "unexpected empty name or namespace",
		},
		{
			name: "Validate_InvalidWorkflowCRDTarballURL_UnexpectedInvalidCRDs",
			initOptions: &InitOptions{
				Name:           "test_init",
				Namespace:      "test",
				Karmada:        &operatorv1alpha1.Karmada{},
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
				CRDTarball: operatorv1alpha1.CRDTarball{
					HTTPSource: &operatorv1alpha1.HTTPSource{
						URL: "http://%41:8080/",
					},
				},
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("invalid URL escape \"%s\"", "%41"),
		},
		{
			name: "Validate_InvalidKarmadaConfig_KarmadaConfigMustBeDefined",
			initOptions: &InitOptions{
				Name:           "test_init",
				Namespace:      "test",
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			wantErr: true,
			errMsg:  "invalid Karmada configuration: Karmada, Karmada components, and Karmada API server must be defined",
		},
		{
			name: "Validate_InvalidKarmadaAPIServerServiceType_UnexpectedServiceType",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						HostCluster: &operatorv1alpha1.HostCluster{
							APIEndpoint: "10.0.0.1",
							SecretRef: &operatorv1alpha1.LocalSecretReference{
								Name:      "test-secret",
								Namespace: "test",
							},
						},
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeClusterIP,
							},
						},
					},
				},
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			wantErr: true,
			errMsg:  "service type of karmada-apiserver must be either NodePort or LoadBalancer",
		},
		{
			name: "Validate_InvalidKarmadaVersion_UnexpectedKarmadaInvalidVersion",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeClusterIP,
							},
						},
					},
				},
				KarmadaVersion: "v1;1;0",
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("unexpected karmada invalid version %s", "v1;1;0"),
		},
		{
			name: "Validate_ValidOptions_IsValidated",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](4),
									},
								},
							},
						},
						HostCluster: &operatorv1alpha1.HostCluster{
							APIEndpoint: "10.0.0.1",
							SecretRef: &operatorv1alpha1.LocalSecretReference{
								Name:      "test-secret",
								Namespace: "test",
							},
						},
					},
				},
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.initOptions.Validate()
			if (err != nil && !test.wantErr) || (err == nil && test.wantErr) {
				t.Errorf("Validate() = got %v error, but want %t error", err, test.wantErr)
			}
			if (err != nil && test.wantErr) && (!strings.Contains(err.Error(), test.errMsg)) {
				t.Errorf("Validate() = got %s, want %s", err.Error(), test.errMsg)
			}
		})
	}
}

func TestNewInitJob(t *testing.T) {
	tests := []struct {
		name          string
		initOptions   *InitOptions
		tasksExpected []workflow.Task
	}{
		{
			name: "NewInitJob_WithInitTasks_AllIsSubset",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](5),
									},
								},
							},
						},
					},
				},
				KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
				KarmadaDataDir: constants.KarmadaDataDir,
				Kubeconfig:     &rest.Config{},
			},
			tasksExpected: DefaultInitTasks,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initJob := NewInitJob(test.initOptions, test.tasksExpected)
			err := util.ContainAllTasks(initJob.Tasks, test.tasksExpected)
			if err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestNewRunData(t *testing.T) {
	karmadaVersion, err := utilversion.ParseGeneric(operatorv1alpha1.DefaultKarmadaImageVersion)
	if err != nil {
		t.Fatalf("failed to parse karmada version: %v", err)
	}

	clientWithoutAPIServiceIP := fakeclientset.NewSimpleClientset()
	clientWithAPIServiceIP := fakeclientset.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					"node-role.kubernetes.io/master": "",
				},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "192.168.1.1",
					},
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "192.168.1.2",
					},
				},
			},
		},
	)

	tests := []struct {
		name         string
		initOptions  *InitOptions
		initTasks    []workflow.Task
		mockFunc     func()
		wantInitData *initData
		wantErr      bool
		errMsg       string
	}{
		{
			name: "NewRunData_EmptyNamespace_NamespaceIsEmpty",
			initOptions: &InitOptions{
				Name: "test_init",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](5),
									},
								},
							},
						},
					},
				},
				KarmadaVersion: karmadaVersion.String(),
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			mockFunc: func() {},
			wantErr:  true,
			errMsg:   "unexpected empty name or namespace",
		},
		{
			name: "NewRunData_FailedToCreateLocalClusterClient_LocalClusterClientCreationError",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](5),
									},
								},
							},
						},
					},
				},
				KarmadaVersion: karmadaVersion.String(),
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			mockFunc: func() {
				util.ClientFactory = func(*rest.Config) (clientset.Interface, error) {
					return nil, fmt.Errorf("failed to create local cluster client")
				}
			},
			wantErr: true,
			errMsg:  "failed to create local cluster client",
		},
		{
			name: "NewRunData_FailedToCreateRemoteClusterClient_RemoteClusterClientCreationError",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](5),
									},
								},
							},
						},
						HostCluster: &operatorv1alpha1.HostCluster{
							APIEndpoint: "10.0.0.1",
							SecretRef: &operatorv1alpha1.LocalSecretReference{
								Name:      "test-secret",
								Namespace: "test",
							},
						},
					},
				},
				KarmadaVersion: karmadaVersion.String(),
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			mockFunc: func() {
				util.ClientFactory = func(*rest.Config) (clientset.Interface, error) {
					return fakeclientset.NewSimpleClientset(), nil
				}
				util.BuildClientFromSecretRefFactory = func(clientset.Interface, *operatorv1alpha1.LocalSecretReference) (clientset.Interface, error) {
					return nil, fmt.Errorf("failed to create remote cluster client")
				}
			},
			wantErr: true,
			errMsg:  "failed to create remote cluster client",
		},
		{
			name: "NewRunData_MissingAPIServerNodeIP_ExpectedValidNodeIPForAPIServer",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](5),
									},
								},
							},
						},
						PrivateRegistry: &operatorv1alpha1.ImageRegistry{
							Registry: "myprivateregistry.example.com",
						},
						FeatureGates: map[string]bool{},
						HostCluster: &operatorv1alpha1.HostCluster{
							Networking: &operatorv1alpha1.Networking{
								DNSDomain: ptr.To("example.com"),
							},
						},
					},
				},
				CRDTarball:     operatorv1alpha1.CRDTarball{},
				KarmadaVersion: karmadaVersion.String(),
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			mockFunc: func() {
				util.ClientFactory = func(*rest.Config) (clientset.Interface, error) {
					return clientWithoutAPIServiceIP, nil
				}
			},
			wantInitData: nil,
			wantErr:      true,
			errMsg:       "there are no nodes in cluster",
		},
		{
			name: "NewRunData_ValidInitOptions_ExpectedInitDataReturned",
			initOptions: &InitOptions{
				Name:      "test_init",
				Namespace: "test",
				Karmada: &operatorv1alpha1.Karmada{
					Spec: operatorv1alpha1.KarmadaSpec{
						Components: &operatorv1alpha1.KarmadaComponents{
							KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
								ServiceType: corev1.ServiceTypeNodePort,
							},
							Etcd: &operatorv1alpha1.Etcd{
								Local: &operatorv1alpha1.LocalEtcd{
									CommonSettings: operatorv1alpha1.CommonSettings{
										Replicas: ptr.To[int32](5),
									},
								},
							},
						},
						PrivateRegistry: &operatorv1alpha1.ImageRegistry{
							Registry: "myprivateregistry.example.com",
						},
						FeatureGates: map[string]bool{},
						HostCluster: &operatorv1alpha1.HostCluster{
							Networking: &operatorv1alpha1.Networking{
								DNSDomain: ptr.To[string]("example.com"),
							},
						},
					},
				},
				CRDTarball:     operatorv1alpha1.CRDTarball{},
				KarmadaVersion: karmadaVersion.String(),
				KarmadaDataDir: constants.KarmadaDataDir,
			},
			mockFunc: func() {
				util.ClientFactory = func(*rest.Config) (clientset.Interface, error) {
					return clientWithAPIServiceIP, nil
				}
			},
			wantInitData: &initData{
				name:                "test_init",
				namespace:           "test",
				karmadaVersion:      karmadaVersion,
				controlplaneAddress: "192.168.1.1",
				remoteClient:        clientWithAPIServiceIP,
				CRDTarball:          operatorv1alpha1.CRDTarball{},
				karmadaDataDir:      constants.KarmadaDataDir,
				privateRegistry:     "myprivateregistry.example.com",
				components: &operatorv1alpha1.KarmadaComponents{
					KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
						ServiceType: corev1.ServiceTypeNodePort,
					},
					Etcd: &operatorv1alpha1.Etcd{
						Local: &operatorv1alpha1.LocalEtcd{
							CommonSettings: operatorv1alpha1.CommonSettings{
								Replicas: ptr.To[int32](5),
							},
						},
					},
				},
				featureGates: map[string]bool{},
				dnsDomain:    "example.com",
				CertStore:    certs.NewCertStore(),
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.mockFunc()
			initData, err := newRunData(test.initOptions)
			if (err != nil && !test.wantErr) || (err == nil && test.wantErr) {
				t.Errorf("newRunData() = got %v error, but want %t error", err, test.wantErr)
			}
			if (err != nil && test.wantErr) && (!strings.Contains(err.Error(), test.errMsg)) {
				t.Errorf("newRunData() = got %s, want %s", err.Error(), test.errMsg)
			}

			err = deepEqualInitData(initData, test.wantInitData)
			if err != nil {
				t.Errorf("newRunData() = initData and wantInitData are not matched, got err: %v", err)
			}
		})
	}
}

func deepEqualInitData(data1, data2 *initData) error {
	if data1 == nil && data2 == nil {
		return nil
	}

	if data1.name != data2.name {
		return fmt.Errorf("expected name %s, got %s", data2.name, data1.name)
	}

	if data1.namespace != data2.namespace {
		return fmt.Errorf("expected namespace %s, got %s", data2.namespace, data1.namespace)
	}

	if data1.controlplaneAddress != data2.controlplaneAddress {
		return fmt.Errorf("expected control plane address %s, got %s", data2.controlplaneAddress, data1.controlplaneAddress)
	}

	if data1.karmadaDataDir != data2.karmadaDataDir {
		return fmt.Errorf("expected karmada data dir %s, got %s", data2.karmadaDataDir, data1.karmadaDataDir)
	}

	if data1.privateRegistry != data2.privateRegistry {
		return fmt.Errorf("expected private registry %s, got %s", data2.privateRegistry, data1.privateRegistry)
	}

	if data1.dnsDomain != data2.dnsDomain {
		return fmt.Errorf("expected dns domain %s, got %s", data2.dnsDomain, data1.dnsDomain)
	}

	if data1.KarmadaVersion() != data2.KarmadaVersion() {
		return fmt.Errorf("expected karamda version %s, got %s", data2.KarmadaVersion(), data1.KarmadaVersion())
	}

	if !reflect.DeepEqual(data1.featureGates, data2.featureGates) {
		return fmt.Errorf("expected feature gates %v, got %v", data2.featureGates, data1.featureGates)
	}

	if !reflect.DeepEqual(data1.components, data2.components) {
		return fmt.Errorf("expected karmada components %v, got %v", data2.components, data1.components)
	}

	if !reflect.DeepEqual(data1.remoteClient, data2.remoteClient) {
		return fmt.Errorf("expected remote client %v, got %v", data2.remoteClient, data1.remoteClient)
	}

	if !reflect.DeepEqual(data1.CRDTarball, data2.CRDTarball) {
		return fmt.Errorf("expected CRD Tarball %v, got %v", data2.CRDTarball, data1.CRDTarball)
	}

	if !reflect.DeepEqual(data1.CertStore.CertList(), data2.CertStore.CertList()) {
		return fmt.Errorf("expdcted cert store list %v, got %v", data2.CertStore.CertList(), data1.CertStore.CertList())
	}

	return nil
}
