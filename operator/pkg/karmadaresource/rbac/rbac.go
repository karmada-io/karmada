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

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
	"github.com/karmada-io/karmada/pkg/util"
)

// EnsureKarmadaRBAC create karmada resource view and edit clusterrole
func EnsureKarmadaRBAC(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents) error {
	if err := grantClusterProxyAdminRBAC(client, cfg); err != nil {
		return err
	}
	if err := grantKarmadaResourceViewClusterRole(client, cfg); err != nil {
		return err
	}
	return grantKarmadaResourceEditClusterRole(client, cfg)
}

func grantClusterProxyAdminRBAC(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents) error {
	role := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(ClusterProxyAdminClusterRole), role); err != nil {
		return fmt.Errorf("err when decoding ClusterProxyAdmin ClusterRole: %w", err)
	}
	util.MergeLabel(role, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)

	// Apply labels from configuration if available
	if cfg != nil && cfg.KarmadaAPIServer != nil {
		patcher.NewPatcher().WithLabels(cfg.KarmadaAPIServer.Labels).ForClusterRole(role)
	}

	if err := apiclient.CreateOrUpdateClusterRole(client, role); err != nil {
		return fmt.Errorf("failed to create or update ClusterRole: %w", err)
	}

	roleBinding := &rbacv1.ClusterRoleBinding{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(ClusterProxyAdminClusterRoleBinding), roleBinding); err != nil {
		return fmt.Errorf("err when decoding ClusterProxyAdmin ClusterRoleBinding: %w", err)
	}
	util.MergeLabel(roleBinding, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)

	// Apply labels from configuration if available
	if cfg != nil && cfg.KarmadaAPIServer != nil {
		patcher.NewPatcher().WithLabels(cfg.KarmadaAPIServer.Labels).ForClusterRoleBinding(roleBinding)
	}

	return apiclient.CreateOrUpdateClusterRoleBinding(client, roleBinding)
}

func grantKarmadaResourceViewClusterRole(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents) error {
	role := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(KarmadaResourceViewClusterRole), role); err != nil {
		return fmt.Errorf("err when decoding Karmada view ClusterRole: %w", err)
	}
	util.MergeLabel(role, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)

	// Apply labels from configuration if available
	if cfg != nil && cfg.KarmadaAPIServer != nil {
		patcher.NewPatcher().WithLabels(cfg.KarmadaAPIServer.Labels).ForClusterRole(role)
	}

	return apiclient.CreateOrUpdateClusterRole(client, role)
}

func grantKarmadaResourceEditClusterRole(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents) error {
	role := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(KarmadaResourceEditClusterRole), role); err != nil {
		return fmt.Errorf("err when decoding Karmada edit ClusterRole: %w", err)
	}
	util.MergeLabel(role, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)

	// Apply labels from configuration if available
	if cfg != nil && cfg.KarmadaAPIServer != nil {
		patcher.NewPatcher().WithLabels(cfg.KarmadaAPIServer.Labels).ForClusterRole(role)
	}

	return apiclient.CreateOrUpdateClusterRole(client, role)
}
