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

package join

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		joinOpts *CommandJoinOption
		args     []string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Validate_WithMoreThanOneArg_OnlyTheClusterNameIsRequired",
			joinOpts: &CommandJoinOption{},
			args:     []string{"cluster2", "cluster3"},
			wantErr:  true,
			errMsg:   "only the cluster name is allowed as an argument",
		},
		{
			name:     "Validate_WithoutClusterNameToJoinWith_ClusterNameIsRequired",
			joinOpts: &CommandJoinOption{ClusterName: ""},
			args:     []string{"cluster2"},
			wantErr:  true,
			errMsg:   "cluster name is required",
		},
		{
			name:     "Validate_ClusterNameExceedsTheMaxLength_ClusterNameIsInvalid",
			joinOpts: &CommandJoinOption{ClusterName: strings.Repeat("a", 49)},
			args:     []string{"cluster2"},
			wantErr:  true,
			errMsg:   "invalid cluster name",
		},
		{
			name: "Validate_WithNameSpaceKarmadaSystem_WarningIssuedAndValidated",
			joinOpts: &CommandJoinOption{
				ClusterName:      "cluster1",
				ClusterNamespace: names.NamespaceKarmadaSystem,
			},
			args:    []string{"cluster2"},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.joinOpts.Validate(test.args)
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

func TestRunJoinCluster(t *testing.T) {
	tests := []struct {
		name                                 string
		clusterName                          string
		clusterID                            types.UID
		joinOpts                             *CommandJoinOption
		controlPlaneRestCfg, clusterCfg      *rest.Config
		controlKubeClient, clusterKubeClient kubeclient.Interface
		karmadaClient                        karmadaclientset.Interface
		prep                                 func(karmadaClient karmadaclientset.Interface, controlKubeClient kubeclient.Interface, clusterKubeClient kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID, clusterName string) error
		verify                               func(karmadaClient karmadaclientset.Interface, controlKubeClient kubeclient.Interface, clusterKubeClint kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID) error
		wantErr                              bool
		errMsg                               func(opts *CommandJoinOption, clusterID types.UID) string
	}{
		{
			name:                "RunJoinCluster_RegisterTheSameClusterWithSameID_TheSameClusterHasBeenRegistered",
			clusterName:         "member1",
			joinOpts:            &CommandJoinOption{},
			controlPlaneRestCfg: &rest.Config{},
			clusterCfg:          &rest.Config{},
			clusterKubeClient:   fakeclientset.NewClientset(),
			karmadaClient:       fakekarmadaclient.NewSimpleClientset(),
			clusterID:           types.UID(uuid.New().String()),
			prep: func(karmadaClient karmadaclientset.Interface, _, clusterKubeClient kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID, clusterName string) error {
				opts.ClusterName = clusterName
				if err := createNamespace(metav1.NamespaceSystem, clusterID, clusterKubeClient); err != nil {
					return err
				}
				if err := createCluster(clusterName, clusterID, karmadaClient); err != nil {
					return err
				}
				clusterKubeClientBuilder = func(*rest.Config) kubeclient.Interface {
					return clusterKubeClient
				}
				karmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
					return karmadaClient
				}
				return nil
			},
			verify: func(karmadaclientset.Interface, kubeclient.Interface, kubeclient.Interface, *CommandJoinOption, types.UID) error {
				return nil
			},
			wantErr: true,
			errMsg: func(opts *CommandJoinOption, clusterID types.UID) string {
				return fmt.Sprintf("the cluster ID %s or the cluster name %s has been registered", clusterID, opts.ClusterName)
			},
		},
		{
			name: "RunJoinCluster_RegisterClusterInControllerPlane_ClusterRegisteredInControllerPlane",
			joinOpts: &CommandJoinOption{
				ClusterNamespace: options.DefaultKarmadaClusterNamespace,
			},
			controlPlaneRestCfg: &rest.Config{},
			clusterCfg:          &rest.Config{},
			controlKubeClient:   fakeclientset.NewClientset(),
			clusterKubeClient:   fakeclientset.NewClientset(),
			karmadaClient:       fakekarmadaclient.NewSimpleClientset(),
			clusterID:           types.UID(uuid.New().String()),
			prep: func(karmadaClient karmadaclientset.Interface, controlKubeClient, clusterKubeClient kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID, clusterName string) error {
				return prepJoinCluster(karmadaClient, controlKubeClient, clusterKubeClient, opts, clusterID, clusterName)
			},
			verify: func(karmadaClient karmadaclientset.Interface, controlKubeClient, clusterKubeClient kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID) error {
				return verifyJoinCluster(karmadaClient, controlKubeClient, clusterKubeClient, opts, clusterID)
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.karmadaClient, test.controlKubeClient, test.clusterKubeClient, test.joinOpts, test.clusterID, test.clusterName); err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			err := test.joinOpts.RunJoinCluster(test.controlPlaneRestCfg, test.clusterCfg)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg(test.joinOpts, test.clusterID)) {
				t.Errorf("expected error message %s to be in %s", test.errMsg(test.joinOpts, test.clusterID), err.Error())
			}
			if err := test.verify(test.karmadaClient, test.controlKubeClient, test.clusterKubeClient, test.joinOpts, test.clusterID); err != nil {
				t.Errorf("failed to verify joining the cluster, got error: %v", err)
			}
		})
	}
}

