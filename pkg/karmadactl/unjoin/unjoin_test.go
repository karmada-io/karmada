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

package unjoin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeclient "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	coretesting "k8s.io/client-go/testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		unjoinOpts *CommandUnjoinOption
		args       []string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "Validate_WithMoreThanOneArg_OnlyTheClusterNameIsRequired",
			unjoinOpts: &CommandUnjoinOption{},
			args:       []string{"cluster2", "cluster3"},
			wantErr:    true,
			errMsg:     "only the cluster name is allowed as an argument",
		},
		{
			name:       "Validate_WithoutClusterNameToJoinWith_ClusterNameIsRequired",
			unjoinOpts: &CommandUnjoinOption{ClusterName: ""},
			args:       []string{"cluster2"},
			wantErr:    true,
			errMsg:     "cluster name is required",
		},
		{
			name: "Validate_WithNegativeWaitValue_WaitValueMustBePositiveDuration",
			unjoinOpts: &CommandUnjoinOption{
				ClusterName: "cluster1",
				Wait:        -1 * time.Hour,
			},
			args:    []string{"cluster2"},
			wantErr: true,
			errMsg:  "must be a positive duration",
		},
		{
			name: "Validate_ValidateCommandUnjoinOptions_Validated",
			unjoinOpts: &CommandUnjoinOption{
				ClusterName: "cluster1",
				Wait:        2 * time.Minute,
			},
			args:    []string{"cluster2"},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.unjoinOpts.Validate(test.args)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestRunUnJoinCluster(t *testing.T) {
	tests := []struct {
		name                                  string
		unjoinOpts                            *CommandUnjoinOption
		controlPlaneRestConfig, clusterConfig *rest.Config
		controlKubeClient, clusterKubeClient  kubeclient.Interface
		karmadaClient                         karmadaclientset.Interface
		prep                                  func(controlKubeClient kubeclient.Interface, clusterKubeClient kubeclient.Interface, karmadaClient karmadaclientset.Interface, opts *CommandUnjoinOption) error
		verify                                func(controlKubeClient kubeclient.Interface, clusterKubeClient kubeclient.Interface, karmadaClient karmadaclientset.Interface, opts *CommandUnjoinOption) error
		wantErr                               bool
		errMsg                                string
	}{
		{
			name:                   "RunUnJoinCluster_DeleteClusterObject_PullModeMemberCluster",
			unjoinOpts:             &CommandUnjoinOption{ClusterName: "member1"},
			controlPlaneRestConfig: &rest.Config{},
			clusterConfig:          &rest.Config{},
			karmadaClient:          fakekarmadaclient.NewSimpleClientset(),
			prep: func(_ kubeclient.Interface, _ kubeclient.Interface, karmadaClient karmadaclientset.Interface, _ *CommandUnjoinOption) error {
				karmadaClient.(*fakekarmadaclient.Clientset).Fake.PrependReactor("get", "clusters", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "member1",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							SyncMode: clusterv1alpha1.Pull,
						},
					}, nil
				})
				controlPlaneKarmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
					return karmadaClient
				}
				return nil
			},
			verify: func(kubeclient.Interface, kubeclient.Interface, karmadaclientset.Interface, *CommandUnjoinOption) error {
				return nil
			},
			wantErr: true,
			errMsg:  "cluster member1 is a Pull mode member cluster, please use command `unregister` if you want to continue unregistering the cluster",
		},
		{
			name:                   "RunUnJoinCluster_DeleteClusterObject_FailedToDeleteClusterObject",
			unjoinOpts:             &CommandUnjoinOption{ClusterName: "member1"},
			controlPlaneRestConfig: &rest.Config{},
			clusterConfig:          &rest.Config{},
			karmadaClient:          fakekarmadaclient.NewSimpleClientset(),
			prep: func(_ kubeclient.Interface, _ kubeclient.Interface, karmadaClient karmadaclientset.Interface, _ *CommandUnjoinOption) error {
				karmadaClient.(*fakekarmadaclient.Clientset).Fake.PrependReactor("get", "clusters", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, &clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "member1",
						},
						Spec: clusterv1alpha1.ClusterSpec{
							SyncMode: clusterv1alpha1.Push,
						},
					}, nil
				})
				karmadaClient.(*fakekarmadaclient.Clientset).Fake.PrependReactor("delete", "clusters", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while deleting the clusters")
				})
				controlPlaneKarmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
					return karmadaClient
				}
				return nil
			},
			verify: func(kubeclient.Interface, kubeclient.Interface, karmadaclientset.Interface, *CommandUnjoinOption) error {
				return nil
			},
			wantErr: true,
			errMsg:  "encountered a network issue while deleting the clusters",
		},
		{
			name: "RunUnJoinCluster_UnjoinCluster_UnjoinedTheCluster",
			unjoinOpts: &CommandUnjoinOption{
				ClusterName:      "member1",
				ClusterNamespace: options.DefaultKarmadaClusterNamespace,
				forceDeletion:    false,
				Wait:             time.Minute,
			},
			controlKubeClient: fakeclientset.NewClientset(),
			karmadaClient:     fakekarmadaclient.NewSimpleClientset(),
			clusterKubeClient: fakeclientset.NewClientset(),
			clusterConfig:     &rest.Config{},
			prep: func(controlKubeClient, clusterKubeClient kubeclient.Interface, karmadaClient karmadaclientset.Interface, opts *CommandUnjoinOption) error {
				return prepUnjoinCluster(opts, controlKubeClient, clusterKubeClient, karmadaClient)
			},
			verify: func(_ kubeclient.Interface, clusterKubeClient kubeclient.Interface, karmadaClient karmadaclientset.Interface, opts *CommandUnjoinOption) error {
				if err := verifyClusterObjectDeleted(karmadaClient, opts); err != nil {
					return err
				}
				if err := verifyRBACResourcesDeleted(clusterKubeClient, opts.ClusterName); err != nil {
					return err
				}
				if err := verifyServiceAccountDeleted(clusterKubeClient, opts.ClusterName, opts.ClusterNamespace); err != nil {
					return err
				}
				if err := verifyNamespaceDeleted(clusterKubeClient, opts.ClusterName, opts.ClusterNamespace); err != nil {
					return err
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.controlKubeClient, test.clusterKubeClient, test.karmadaClient, test.unjoinOpts); err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			err := test.unjoinOpts.RunUnJoinCluster(test.controlPlaneRestConfig, test.clusterConfig)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.controlKubeClient, test.clusterKubeClient, test.karmadaClient, test.unjoinOpts); err != nil {
				t.Errorf("failed to verify unjoining the cluster %s, got error: %v", test.unjoinOpts.ClusterName, err)
			}
		})
	}
}

