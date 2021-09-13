package server

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app/options"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func TestAccurateSchedulerEstimatorServer_MaxAvailableReplicas(t *testing.T) {
	opt := &options.Options{
		ClusterName: "fake",
	}
	type args struct {
		request *pb.MaxAvailableReplicasRequest
	}
	tests := []struct {
		name         string
		objs         []runtime.Object
		args         args
		wantResponse *pb.MaxAvailableReplicasResponse
		wantErr      bool
	}{
		{
			name: "normal",
			// node 1 left: 2 cpu, 6 mem, 8 pod, 14 storage
			// node 2 left: 3 cpu, 5 mem, 9 pod, 12 storage
			// node 3 left: 8 cpu, 16 mem, 11 pod, 16 storage
			objs: []runtime.Object{
				testhelper.NewNode("machine1", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewNode("machine2", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewNode("machine3", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod1", "machine1", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod2", "machine1", 3*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod3", "machine1", 2*testhelper.ResourceUnitCPU, 4*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod4", "machine2", 4*testhelper.ResourceUnitCPU, 8*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod5", "machine2", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
			},
			// request 1 cpu, 2 mem
			args: args{
				request: &pb.MaxAvailableReplicasRequest{
					Cluster: "fake",
					ReplicaRequirements: pb.ReplicaRequirements{
						ResourceRequest: testhelper.NewResourceList(1*testhelper.ResourceUnitCPU, 2*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
					},
				},
			},
			wantResponse: &pb.MaxAvailableReplicasResponse{
				MaxReplicas: 12,
			},
			wantErr: false,
		},
		{
			name: "pod resource strict",
			// node 1 left: 2 cpu, 6 mem, 1 pod, 14 storage
			// node 2 left: 3 cpu, 5 mem, 1 pod, 12 storage
			// node 3 left: 8 cpu, 16 mem, 11 pod, 16 storage
			objs: []runtime.Object{
				testhelper.NewNode("machine1", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 4*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewNode("machine2", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 3*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewNode("machine3", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod1", "machine1", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod2", "machine1", 3*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod3", "machine1", 2*testhelper.ResourceUnitCPU, 4*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod4", "machine2", 4*testhelper.ResourceUnitCPU, 8*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod5", "machine2", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
			},
			// request 1 cpu, 2 mem
			args: args{
				request: &pb.MaxAvailableReplicasRequest{
					Cluster: "fake",
					ReplicaRequirements: pb.ReplicaRequirements{
						ResourceRequest: testhelper.NewResourceList(1*testhelper.ResourceUnitCPU, 2*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
					},
				},
			},
			wantResponse: &pb.MaxAvailableReplicasResponse{
				MaxReplicas: 10,
			},
			wantErr: false,
		},
		{
			name: "request with node selector",
			// node 1(with label: a = 1) left: 2 cpu, 6 mem, 8 pod, 14 storage
			// node 2(with label: a = 3; b = 2) left: 3 cpu, 5 mem, 9 pod, 12 storage
			// node 3(without labels) left: 8 cpu, 16 mem, 11 pod, 16 storage
			objs: []runtime.Object{
				testhelper.MakeNodeWithLabels("machine1", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage, map[string]string{"a": "1"}),
				testhelper.MakeNodeWithLabels("machine2", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage, map[string]string{"a": "3", "b": "2"}),
				testhelper.NewNode("machine3", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod1", "machine1", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod2", "machine1", 3*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod3", "machine1", 2*testhelper.ResourceUnitCPU, 4*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod4", "machine2", 4*testhelper.ResourceUnitCPU, 8*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod5", "machine2", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
			},
			// request 1 cpu, 2 mem and with node label a = 3
			args: args{
				request: &pb.MaxAvailableReplicasRequest{
					Cluster: "fake",
					ReplicaRequirements: pb.ReplicaRequirements{
						NodeClaim: &pb.NodeClaim{
							NodeSelector: map[string]string{
								"a": "3",
							},
						},
						ResourceRequest: testhelper.NewResourceList(1*testhelper.ResourceUnitCPU, 2*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
					},
				},
			},
			wantResponse: &pb.MaxAvailableReplicasResponse{
				MaxReplicas: 2,
			},
			wantErr: false,
		},
		{
			name: "request with node affinity",
			// node 1(with label: a = 1) left: 2 cpu, 6 mem, 8 pod, 14 storage
			// node 2(with label: a = 3; b = 2) left: 3 cpu, 5 mem, 9 pod, 12 storage
			// node 3(without labels) left: 8 cpu, 16 mem, 11 pod, 16 storage
			objs: []runtime.Object{
				testhelper.MakeNodeWithLabels("machine1", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage, map[string]string{"a": "1"}),
				testhelper.MakeNodeWithLabels("machine2", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage, map[string]string{"a": "3", "b": "2"}),
				testhelper.NewNode("machine3", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod1", "machine1", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod2", "machine1", 3*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod3", "machine1", 2*testhelper.ResourceUnitCPU, 4*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod4", "machine2", 4*testhelper.ResourceUnitCPU, 8*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod5", "machine2", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
			},
			// request 1 cpu, 2 mem and with node label a > 0
			args: args{
				request: &pb.MaxAvailableReplicasRequest{
					Cluster: "fake",
					ReplicaRequirements: pb.ReplicaRequirements{
						NodeClaim: &pb.NodeClaim{
							NodeAffinity: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "a",
												Operator: corev1.NodeSelectorOpGt,
												Values:   []string{"0"},
											},
										},
									},
								},
							},
						},
						ResourceRequest: testhelper.NewResourceList(1*testhelper.ResourceUnitCPU, 2*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
					},
				},
			},
			wantResponse: &pb.MaxAvailableReplicasResponse{
				MaxReplicas: 4,
			},
			wantErr: false,
		},
		{
			name: "request with tolerations",
			// node 1(with taint: key1 = value1) left: 2 cpu, 6 mem, 8 pod, 14 storage
			// node 2(with label: key2 = value2) left: 3 cpu, 5 mem, 9 pod, 12 storage
			// node 3(without labels) left: 8 cpu, 16 mem, 11 pod, 16 storage
			objs: []runtime.Object{
				testhelper.MakeNodeWithTaints("machine1", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage, []corev1.Taint{{Key: "key1", Value: "value1", Effect: corev1.TaintEffectNoSchedule}}),
				testhelper.MakeNodeWithTaints("machine2", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage, []corev1.Taint{{Key: "key2", Value: "value2", Effect: corev1.TaintEffectNoSchedule}}),
				testhelper.NewNode("machine3", 8*testhelper.ResourceUnitCPU, 16*testhelper.ResourceUnitMem, 11*testhelper.ResourceUnitPod, 16*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod1", "machine1", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod2", "machine1", 3*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
				testhelper.NewPodWithRequest("pod3", "machine1", 2*testhelper.ResourceUnitCPU, 4*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod4", "machine2", 4*testhelper.ResourceUnitCPU, 8*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
				testhelper.NewPodWithRequest("pod5", "machine2", 1*testhelper.ResourceUnitCPU, 3*testhelper.ResourceUnitMem, 2*testhelper.ResourceUnitEphemeralStorage),
			},
			// request 1 cpu, 2 mem and with node label a > 0
			args: args{
				request: &pb.MaxAvailableReplicasRequest{
					Cluster: "fake",
					ReplicaRequirements: pb.ReplicaRequirements{
						NodeClaim: &pb.NodeClaim{
							Tolerations: []corev1.Toleration{
								{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1"},
							},
						},
						ResourceRequest: testhelper.NewResourceList(1*testhelper.ResourceUnitCPU, 2*testhelper.ResourceUnitMem, testhelper.ResourceUnitZero),
					},
				},
			},
			wantResponse: &pb.MaxAvailableReplicasResponse{
				MaxReplicas: 10,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			es := NewEstimatorServer(fake.NewSimpleClientset(tt.objs...), opt)

			es.informerFactory.Start(ctx.Done())
			if !es.waitForCacheSync(ctx.Done()) {
				t.Errorf("MaxAvailableReplicas() error = %v, wantErr %v", fmt.Errorf("failed to wait for cache sync"), tt.wantErr)
			}

			gotResponse, err := es.MaxAvailableReplicas(ctx, tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("MaxAvailableReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResponse, tt.wantResponse) {
				t.Errorf("MaxAvailableReplicas() gotResponse = %v, want %v", gotResponse, tt.wantResponse)
			}
		})
	}
}
