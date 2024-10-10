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

package util

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestGetControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		address      string
		port         string
		wantErr      bool
		wantEndpoint string
	}{
		{
			name:    "GetControlplaneEndpoint_InvalidAddress_AddressIsInvalid",
			address: "192.168:1:1",
			port:    "6060",
			wantErr: true,
		},
		{
			name:         "GetControlplaneEndpoint_ParseEndpoint_EndpointParsed",
			address:      "192.168.1.1",
			port:         "6060",
			wantErr:      false,
			wantEndpoint: "https://192.168.1.1:6060",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controlPlaneEndpoint, err := GetControlplaneEndpoint(test.address, test.port)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if controlPlaneEndpoint != test.wantEndpoint {
				t.Errorf("expected endpoint %s, but got %s", test.wantEndpoint, controlPlaneEndpoint)
			}
		})
	}
}

func TestGetAPIServiceIP(t *testing.T) {
	tests := []struct {
		name             string
		client           clientset.Interface
		prep             func(clientset.Interface) error
		wantAPIServiceIP string
		wantErr          bool
		errMsg           string
	}{
		{
			name:    "GetAPIServiceIP_WithNoNodesInTheCluster_FailedToGetAPIServiceIP",
			client:  fakeclientset.NewSimpleClientset(),
			prep:    func(clientset.Interface) error { return nil },
			wantErr: true,
			errMsg:  "there are no nodes in cluster",
		},
		{
			name:   "GetAPIServiceIP_WithoutMasterNode_WorkerNodeAPIServiceIPReturned",
			client: fakeclientset.NewSimpleClientset(),
			prep: func(client clientset.Interface) error {
				nodes := []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
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
				}
				for _, node := range nodes {
					_, err := client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
					if err != nil {
						return fmt.Errorf("failed to create node %s: %v", node.Name, err)
					}
				}
				return nil
			},
			wantErr:          false,
			wantAPIServiceIP: "192.168.1.2",
		},
		{
			name:   "GetAPIServiceIP_WithMasterNode_MasterNodeAPIServiceIPReturned",
			client: fakeclientset.NewSimpleClientset(),
			prep: func(client clientset.Interface) error {
				nodes := []*corev1.Node{
					{
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
					{
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
				}
				for _, node := range nodes {
					_, err := client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
					if err != nil {
						return fmt.Errorf("failed to create node %s: %v", node.Name, err)
					}
				}
				return nil
			},
			wantErr:          false,
			wantAPIServiceIP: "192.168.1.1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Errorf("failed to prep before getting API service IP: %v", err)
			}
			apiServiceIP, err := GetAPIServiceIP(test.client)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if apiServiceIP != test.wantAPIServiceIP {
				t.Errorf("expected API service IP %s, but got %s", test.wantAPIServiceIP, apiServiceIP)
			}
		})
	}
}
