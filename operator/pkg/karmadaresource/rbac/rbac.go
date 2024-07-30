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
	if err := grantKarmadaResourceViewClusterrole(client); err != nil {
		return err
	}
	return grantKarmadaResourceEditClusterrole(client)
}

func grantKarmadaResourceViewClusterrole(client clientset.Interface) error {
	role := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(KarmadaResourceViewClusterRole), role); err != nil {
		return fmt.Errorf("err when decoding Karmada view Clusterrole: %w", err)
	}
	util.MergeLabel(role, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)
	return apiclient.CreateOrUpdateClusterRole(client, role)
}

func grantKarmadaResourceEditClusterrole(client clientset.Interface) error {
	role := &rbacv1.ClusterRole{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), []byte(KarmadaResourceEditClusterRole), role); err != nil {
		return fmt.Errorf("err when decoding Karmada edit Clusterrole: %w", err)
	}
	util.MergeLabel(role, util.KarmadaSystemLabel, util.KarmadaSystemLabelValue)
	return apiclient.CreateOrUpdateClusterRole(client, role)
}
