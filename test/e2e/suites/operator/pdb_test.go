/*
Copyright 2025 The Karmada Authors.

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

package e2e

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	operatorresource "github.com/karmada-io/karmada/test/e2e/framework/resource/operator"
	"github.com/karmada-io/karmada/test/helper"
)

var deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
var statefulSetGVK = appsv1.SchemeGroupVersion.WithKind("StatefulSet")

var _ = ginkgo.Describe("PodDisruptionBudget configuration testing", func() {
	var karmadaName string
	var karmada *operatorv1alpha1.Karmada
	var err error

	var minAvailable intstr.IntOrString
	var maxUnavailable intstr.IntOrString

	ginkgo.BeforeEach(func() {
		karmadaName = KarmadaInstanceNamePrefix + rand.String(RandomStrLength)
		karmada = helper.NewKarmada(testNamespace, karmadaName)
	})

	ginkgo.Context("PodDisruptionBudget validation testing", func() {
		ginkgo.It("Should fail to deploy Karmada with invalid PDB configuration when minAvailable or maxUnavailable are both empty", func() {
			karmada.Spec.Components.KarmadaControllerManager.CommonSettings.PodDisruptionBudgetConfig = &operatorv1alpha1.PodDisruptionBudgetConfig{}
			err = operatorresource.CreateKarmadaInstance(operatorClient, karmada)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(err.Error()).Should(gomega.ContainSubstring("either minAvailable or maxUnavailable must be set"))
		})

		ginkgo.It("Should fail to deploy Karmada with invalid PDB configuration when both minAvailable and maxUnavailable are set", func() {
			minAvailable = intstr.FromInt32(1)
			maxUnavailable = intstr.FromInt32(1)
			karmada.Spec.Components.KarmadaControllerManager.CommonSettings.PodDisruptionBudgetConfig = &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable:   &minAvailable,
				MaxUnavailable: &maxUnavailable,
			}
			err = operatorresource.CreateKarmadaInstance(operatorClient, karmada)
			gomega.Expect(err).Should(gomega.HaveOccurred())
			gomega.Expect(err.Error()).Should(gomega.ContainSubstring("minAvailable and maxUnavailable are mutually exclusive"))
		})
	})

	ginkgo.Context("PodDisruptionBudget functionality testing", func() {
		ginkgo.It("should handle PDB correctly when podDisruptionBudgetConfig is specified in CommonSettings", func() {
			var cmPDBName string
			var cmPDB *policyv1.PodDisruptionBudget
			var etcdPDBName string
			var etcdPDB *policyv1.PodDisruptionBudget

			ginkgo.By("deploy a Karmada instance with valid PDB configuration", func() {
				minAvailable = intstr.FromInt32(1)
				pdbconfig := &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &minAvailable,
				}
				karmada.Spec.Components.Etcd.Local = &operatorv1alpha1.LocalEtcd{
					CommonSettings: operatorv1alpha1.CommonSettings{
						PodDisruptionBudgetConfig: pdbconfig,
					},
				}
				karmada.Spec.Components.KarmadaAPIServer.CommonSettings.PodDisruptionBudgetConfig = pdbconfig
				karmada.Spec.Components.KarmadaControllerManager.CommonSettings.PodDisruptionBudgetConfig = pdbconfig
				karmada.Spec.Components.KarmadaAggregatedAPIServer.CommonSettings.PodDisruptionBudgetConfig = pdbconfig
				karmada.Spec.Components.KarmadaScheduler.CommonSettings.PodDisruptionBudgetConfig = pdbconfig
				karmada.Spec.Components.KarmadaWebhook.CommonSettings.PodDisruptionBudgetConfig = pdbconfig
				karmada.Spec.Components.KarmadaMetricsAdapter.CommonSettings.PodDisruptionBudgetConfig = pdbconfig

				err = operatorresource.CreateKarmadaInstance(operatorClient, karmada)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				operatorresource.WaitKarmadaReady(operatorClient, testNamespace, karmadaName, time.Now())
			})

			ginkgo.By("check if PDB of component etcd is created successfully", func() {
				etcdPDBName = util.KarmadaEtcdName(karmadaName)
				etcdPDB, err = hostClient.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.TODO(), etcdPDBName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(etcdPDB.Spec.MinAvailable.IntValue()).Should(gomega.Equal(1))
				gomega.Expect(etcdPDB.Spec.MaxUnavailable == nil).Should(gomega.BeTrue())

				var etcd *appsv1.StatefulSet
				etcd, err = hostClient.AppsV1().StatefulSets(testNamespace).Get(context.TODO(), etcdPDBName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(etcdPDB.Labels).Should(gomega.Equal(etcd.Spec.Template.Labels))
				gomega.Expect(etcdPDB.Spec.Selector.MatchLabels).Should(gomega.Equal(etcd.Spec.Template.Labels))
				gomega.Expect(len(etcdPDB.ObjectMeta.OwnerReferences)).Should(gomega.Equal(1))
				// object etcd has no gvk info, so cannot use etcd.GroupVersionKind() to get StatefulSet gvk.
				expectedOwnerReferences := *metav1.NewControllerRef(etcd, statefulSetGVK)
				expectedOwnerReferences.Controller = ptr.To(true)
				expectedOwnerReferences.BlockOwnerDeletion = ptr.To(true)
				gomega.Expect(etcdPDB.ObjectMeta.OwnerReferences[0]).Should(gomega.Equal(expectedOwnerReferences))
			})

			ginkgo.By("check if PDB of component KarmadaControllerManager is created successfully", func() {
				cmPDBName = util.KarmadaControllerManagerName(karmadaName)
				cmPDB, err = hostClient.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.TODO(), cmPDBName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(cmPDB.Spec.MinAvailable.IntValue()).Should(gomega.Equal(1))
				gomega.Expect(cmPDB.Spec.MaxUnavailable == nil).Should(gomega.BeTrue())

				var karmadaControllerManager *appsv1.Deployment
				karmadaControllerManager, err = hostClient.AppsV1().Deployments(testNamespace).Get(context.TODO(), cmPDBName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(cmPDB.Labels).Should(gomega.Equal(karmadaControllerManager.Spec.Template.Labels))
				gomega.Expect(cmPDB.Spec.Selector.MatchLabels).Should(gomega.Equal(karmadaControllerManager.Spec.Template.Labels))
				gomega.Expect(len(cmPDB.ObjectMeta.OwnerReferences)).Should(gomega.Equal(1))
				// object karmadaControllerManager has no gvk info, so cannot use karmadaControllerManager.GroupVersionKind() to get Deployment gvk.
				expectedOwnerReferences := *metav1.NewControllerRef(karmadaControllerManager, deploymentGVK)
				expectedOwnerReferences.Controller = ptr.To(true)
				expectedOwnerReferences.BlockOwnerDeletion = ptr.To(true)
				gomega.Expect(cmPDB.ObjectMeta.OwnerReferences[0]).Should(gomega.Equal(expectedOwnerReferences))
			})

			ginkgo.By("set PodDisruptionBudgetConfig.MaxUnavailable to 1", func() {
				karmada, err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Get(context.TODO(), karmadaName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				maxUnavailable = intstr.FromInt32(1)
				karmada.Spec.Components.Etcd.Local.CommonSettings.PodDisruptionBudgetConfig = &operatorv1alpha1.PodDisruptionBudgetConfig{
					MaxUnavailable: &maxUnavailable,
				}
				karmada.Spec.Components.KarmadaControllerManager.CommonSettings.PodDisruptionBudgetConfig = &operatorv1alpha1.PodDisruptionBudgetConfig{
					MaxUnavailable: &maxUnavailable,
				}
				operatorresource.UpdateKarmadaInstanceWithSpec(operatorClient, testNamespace, karmadaName, karmada.Spec)
				operatorresource.WaitKarmadaReady(operatorClient, testNamespace, karmadaName, operatorresource.GetLastTransitionTime(karmada, operatorv1alpha1.Ready))
			})

			ginkgo.By("check if PDB of etcd is updated successfully", func() {
				etcdPDB, err = hostClient.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.TODO(), etcdPDBName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(etcdPDB.Spec.MinAvailable == nil).Should(gomega.BeTrue())
				gomega.Expect(etcdPDB.Spec.MaxUnavailable.IntValue()).Should(gomega.Equal(1))
			})

			ginkgo.By("check if PDB of KarmadaControllerManager is updated successfully", func() {
				cmPDB, err = hostClient.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.TODO(), cmPDBName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				gomega.Expect(cmPDB.Spec.MinAvailable == nil).Should(gomega.BeTrue())
				gomega.Expect(cmPDB.Spec.MaxUnavailable.IntValue()).Should(gomega.Equal(1))
			})

			ginkgo.By("remove PodDisruptionBudgetConfig", func() {
				karmada, err = operatorClient.OperatorV1alpha1().Karmadas(testNamespace).Get(context.TODO(), karmadaName, metav1.GetOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				karmada.Spec.Components.Etcd.Local.CommonSettings.PodDisruptionBudgetConfig = nil
				karmada.Spec.Components.KarmadaControllerManager.CommonSettings.PodDisruptionBudgetConfig = nil
				operatorresource.UpdateKarmadaInstanceWithSpec(operatorClient, testNamespace, karmadaName, karmada.Spec)
				operatorresource.WaitKarmadaReady(operatorClient, testNamespace, karmadaName, operatorresource.GetLastTransitionTime(karmada, operatorv1alpha1.Ready))
			})

			ginkgo.By("check if PDB is deleted", func() {
				gomega.Eventually(func() bool {
					_, err := hostClient.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.TODO(), etcdPDBName, metav1.GetOptions{})
					if !apierrors.IsNotFound(err) {
						return false
					}

					_, err = hostClient.PolicyV1().PodDisruptionBudgets(testNamespace).Get(context.TODO(), cmPDBName, metav1.GetOptions{})
					return apierrors.IsNotFound(err)
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})
})
