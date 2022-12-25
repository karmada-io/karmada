package v1alpha1

import (
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestKarmadaDefault(t *testing.T) {
	testCases := []struct {
		desc    string
		karmada *Karmada
		want    *Karmada
	}{
		{
			desc: "simple karmada resource",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test01",
				},
			},
			want: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test01",
				},
				Spec: KarmadaSpec{
					Components: &KarmadaComponents{
						Etcd: &Etcd{
							Local: &LocalEtcd{
								Image: Image{
									ImageRepository: "registry.k8s.io/etcd",
									ImageTag:        "3.5.3-0",
								},
								VolumeData: &VolumeData{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						KarmadaAPIServer: &KarmadaAPIServer{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "registry.k8s.io/kube-apiserver",
									ImageTag:        "v1.25.2",
								},
								Replicas: pointer.Int32(1),
							},
							ServiceSubnet: pointer.String("10.96.0.0/12"),
						},
						KarmadaAggregratedAPIServer: &KarmadaAggregratedAPIServer{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "docker.io/karmada/karmada-aggregated-apiserver",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KubeControllerManager: &KubeControllerManager{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "registry.k8s.io/karmada/kube-controller-manager",
									ImageTag:        "v1.25.2",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KarmadaControllerManager: &KarmadaControllerManager{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "docker.io/karmada/karmada-controller-manager",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KarmadaScheduler: &KarmadaScheduler{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "docker.io/karmada/karmada-scheduler",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KarmadaWebhook: &KarmadaWebhook{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "docker.io/karmada/karmada-webhook",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
					},
				},
			},
		},
		{
			desc: "specify private registry",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test02",
				},
				Spec: KarmadaSpec{
					PrivateRegistry: &ImageRegistry{
						Registry: "127.0.0.1:8080",
					},
				},
			},
			want: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test02",
				},
				Spec: KarmadaSpec{
					PrivateRegistry: &ImageRegistry{
						Registry: "127.0.0.1:8080",
					},
					Components: &KarmadaComponents{
						Etcd: &Etcd{
							Local: &LocalEtcd{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/etcd",
									ImageTag:        "3.5.3-0",
								},
								VolumeData: &VolumeData{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						KarmadaAPIServer: &KarmadaAPIServer{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/kube-apiserver",
									ImageTag:        "v1.25.2",
								},
								Replicas: pointer.Int32(1),
							},
							ServiceSubnet: pointer.String("10.96.0.0/12"),
						},
						KarmadaAggregratedAPIServer: &KarmadaAggregratedAPIServer{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/karmada/karmada-aggregated-apiserver",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KubeControllerManager: &KubeControllerManager{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/karmada/kube-controller-manager",
									ImageTag:        "v1.25.2",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KarmadaControllerManager: &KarmadaControllerManager{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/karmada/karmada-controller-manager",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KarmadaScheduler: &KarmadaScheduler{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/karmada/karmada-scheduler",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
						KarmadaWebhook: &KarmadaWebhook{
							CommonSettings: CommonSettings{
								Image: Image{
									ImageRepository: "127.0.0.1:8080/karmada/karmada-webhook",
									ImageTag:        "v1.4.0",
								},
								Replicas: pointer.Int32(1),
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			tt.karmada.Default()

			if !reflect.DeepEqual(tt.karmada, tt.want) {
				t.Errorf("Default() = %v, want %v", tt.karmada, tt.want)
			}
		})
	}
}

func TestValidateCreate(t *testing.T) {
	testCases := []struct {
		desc        string
		karmada     *Karmada
		wantErr     bool
		expectedErr error
	}{
		{
			desc: "valid karmada object",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test02",
				},
				Spec: KarmadaSpec{
					Components: &KarmadaComponents{
						Etcd: &Etcd{
							External: &ExternalEtcd{
								Endpoints: []string{"localhost:2379"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			desc: "empty field karmada.spec.components",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test01",
				},
			},
			wantErr:     true,
			expectedErr: errors.New("Karmada.operator.karmada.io \"karmada-test01\" is invalid: spec.components: Required value: expected non-empty components object"),
		},
		{
			desc: "invalid field karmada.spec",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test01",
				},
				Spec: KarmadaSpec{
					HostCluster: &HostCluster{
						Networking: &Networking{
							DNSDomain: pointer.String("aaaaa/bbbbb"),
						},
					},
				},
			},
			wantErr:     true,
			expectedErr: errors.New("Karmada.operator.karmada.io \"karmada-test01\" is invalid: [spec.hostCluster.networking.dnsDomain: Invalid value: \"aaaaa/bbbbb\": must be a valid domain value: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'), spec.components: Required value: expected non-empty components object]"),
		},
		{
			desc: "empty field karmada.spec.components.etcd",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test02",
				},
				Spec: KarmadaSpec{
					Components: &KarmadaComponents{},
				},
			},
			wantErr:     true,
			expectedErr: errors.New("Karmada.operator.karmada.io \"karmada-test02\" is invalid: spec.components.etcd: Required value: expected non-empty etcd object"),
		},
		{
			desc: "invalid field karmada.spec.components.etcd",
			karmada: &Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "karmada-system",
					Name:      "karmada-test02",
				},
				Spec: KarmadaSpec{
					Components: &KarmadaComponents{
						Etcd: &Etcd{},
					},
				},
			},
			wantErr:     true,
			expectedErr: errors.New("Karmada.operator.karmada.io \"karmada-test02\" is invalid: spec.components.etcd.data: Required value: expected either spec.components.etcd.external or spec.components.etcd.local to be populated"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.karmada.ValidateCreate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Expected err=%v, got %v", tt.wantErr, err)
				return
			}

			// check whether the error message is consistent.
			if tt.wantErr && tt.expectedErr != nil {
				if err.Error() != tt.expectedErr.Error() {
					t.Errorf("Expected error message = %s, got error message %s", tt.expectedErr.Error(), err.Error())
				}
			}
		})
	}
}