func prepUnjoinCluster(opts *CommandUnjoinOption, controlKubeClient, clusterKubeClient kubeclient.Interface, karmadaClient karmadaclientset.Interface) error {
	// Create cluster object on karmada client.
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: opts.ClusterName,
		},
	}
	if _, err := karmadaClient.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create cluster %s, got error: %v", opts.ClusterName, err)
	}
	if err := createNamespace(clusterKubeClient, cluster.GetName(), opts.ClusterNamespace); err != nil {
		return err
	}
	if err := createServiceAccount(clusterKubeClient, cluster.GetName(), opts.ClusterNamespace); err != nil {
		return err
	}
	if err := createRBACResources(clusterKubeClient, cluster.GetName()); err != nil {
		return err
	}
	controlPlaneKarmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
		return karmadaClient
	}
	controlPlaneKubeClientBuilder = func(*rest.Config) kubeclient.Interface {
		return controlKubeClient
	}
	clusterKubeClientBuilder = func(*rest.Config) kubeclient.Interface {
		return clusterKubeClient
	}
	return nil
}

func createRBACResources(clusterKubeClient kubeclient.Interface, unjoiningClusterName string) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.GenerateRoleName(unjoiningClusterName),
		},
	}
	if _, err := util.CreateClusterRole(clusterKubeClient, clusterRole); err != nil {
		return fmt.Errorf("failed to create cluster role %s in unjoining cluster %s, got error: %v", clusterRole.GetName(), unjoiningClusterName, err)
	}

	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.GenerateRoleName(serviceAccountName),
		},
		RoleRef: rbacv1.RoleRef{Name: clusterRole.GetName()},
	}
	if _, err := util.CreateClusterRoleBinding(clusterKubeClient, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create cluster role binding %s in unjoining cluster %s, got error: %v", clusterRoleBinding.GetName(), unjoiningClusterName, err)
	}

	return nil
}

func createServiceAccount(clusterKubeClient kubeclient.Interface, unjoiningClusterName, namespace string) error {
	serviceAccountObj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.GenerateServiceAccountName(unjoiningClusterName),
			Namespace: namespace,
		},
	}
	if _, err := clusterKubeClient.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccountObj, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create service account %s in unjoining cluster %s, got error: %v", serviceAccountObj.GetName(), unjoiningClusterName, err)
	}
	return nil
}

func createNamespace(clusterKubeClient kubeclient.Interface, unjoiningClusterName, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if _, err := clusterKubeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create namespace %s in unjoining cluster %s, got error: %v", namespace, unjoiningClusterName, err)
	}
	return nil
}

func verifyClusterObjectDeleted(karmadaClient karmadaclientset.Interface, opts *CommandUnjoinOption) error {
	if _, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{}); err == nil {
		return fmt.Errorf("expected cluster %s to be deleted, but still it is found", opts.ClusterName)
	}
	return nil
}

func verifyRBACResourcesDeleted(clusterKubeClient kubeclient.Interface, unjoiningClusterName string) error {
	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	clusterRoleName := names.GenerateRoleName(serviceAccountName)
	clusterRoleBindingName := clusterRoleName
	if err := clusterKubeClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), clusterRoleBindingName, metav1.DeleteOptions{}); err == nil {
		return fmt.Errorf("expected cluster role binding %s in unjoining cluster %s to be deleted, but it is still found", clusterRoleBindingName, unjoiningClusterName)
	}
	if err := clusterKubeClient.RbacV1().ClusterRoles().Delete(context.TODO(), clusterRoleName, metav1.DeleteOptions{}); err == nil {
		return fmt.Errorf("expected cluster role name %s in unjoining cluster %s to be deleted, but it is still found", clusterRoleName, unjoiningClusterName)
	}
	return nil
}

func verifyServiceAccountDeleted(clusterKubeClient kubeclient.Interface, unjoiningClusterName, namespace string) error {
	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	if err := clusterKubeClient.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), serviceAccountName, metav1.DeleteOptions{}); err == nil {
		return fmt.Errorf("expected service account %s in unjoining cluster %s to be deleted, but it is still found", serviceAccountName, unjoiningClusterName)
	}
	return nil
}

func verifyNamespaceDeleted(clusterKubeClient kubeclient.Interface, unjoiningClusterName, namespace string) error {
	if err := clusterKubeClient.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{}); err == nil {
		return fmt.Errorf("expected namespace %s in unjoining cluster %s to be deleted, but it is still found", namespace, unjoiningClusterName)
	}
	return nil
}
