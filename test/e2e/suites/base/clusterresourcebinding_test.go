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

package base

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("ClusterResourceBinding test", func() {
	ginkgo.Context("permanent id label testing", func() {
		var (
			policy                  *policyv1alpha1.ClusterPropagationPolicy
			clusterRole             *rbacv1.ClusterRole
			bindingName             string
			hasPermanentIDLabelFunc = func(labels map[string]string) bool {
				return len(labels) > 0 && labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel] != ""
			}
		)
		ginkgo.BeforeEach(func() {
			policyName := clusterRoleNamePrefix + rand.String(RandomStrLength)
			clusterRoleName := fmt.Sprintf("system:test-%s", policyName)

			clusterRole = testhelper.NewClusterRole(clusterRoleName, nil)
			policy = testhelper.NewClusterPropagationPolicy(policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: clusterRole.APIVersion,
					Kind:       clusterRole.Kind,
					Name:       clusterRole.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
			})

			bindingName = names.GenerateBindingName(clusterRole.Kind, clusterRole.Name)
		})

		ginkgo.BeforeEach(func() {
			framework.CreateClusterPropagationPolicy(karmadaClient, policy)
			framework.CreateClusterRole(kubeClient, clusterRole)
			ginkgo.DeferCleanup(func() {
				framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
				framework.RemoveClusterRole(kubeClient, clusterRole.Name)
				framework.WaitClusterRoleDisappearOnClusters(framework.ClusterNames(), clusterRole.Name)
			})
		})

		ginkgo.BeforeEach(func() {
			framework.WaitClusterResourceBindingFitWith(karmadaClient, bindingName, func(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) bool {
				return hasPermanentIDLabelFunc(clusterResourceBinding.Labels)
			})
		})

		ginkgo.It("creates work with permanent ID label", func() {
			framework.WaitClusterResourceBindingFitWith(karmadaClient, bindingName, func(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) bool {
				workList, err := helper.GetWorksByLabelsSet(context.Background(), controlPlaneClient,
					labels.Set{
						workv1alpha2.ClusterResourceBindingPermanentIDLabel: clusterResourceBinding.Labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel],
					},
				)
				if err != nil {
					return false
				}

				return workList != nil && len(workList.Items) == len(framework.ClusterNames())
			})
		})

		ginkgo.It("propagates resource with permanent ID label", func() {
			framework.WaitClusterRolePresentOnClustersFitWith(framework.ClusterNames(), clusterRole.Name, func(clusterRole *rbacv1.ClusterRole) bool {
				return hasPermanentIDLabelFunc(clusterRole.Labels)
			})
		})
	})
})