// createNamespace creates a Kubernetes namespace with the specified name and clusterID.
func createNamespace(name string, clusterID types.UID, client kubeclient.Interface) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  clusterID,
		},
	}
	ns, err := client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s, got error: %v", ns.GetName(), err)
	}
	return nil
}

// createCluster creates a Karmada cluster resource with the specified name and clusterID.
func createCluster(name string, clusterID types.UID, karmadaClient karmadaclientset.Interface) error {
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterv1alpha1.ClusterSpec{
			ID: string(clusterID),
		},
	}
	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create cluster %s, got error: %v", cluster.GetName(), err)
	}
	return nil
}

// prepJoinCluster prepares the cluster for joining Karmada by creating required namespaces, secrets, and service accounts.
func prepJoinCluster(karmadaClient karmadaclientset.Interface, controlKubeClient, clusterKubeClient kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID, clusterName string) error {
	opts.ClusterName = clusterName
	if err := createNamespace(metav1.NamespaceSystem, clusterID, clusterKubeClient); err != nil {
		return err
	}

	name := names.GenerateServiceAccountName("impersonator")
	if err := createSecret(
		name, opts.ClusterNamespace, map[string]string{corev1.ServiceAccountNameKey: name},
		corev1.SecretTypeServiceAccountToken, map[string][]byte{"token": []byte("test-token-12345")}, clusterKubeClient,
	); err != nil {
		return err
	}

	name = names.GenerateServiceAccountName(opts.ClusterName)
	if err := createSecret(
		name, opts.ClusterNamespace, map[string]string{corev1.ServiceAccountNameKey: name},
		corev1.SecretTypeServiceAccountToken, map[string][]byte{"token": []byte("test-token-123456")}, clusterKubeClient,
	); err != nil {
		return err
	}

	clusterKubeClientBuilder = func(*rest.Config) kubeclient.Interface {
		return clusterKubeClient
	}
	karmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
		return karmadaClient
	}
	controlPlaneKubeClientBuilder = func(*rest.Config) kubeclient.Interface {
		return controlKubeClient
	}
	return nil
}

// createSecret creates a Kubernetes secret in the specified namespace with the provided data and annotations.
func createSecret(name, namespace string, annotations map[string]string, secretType corev1.SecretType, data map[string][]byte, client kubeclient.Interface) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Type: secretType,
		Data: data,
	}
	if secret, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create secret %s, got error: %v", secret.GetName(), err)
	}
	return nil
}

// verifyJoinCluster verifies the resources created during the join process, such as service accounts, secrets, and the cluster resource.
func verifyJoinCluster(karmadaClient karmadaclientset.Interface, controlKubeClient, clusterKubeClient kubeclient.Interface, opts *CommandJoinOption, clusterID types.UID) error {
	// Verify impersonator service account created on the clusterkubeclient.
	saName := names.GenerateServiceAccountName("impersonator")
	if _, err := clusterKubeClient.CoreV1().ServiceAccounts(opts.ClusterNamespace).Get(context.TODO(), saName, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("failed to get service account %s, got error: %v", saName, err)
	}

	// Verify cluster service account created on the clusterkubeclient.
	saName = names.GenerateServiceAccountName(opts.ClusterName)
	if _, err := clusterKubeClient.CoreV1().ServiceAccounts(opts.ClusterNamespace).Get(context.TODO(), saName, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("failed to get service account %s, got error: %v", saName, err)
	}

	// Verify impersonator secret created on the controlplane kubeclient.
	secretName := names.GenerateImpersonationSecretName(opts.ClusterName)
	if _, err := controlKubeClient.CoreV1().Secrets(opts.ClusterNamespace).Get(context.TODO(), secretName, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("failed to get secret %s, got error: %v", secretName, err)
	}

	// Verify secret created on the controlplane kubeclient.
	secretName = opts.ClusterName
	if _, err := controlKubeClient.CoreV1().Secrets(opts.ClusterNamespace).Get(context.TODO(), secretName, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("failed to get secret %s, got error: %v", secretName, err)
	}

	// Verify new cluster created on the controlplane karmadaclient.
	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get cluster name %s, got error: %v", opts.ClusterName, err)
	}

	if cluster.Spec.ID != string(clusterID) {
		return fmt.Errorf("expected cluster ID to be %s, but got %s", string(clusterID), cluster.Spec.ID)
	}

	secretRefExpected := &clusterv1alpha1.LocalSecretReference{
		Namespace: options.DefaultKarmadaClusterNamespace,
		Name:      opts.ClusterName,
	}
	if !reflect.DeepEqual(cluster.Spec.SecretRef, secretRefExpected) {
		return fmt.Errorf("expected secret ref %v to be equal to %v", secretRefExpected, cluster.Spec.SecretRef)
	}

	impersonatorSecretRefExpected := &clusterv1alpha1.LocalSecretReference{
		Namespace: options.DefaultKarmadaClusterNamespace,
		Name:      names.GenerateImpersonationSecretName(opts.ClusterName),
	}
	if !reflect.DeepEqual(cluster.Spec.ImpersonatorSecretRef, impersonatorSecretRefExpected) {
		return fmt.Errorf("expected impersonator secret ref %v to be equal to %v", impersonatorSecretRefExpected, cluster.Spec.ImpersonatorSecretRef)
	}

	return nil
}
