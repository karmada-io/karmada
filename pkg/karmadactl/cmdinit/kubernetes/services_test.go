package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCommandInitOption_makeEtcdService(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	svc := cmdOpt.makeEtcdService("foo")
	if svc == nil {
		t.Errorf("makeEtcdService() return nil")
	}
}

func TestCommandInitOption_makeKarmadaAPIServerService(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	svc := cmdOpt.makeKarmadaAPIServerService()
	if svc == nil {
		t.Errorf("makeKarmadaAPIServerService() return nil")
	}
}

func TestCommandInitOption_kubeControllerManagerService(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	svc := cmdOpt.kubeControllerManagerService()
	if svc == nil {
		t.Errorf("kubeControllerManagerService() return nil")
	}
}

func TestCommandInitOption_karmadaWebhookService(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	svc := cmdOpt.karmadaWebhookService()
	if svc == nil {
		t.Errorf("karmadaWebhookService() return nil")
	}
}

func TestCommandInitOption_karmadaAggregatedAPIServerService(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	svc := cmdOpt.karmadaAggregatedAPIServerService()
	if svc == nil {
		t.Errorf("karmadaAggregatedAPIServerService() return nil")
	}
}

func TestCommandInitOption_isNodePortExist(t *testing.T) {
	tests := []struct {
		name   string
		option CommandInitOption
		svc    []*corev1.Service
		exists bool
	}{
		{
			name: "there is no svc",
			option: CommandInitOption{
				KubeClientSet:            fake.NewSimpleClientset(),
				KarmadaAPIServerNodePort: 30000,
			},
			exists: false,
		},
		{
			name: "there is no node port svc",
			option: CommandInitOption{
				KubeClientSet:            fake.NewSimpleClientset(),
				KarmadaAPIServerNodePort: 30000,
			},
			svc: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Port: 30000}}},
				},
			},
			exists: false,
		},
		{
			name: "there is node port svc and it's port is same with KarmadaAPIServerNodePort",
			option: CommandInitOption{
				KubeClientSet:            fake.NewSimpleClientset(),
				KarmadaAPIServerNodePort: 30000,
			},
			svc: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort, Ports: []corev1.ServicePort{{NodePort: 30000}}},
				},
			},
			exists: true,
		},
		{
			name: "there is node port svc and it's port is not same with KarmadaAPIServerNodePort",
			option: CommandInitOption{
				KubeClientSet:            fake.NewSimpleClientset(),
				KarmadaAPIServerNodePort: 30001,
			},
			svc: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort, Ports: []corev1.ServicePort{{NodePort: 30000}}},
				},
			},
			exists: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, svc := range tt.svc {
				_, err := tt.option.KubeClientSet.CoreV1().Services("karmada-system").Create(context.Background(), svc, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("create failed, err: %v", err)
				}
			}
			if got := tt.option.isNodePortExist(); got != tt.exists {
				t.Errorf("CommandInitOption.isNodePortExist() = %v, want %v", got, tt.exists)
			}
		})
	}
}
