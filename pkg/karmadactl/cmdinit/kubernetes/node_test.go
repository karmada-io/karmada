package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCommandInitOption_getKarmadaAPIServerIP(t *testing.T) {
	tests := []struct {
		name    string
		option  CommandInitOption
		nodes   []string
		labels  map[string]string
		wantErr bool
	}{
		{
			name: "KarmadaAPIServerAdvertiseAddress is not empty",
			option: CommandInitOption{
				KubeClientSet:                    fake.NewSimpleClientset(),
				KarmadaAPIServerAdvertiseAddress: "127.0.0.1",
			},
			nodes:   []string{"node1"},
			labels:  map[string]string{},
			wantErr: false,
		},
		{
			name: "three nodes but they are not master",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			nodes:   []string{"node1", "node2", "node3"},
			labels:  map[string]string{},
			wantErr: false,
		},
		{
			name: "three master nodes",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			nodes:   []string{"node1", "node2", "node3"},
			labels:  map[string]string{"node-role.kubernetes.io/control-plane": ""},
			wantErr: false,
		},
		{
			name: "no nodes",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range tt.nodes {
				_, err := tt.option.KubeClientSet.CoreV1().Nodes().Create(context.Background(), &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   v,
						Labels: tt.labels,
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{Address: "127.0.0.1"},
						},
					},
				}, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("create node error: %v", err)
				}
			}
			if err := tt.option.getKarmadaAPIServerIP(); (err != nil) != tt.wantErr {
				t.Errorf("CommandInitOption.getKarmadaAPIServerIP() = %v, want error:%v", err, tt.wantErr)
			}
		})
	}
}

func Test_nodeStatus(t *testing.T) {
	tests := []struct {
		name           string
		nodeConditions []corev1.NodeCondition
		isHealth       bool
	}{
		{
			name: "node is ready",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			isHealth: true,
		},
		{
			name: "node is unready",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
			isHealth: false,
		},
		{
			name: "node's memory pressure is true",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
			},
			isHealth: false,
		},
		{
			name: "node's memory pressure is false",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
			},
			isHealth: true,
		},
		{
			name: "node's disk pressure is true",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
			},
			isHealth: false,
		},
		{
			name: "node's disk pressure is false",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
			},
			isHealth: true,
		},
		{
			name: "node's network unavailable is false",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse},
			},
			isHealth: true,
		},
		{
			name: "node's network unavailable is true",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue},
			},
			isHealth: false,
		},
		{
			name: "node's pid pressure is false",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse},
			},
			isHealth: true,
		},
		{
			name: "node's pid pressure is true",
			nodeConditions: []corev1.NodeCondition{
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionTrue},
			},
			isHealth: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeStatus(tt.nodeConditions); got != tt.isHealth {
				t.Errorf("nodeStatus() = %v, want %v", got, tt.isHealth)
			}
		})
	}
}

func TestCommandInitOption_AddNodeSelectorLabels(t *testing.T) {
	tests := []struct {
		name    string
		option  CommandInitOption
		status  corev1.ConditionStatus
		spec    corev1.NodeSpec
		wantErr bool
	}{
		{
			name: "there is healthy node",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			status:  corev1.ConditionTrue,
			spec:    corev1.NodeSpec{},
			wantErr: false,
		},
		{
			name: "there is unhealthy node",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			status:  corev1.ConditionFalse,
			spec:    corev1.NodeSpec{},
			wantErr: true,
		},
		{
			name: "there is taint node",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			status: corev1.ConditionTrue,
			spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.option.KubeClientSet.CoreV1().Nodes().Create(context.Background(), &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: tt.status},
					},
				},
				Spec: tt.spec,
			}, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("create node error: %v", err)
			}
			if err := tt.option.AddNodeSelectorLabels(); (err != nil) != tt.wantErr {
				t.Errorf("CommandInitOption.AddNodeSelectorLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandInitOption_isNodeExist(t *testing.T) {
	tests := []struct {
		name     string
		option   CommandInitOption
		nodeName string
		labels   map[string]string
		exists   bool
	}{
		{
			name: "there is matched node",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			nodeName: "node1",
			labels:   map[string]string{"foo": "bar"},
			exists:   true,
		},
		{
			name: "there is no matched node",
			option: CommandInitOption{
				KubeClientSet: fake.NewSimpleClientset(),
			},
			nodeName: "node2",
			labels:   map[string]string{"bar": "foo"},
			exists:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.option.KubeClientSet.CoreV1().Nodes().Create(context.Background(), &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: tt.nodeName, Labels: tt.labels},
			}, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("create node error: %v", err)
			}
			if got := tt.option.isNodeExist("foo=bar"); got != tt.exists {
				t.Errorf("CommandInitOption.isNodeExist() = %v, want %v", got, tt.exists)
			}
		})
	}
}
