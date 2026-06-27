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

package estimator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	clusterName := "test-cluster"
	name, namespace := names.GenerateEstimatorDeploymentName(clusterName), "test"
	var replicas int32 = 2
	tests := []struct {
		name       string
		listOpts   *addoninit.CommandAddonsListOption
		prep       func(*addoninit.CommandAddonsListOption) error
		wantStatus string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Status_WithoutClusters_AddonUnknownStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{},
			},
			prep:       func(*addoninit.CommandAddonsListOption) error { return nil },
			wantStatus: addoninit.AddonUnknownStatus,
		},
		{
			name: "Status_WithoutKarmadaSchedulerEstimator_AddonDisabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Cluster:       clusterName,
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
					Namespace:     namespace,
					Cluster:       clusterName,
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
			name: "Status_ForKarmadaSchedulerEstimatorNotFullyAvailable_AddonUnhealthyStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					Cluster:       clusterName,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaSchedulerEstimatorDeployment(listOpts.KubeClientSet, replicas, listOpts.Cluster, listOpts.Namespace); err != nil {
					return fmt.Errorf("failed to create karmada scheduler estimator deployment, got error: %v", err)
				}
				return addonutils.SimulateDeploymentUnready(listOpts.KubeClientSet, name, listOpts.Namespace)
			},
			wantStatus: addoninit.AddonUnhealthyStatus,
		},
		{
			name: "Status_WithFullyAvailableKarmadaSchedulerEstimator_AddonEnabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					Cluster:       clusterName,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaSchedulerEstimatorDeployment(listOpts.KubeClientSet, replicas, listOpts.Cluster, listOpts.Namespace); err != nil {
					return fmt.Errorf("failed to create karmada scheduler estimator deployment, got error: %v", err)
				}
				return nil
			},
			wantStatus: addoninit.AddonEnabledStatus,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.listOpts); err != nil {
				t.Fatalf("failed to prep test env before checking on karmada scheduler estimator addon status, got error: %v", err)
			}
			schedulerEstimatorAddonStatus, err := status(test.listOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Fatalf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if schedulerEstimatorAddonStatus != test.wantStatus {
				t.Errorf("expected addon status to be %s, but got %s", test.wantStatus, schedulerEstimatorAddonStatus)
			}
		})
	}
}

func TestEnableEstimator(t *testing.T) {
	clusterName := "test-cluster"
	name, namespace := names.GenerateEstimatorDeploymentName(clusterName), "test"
	var replicas int32 = 2
	tests := []struct {
		name       string
		enableOpts *addoninit.CommandAddonsEnableOption
		prep       func() error
		verify     func(clientset.Interface, *addoninit.CommandAddonsEnableOption) error
		wantErr    bool
		errMsg     string
	}{
		{
			name: "EnableDescheduler_WaitingForKarmadaSchedulerEstimator_Created",
			enableOpts: &addoninit.CommandAddonsEnableOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					Cluster:       clusterName,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
				KarmadaEstimatorReplicas: replicas,
				MemberKubeConfig:         filepath.Join(os.TempDir(), "member-kube-test.config"),
				MemberContext:            "member1",
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
			verify: func(c clientset.Interface, enableOpts *addoninit.CommandAddonsEnableOption) error {
				secretName := fmt.Sprintf("%s-kubeconfig", enableOpts.Cluster)
				if _, err := c.CoreV1().Secrets(enableOpts.Namespace).Get(context.TODO(), secretName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to get secret %s, got an error: %v", secretName, err)
				}
				if _, err := c.CoreV1().Services(enableOpts.Namespace).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to get service %s, got an error: %v", name, err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep test environment before enabling karmada scheduler estimator, got an error: %v", err)
			}
			err := enableEstimator(test.enableOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(test.enableOpts.KubeClientSet, test.enableOpts); err != nil {
				t.Errorf("failed to verify enabling karmada scheduler estimator, got an error: %v", err)
			}
		})
	}
}

func TestDisableEstimator(t *testing.T) {
	var (
		clusterName       = "test-cluster"
		name              = names.GenerateEstimatorDeploymentName(clusterName)
		secretName        = fmt.Sprintf("%s-kubeconfig", clusterName)
		namespace         = "test"
		client            = fakeclientset.NewSimpleClientset()
		replicas    int32 = 2
	)
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
			name: "DisableKarmadaSchedulerEstimator_DisablingKarmadaSchedulerEstimator_Disabled",
			enableOpts: &addoninit.CommandAddonsEnableOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: client,
				},
				KarmadaEstimatorReplicas: replicas,
				MemberKubeConfig:         filepath.Join(os.TempDir(), "member-kube-test.config"),
				MemberContext:            "member1",
			},
			disableOpts: &addoninit.CommandAddonsDisableOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					Cluster:       clusterName,
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
				return enableEstimator(enableOpts)
			},
			verify: func(client clientset.Interface) error {
				if _, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
					return fmt.Errorf("expected deployment %s in namespace %s to be deleted, but still found", name, namespace)
				}
				if _, err := client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
					return fmt.Errorf("expected service %s in namespace %s to be deleted, but still found", name, namespace)
				}
				if _, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{}); err == nil {
					return fmt.Errorf("expected secret %s in namespace %s to be deleted, but still found", secretName, namespace)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.enableOpts); err != nil {
				t.Fatalf("failed to prep test environment before disabling karmada scheduler estimator addon, got an error: %v", err)
			}
			err := disableEstimator(test.disableOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(client); err != nil {
				t.Errorf("failed to verify disabling karmada scheduler estimator addon, got an error: %v", err)
			}
		})
	}
}

// createKarmadaSchedulerEstimatorDeployment creates or updates a Deployment for the Karmada descheduler
// in the specified namespace and member cluster, using the provided number of replicas. It parses
// the Deployment template and applies it to the cluster.
func createKarmadaSchedulerEstimatorDeployment(c clientset.Interface, replicas int32, memberClusterName, namespace string) error {
	karmadaEstimatorDeploymentBytes, err := addonutils.ParseTemplate(karmadaEstimatorDeployment, DeploymentReplace{
		Namespace:         namespace,
		MemberClusterName: memberClusterName,
		Replicas:          ptr.To(replicas),
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada scheduler estimator deployment template :%v", err)
	}

	karmadaEstimatorDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaEstimatorDeploymentBytes, karmadaEstimatorDeployment); err != nil {
		return fmt.Errorf("decode karmada scheduler estimator deployment error: %v", err)
	}
	if err := cmdutil.CreateOrUpdateDeployment(c, karmadaEstimatorDeployment); err != nil {
		return fmt.Errorf("create or update scheduler estimator deployment error: %v", err)
	}
	return nil
}
