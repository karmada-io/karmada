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

package descheduler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	addonutils "github.com/karmada-io/karmada/pkg/karmadactl/addons/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestStatus(t *testing.T) {
	name, namespace := names.KarmadaDeschedulerComponentName, "test"
	var replicas int32 = 2
	var priorityClass = "system-node-critical"
	tests := []struct {
		name       string
		listOpts   *addoninit.CommandAddonsListOption
		prep       func(*addoninit.CommandAddonsListOption) error
		wantStatus string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Status_WithoutDescheduler_AddonDisabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep:       func(*addoninit.CommandAddonsListOption) error { return nil },
			wantStatus: addoninit.AddonDisabledStatus,
		},
		{
			name: "Status_WithNetworkIssue_AddonUnknownStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				return addonutils.SimulateNetworkErrorOnOp(listOpts.KubeClientSet, "get", "deployments")
			},
			wantStatus: addoninit.AddonUnknownStatus,
			wantErr:    true,
			errMsg:     "unexpected error: encountered a network issue while get the deployments",
		},
		{
			name: "Status_ForKarmadaDeschedulerNotFullyAvailable_AddonUnhealthyStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaDeschedulerDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClass); err != nil {
					return fmt.Errorf("failed to create karmada descheduler deployment, got error: %v", err)
				}
				return addonutils.SimulateDeploymentUnready(listOpts.KubeClientSet, name, listOpts.Namespace)
			},
			wantStatus: addoninit.AddonUnhealthyStatus,
		},
		{
			name: "Status_WithAvailableKarmadaDeschedulerDeployment_AddonEnabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaDeschedulerDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClass); err != nil {
					return fmt.Errorf("failed to create karmada descheduler deployment, got error: %v", err)
				}
				return nil
			},
			wantStatus: addoninit.AddonEnabledStatus,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.listOpts); err != nil {
				t.Fatalf("failed to prep test env before checking on karmada descheduler addon status, got error: %v", err)
			}
			deschedulerAddonStatus, err := status(test.listOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Fatalf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if deschedulerAddonStatus != test.wantStatus {
				t.Errorf("expected addon status to be %s, but got %s", test.wantStatus, deschedulerAddonStatus)
			}
		})
	}
}

func TestEnableDescheduler(t *testing.T) {
	name, namespace := names.KarmadaDeschedulerComponentName, "test"
	var replicas int32 = 2
	tests := []struct {
		name       string
		enableOpts *addoninit.CommandAddonsEnableOption
		prep       func() error
		wantErr    bool
		errMsg     string
	}{
		{
			name: "EnableDescheduler_WaitingForKarmadaDescheduler_Created",
			enableOpts: &addoninit.CommandAddonsEnableOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
				KarmadaDeschedulerReplicas: replicas,
			},
			prep: func() error {
				addonutils.WaitForDeploymentRollout = func(client clientset.Interface, _ *appsv1.Deployment, _ int) error {
					_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("failed to get deployment %s, got an error: %v", name, err)
					}
					return nil
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep test environment before enabling descheduler, got an error: %v", err)
			}
			err := enableDescheduler(test.enableOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestDisableDescheduler(t *testing.T) {
	name, namespace := names.KarmadaDeschedulerComponentName, "test"
	client := fakeclientset.NewSimpleClientset()
	var replicas int32 = 2
	tests := []struct {
		name        string
		enableOpts  *addoninit.CommandAddonsEnableOption
		disableOpts *addoninit.CommandAddonsDisableOption
		prep        func(*addoninit.CommandAddonsEnableOption) error
		verify      func(clientset.Interface) error
		wantErr     bool
		errMsg      string
	}{
		{
			name: "DisableDescheduler_DisablingKarmadaDescheduler_Disabled",
			enableOpts: &addoninit.CommandAddonsEnableOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: client,
				},
				KarmadaDeschedulerReplicas: replicas,
			},
			disableOpts: &addoninit.CommandAddonsDisableOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: client,
				},
			},
			prep: func(enableOpts *addoninit.CommandAddonsEnableOption) error {
				addonutils.WaitForDeploymentRollout = func(client clientset.Interface, _ *appsv1.Deployment, _ int) error {
					_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("failed to get deployment %s, got an error: %v", name, err)
					}
					return nil
				}
				if err := enableDescheduler(enableOpts); err != nil {
					return fmt.Errorf("failed to enable descheduler, got an error: %v", err)
				}
				return nil
			},
			verify: func(client clientset.Interface) error {
				_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("deployment %s was expected to be deleted, but it was still found", name)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.enableOpts); err != nil {
				t.Fatalf("failed to prep test environment before disabling descheduler, got an error: %v", err)
			}
			err := disableDescheduler(test.disableOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(client); err != nil {
				t.Errorf("failed to verify disabling descheduler, got an error: %v", err)
			}
		})
	}
}

// createKarmadaDeschedulerDeployment creates or updates a Deployment for the Karmada descheduler
// in the specified namespace with the provided number of replicas.
// It parses and decodes the template for the Deployment before applying it to the cluster.
func createKarmadaDeschedulerDeployment(c clientset.Interface, replicas int32, namespace, priorityClass string) error {
	karmadaDeschedulerDeploymentBytes, err := addonutils.ParseTemplate(karmadaDeschedulerDeployment, DeploymentReplace{
		Namespace:         namespace,
		Replicas:          ptr.To[int32](replicas),
		PriorityClassName: priorityClass,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada descheduler deployment template: %v", err)
	}

	karmadaDeschedulerDeployment := &appsv1.Deployment{}
	if err = kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaDeschedulerDeploymentBytes, karmadaDeschedulerDeployment); err != nil {
		return fmt.Errorf("failed to decode karmada descheduler deployment, got error: %v", err)
	}
	if err = cmdutil.CreateOrUpdateDeployment(c, karmadaDeschedulerDeployment); err != nil {
		return fmt.Errorf("failed to create karmada descheduler deployment, got error: %v", err)
	}
	return nil
}
