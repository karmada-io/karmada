/*
Copyright 2023 The Karmada Authors.

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

package rbac

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/pkg/util"
)

// EnsureKarmadaRBAC create karmada resource view and edit clusterrole
func EnsureKarmadaRBAC(client clientset.Interface) error {
	if err := grantClusterProxyAdminRBAC(client); err != nil {
		return err
	}
	if err := grantKarmadaResourceViewClusterRole(client); err != nil {
		return err
	}
	if err := grantKarmadaResourceEditClusterRole(client); err != nil {
		return err
	}
	return grantPullModeRBAC(client)
}

func grantClusterProxyAdminRBAC(client clientset.Interface) error {
	if err := grantRBACClusterRole(client, ClusterProxyAdminClusterRole); err != nil {
		return err
	}

	return grantRBACClusterRoleBinding(client, ClusterProxyAdminClusterRoleBinding)
}

func grantKarmadaResourceViewClusterRole(client clientset.Interface) error {
	return grantRBACClusterRole(client, KarmadaResourceViewClusterRole)
}

func grantKarmadaResourceEditClusterRole(client clientset.Interface) error {
	return grantRBACClusterRole(client, KarmadaResourceEditClusterRole)
}

func grantPullModeRBAC(client clientset.Interface) error {
	if err := grantRBACRole(client, ClusterInfoRole); err != nil {
		return err
	}
	if err := grantRBACRoleBinding(client, ClusterInfoRoleBinding); err != nil {
		return err
	}
	if err := grantRBACClusterRole(client, CSRAutoApproverClusterRole); err != nil {
		return err
	}
	if err := grantRBACClusterRoleBinding(client, CSRAutoApproverClusterRoleBinding); err != nil {
		return err
	}
	if err := grantRBACClusterRole(client, AgentBootstrapClusterRole); err != nil {
		return err
	}
	if err := grantRBACClusterRoleBinding(client, AgentBootstrapClusterRoleBinding); err != nil {
		return err
	}
	if err := grantRBACClusterRole(client, CSRSelfAutoApproverClusterRole); err != nil {
		return err
	}
	if err := grantRBACClusterRoleBinding(client, CSRSelfAutoApproverClusterRoleBinding); err != nil {
		return err
	}
	if err := grantRBACClusterRole(client, AgentRBACGeneratorClusterRole); err != nil {
		return err
	}

	return grantRBACClusterRoleBinding(client, AgentRBACGeneratorClusterRoleBinding)
}

func grantRBACRole(client clientset.Interface, role string) error {
	roleObject := &rbacv1.Role{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(role), roleObject); err != nil {
		return fmt.Errorf("err when decoding into Role object: %w", err)
	}
	util.MergeLabel(roleObject, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)
	return apiclient.CreateOrUpdateRole(client, roleObject)
}

func grantRBACRoleBinding(client clientset.Interface, roleBinding string) error {
	roleBindingObject := &rbacv1.RoleBinding{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(roleBinding), roleBindingObject); err != nil {
		return fmt.Errorf("err when decoding into RoleBinding object: %w", err)
	}
	util.MergeLabel(roleBindingObject, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)
	return apiclient.CreateOrUpdateRoleBinding(client, roleBindingObject)
}

func grantRBACClusterRole(client clientset.Interface, clusterRole string) error {
	clusterRoleObject := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(clusterRole), clusterRoleObject); err != nil {
		return fmt.Errorf("err when decoding into ClusterRole object: %w", err)
	}
	util.MergeLabel(clusterRoleObject, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)
	return apiclient.CreateOrUpdateClusterRole(client, clusterRoleObject)
}

func grantRBACClusterRoleBinding(client clientset.Interface, clusterRoleBinding string) error {
	clusterRoleBindingObject := &rbacv1.ClusterRoleBinding{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(clusterRoleBinding), clusterRoleBindingObject); err != nil {
		return fmt.Errorf("err when decoding into RoleBinding object: %w", err)
	}
	util.MergeLabel(clusterRoleBindingObject, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)
	return apiclient.CreateOrUpdateClusterRoleBinding(client, clusterRoleBindingObject)
}
