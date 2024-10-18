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
	"strings"
	"testing"

	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewDeInitDataJob(t *testing.T) {
	tests := []struct {
		name          string
		deInitOptions *DeInitOptions
		tasksExpected []workflow.Task
	}{
		{
			name: "NewDeInitDataJob_WithInitTasks_AllIsSubset",
			deInitOptions: &DeInitOptions{
				Name:        "test_deinit",
				Namespace:   "test",
				Kubeconfig:  &rest.Config{},
				HostCluster: &operatorv1alpha1.HostCluster{},
			},
			tasksExpected: DefaultDeInitTasks,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deInitJob := NewDeInitDataJob(test.deInitOptions, test.tasksExpected)
			err := util.ContainAllTasks(deInitJob.Tasks, test.tasksExpected)
			if err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestRunNewDeInitJob(t *testing.T) {
	tests := []struct {
		name          string
		deInitOptions *DeInitOptions
		tasksExpected []workflow.Task
		mockFunc      func()
		wantErr       bool
		errMsg        string
	}{
		{
			name: "RunNewDeInitJob_EmptyNamespace_NamespaceIsEmpty",
			deInitOptions: &DeInitOptions{
				Name:        "test_deinit",
				Kubeconfig:  &rest.Config{},
				HostCluster: &operatorv1alpha1.HostCluster{},
			},
			tasksExpected: DefaultDeInitTasks,
			mockFunc:      func() {},
			wantErr:       true,
			errMsg:        "unexpected empty name or namespace",
		},
		{
			name: "RunNewDeInitJob_EmptyName_NameIsEmpty",
			deInitOptions: &DeInitOptions{
				Namespace:   "test",
				Kubeconfig:  &rest.Config{},
				HostCluster: &operatorv1alpha1.HostCluster{},
			},
			tasksExpected: DefaultDeInitTasks,
			mockFunc:      func() {},
			wantErr:       true,
			errMsg:        "unexpected empty name",
		},
		{
			name: "RunNewDeInitJob_FailedToCreateLocalClusterClient_LocalClusterClientCreationError",
			deInitOptions: &DeInitOptions{
				Name:        "test_deinit",
				Namespace:   "test",
				Kubeconfig:  &rest.Config{},
				HostCluster: &operatorv1alpha1.HostCluster{},
			},
			tasksExpected: DefaultDeInitTasks,
			mockFunc: func() {
				util.ClientFactory = func(*rest.Config) (clientset.Interface, error) {
					return nil, fmt.Errorf("failed to create local cluster client")
				}
			},
			wantErr: true,
			errMsg:  "failed to create local cluster client",
		},
		{
			name: "RunNewDeInitJob_FailedToCreateRemoteClusterClient_RemoteClusterClientCreationError",
			deInitOptions: &DeInitOptions{
				Name:       "test_deinit",
				Namespace:  "test",
				Kubeconfig: &rest.Config{},
				HostCluster: &operatorv1alpha1.HostCluster{
					SecretRef: &operatorv1alpha1.LocalSecretReference{
						Namespace: "test",
						Name:      "karmada-demo",
					},
				},
			},
			tasksExpected: DefaultDeInitTasks,
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
			name: "RunNewDeInitJob_WithDeInitTasks_RunIsSuccessful",
			deInitOptions: &DeInitOptions{
				Name:        "test_deinit",
				Namespace:   "test",
				Kubeconfig:  &rest.Config{},
				HostCluster: &operatorv1alpha1.HostCluster{},
			},
			mockFunc: func() {
				util.ClientFactory = func(*rest.Config) (clientset.Interface, error) {
					return fakeclientset.NewSimpleClientset(), nil
				}
			},
			tasksExpected: DefaultDeInitTasks,
			wantErr:       false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.mockFunc()
			deInitJob := NewDeInitDataJob(test.deInitOptions, test.tasksExpected)
			err := deInitJob.Run()
			if (err != nil && !test.wantErr) || (err == nil && test.wantErr) {
				t.Errorf("RunNewDeInitJob() = got %v error, but want %t error", err, test.wantErr)
			}
			if (err != nil && test.wantErr) && (!strings.Contains(err.Error(), test.errMsg)) {
				t.Errorf("RunNewDeInitJob() = got %s, want %s", err.Error(), test.errMsg)
			}
		})
	}
}
