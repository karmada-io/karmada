/*
Copyright 2026 The Karmada Authors.

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
	"sort"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func eventuallyGetBindingClusters(namespace, name string) []string {
	var clusters []string
	bindingName := names.GenerateBindingName("Deployment", name)
	gomega.Eventually(func() error {
		binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(context.TODO(), bindingName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		c := make([]string, 0, len(binding.Spec.Clusters))
		for _, sc := range binding.Spec.Clusters {
			c = append(c, sc.Name)
		}
		sort.Strings(c)
		if len(c) == 0 {
			for _, cond := range binding.Status.Conditions {
				if cond.Type == workv1alpha2.Scheduled && cond.Reason == workv1alpha2.BindingReasonNoClusterFit {
					clusters = c
					return nil
				}
			}
			return context.DeadlineExceeded
		}
		clusters = c
		return nil
	}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
	return clusters
}

var _ = ginkgo.Describe("[WorkloadAffinity] inter-workload affinity/anti-affinity scheduling", func() {
	ginkgo.When("Same-group workloads schedule to different clusters", func() {
		var (
			dep1, dep2    *appsv1.Deployment
			policy        *policyv1alpha1.PropagationPolicy
			groupLabelKey string
			groupValue    string
		)

		ginkgo.BeforeEach(func() {
			groupLabelKey = "karmada.io/group-" + rand.String(RandomStrLength)
			groupValue = "ha"

			dep1 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep1.Labels = map[string]string{groupLabelKey: groupValue}
			dep2 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep2.Labels = map[string]string{groupLabelKey: groupValue}

			policy = testhelper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{groupLabelKey: groupValue},
					},
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: groupLabelKey},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				})
		})

		ginkgo.It("Target cluster sets of same-group workloads do not equal", func() {
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, dep1.Namespace, dep1.Name)
				framework.RemoveDeployment(kubeClient, dep2.Namespace, dep2.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, dep1)
			// TODO: the waiting logic can be removed after the scheduler builds
			// a cache to get other binding objects in a timely manner
			c1 := eventuallyGetBindingClusters(dep1.Namespace, dep1.Name)
			gomega.Expect(len(c1)).To(gomega.Equal(1))
			framework.WaitDeploymentPresentOnClusterFitWith(c1[0], dep1.Namespace, dep1.Name, func(*appsv1.Deployment) bool { return true })
			framework.CreateDeployment(kubeClient, dep2)

			c2 := eventuallyGetBindingClusters(dep2.Namespace, dep2.Name)
			gomega.Expect(len(c2)).To(gomega.Equal(1))
			gomega.Expect(c1[0]).ToNot(gomega.Equal(c2[0]))
		})
	})

	ginkgo.When("Same-group workloads co-locate on the same clusters", func() {
		var (
			dep1, dep2    *appsv1.Deployment
			policy        *policyv1alpha1.PropagationPolicy
			groupLabelKey string
			groupValue    string
		)

		ginkgo.BeforeEach(func() {
			groupLabelKey = "karmada.io/group-" + rand.String(RandomStrLength)
			groupValue = "train"

			dep1 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep1.Labels = map[string]string{groupLabelKey: groupValue}
			dep2 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep2.Labels = map[string]string{groupLabelKey: groupValue}

			policy = testhelper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{groupLabelKey: groupValue},
					},
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity: &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: groupLabelKey},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				})
		})

		ginkgo.It("Target cluster sets of same-group workloads are identical", func() {
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, dep1.Namespace, dep1.Name)
				framework.RemoveDeployment(kubeClient, dep2.Namespace, dep2.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			})

			framework.CreatePropagationPolicy(karmadaClient, policy)
			framework.CreateDeployment(kubeClient, dep1)
			// TODO: the waiting logic can be removed after the scheduler builds
			// a cache to get other binding objects in a timely manner
			c1 := eventuallyGetBindingClusters(dep1.Namespace, dep1.Name)
			gomega.Expect(len(c1)).To(gomega.Equal(1))
			framework.WaitDeploymentPresentOnClusterFitWith(c1[0], dep1.Namespace, dep1.Name, func(*appsv1.Deployment) bool { return true })

			framework.CreateDeployment(kubeClient, dep2)
			c2 := eventuallyGetBindingClusters(dep2.Namespace, dep2.Name)
			gomega.Expect(len(c2)).To(gomega.Equal(1))

			gomega.Expect(c1[0]).To(gomega.Equal(c2[0]))
		})
	})

	ginkgo.When("Same label across namespaces can co-locate", func() {
		var (
			ns2              string
			dep1, dep2       *appsv1.Deployment
			policy1, policy2 *policyv1alpha1.PropagationPolicy
			targetCluster    string
			groupLabelKey    string
			groupValue       string
		)

		ginkgo.BeforeEach(func() {
			ns2 = "karmada-e2e-affinity-" + rand.String(RandomStrLength)
			framework.CreateNamespace(kubeClient, testhelper.NewNamespace(ns2))
			framework.WaitNamespacePresentOnClusters(framework.ClusterNames(), ns2)
			ginkgo.DeferCleanup(func() {
				framework.RemoveNamespace(kubeClient, ns2)
			})

			targetCluster = framework.ClusterNames()[0]
			groupLabelKey = "karmada.io/group-" + rand.String(RandomStrLength)
			groupValue = "svc"

			dep1 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep1.Labels = map[string]string{groupLabelKey: groupValue}
			dep2 = testhelper.NewDeployment(ns2, deploymentNamePrefix+rand.String(RandomStrLength))
			dep2.Labels = map[string]string{groupLabelKey: groupValue}

			policy1 = testhelper.NewPropagationPolicy(dep1.Namespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{groupLabelKey: groupValue},
					},
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: groupLabelKey},
					},
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{targetCluster}},
				})
			policy2 = testhelper.NewPropagationPolicy(dep2.Namespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{groupLabelKey: groupValue},
					},
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: groupLabelKey},
					},
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{targetCluster}},
				})
		})

		ginkgo.It("Same-group workloads in different namespaces co-locate on the same cluster", func() {
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, dep1.Namespace, dep1.Name)
				framework.RemoveDeployment(kubeClient, dep2.Namespace, dep2.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy1.Namespace, policy1.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy2.Namespace, policy2.Name)
			})

			framework.CreatePropagationPolicy(karmadaClient, policy1)
			framework.CreateDeployment(kubeClient, dep1)
			// TODO: the waiting logic can be removed after the scheduler builds
			// a cache to get other binding objects in a timely manner
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, dep1.Namespace, dep1.Name, func(*appsv1.Deployment) bool { return true })

			framework.CreatePropagationPolicy(karmadaClient, policy2)
			framework.CreateDeployment(kubeClient, dep2)
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, dep2.Namespace, dep2.Name, func(*appsv1.Deployment) bool { return true })
		})
	})

	ginkgo.When("Scheduling behavior when both Affinity and AntiAffinity exist", func() {
		var (
			dep1, dep2, dep3                       *appsv1.Deployment
			policy1, policy2, policy3              *policyv1alpha1.PropagationPolicy
			affinityLabelKey, antiaffinityLabelKey string
			groupValue                             string
			targetCluster                          string
		)

		ginkgo.BeforeEach(func() {
			affinityLabelKey = "karmada.io/affinity-" + rand.String(RandomStrLength)
			antiaffinityLabelKey = "karmada.io/antiaffinity-" + rand.String(RandomStrLength)
			groupValue = "mix"
			dep1 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep1.Labels = map[string]string{affinityLabelKey: groupValue}
			dep2 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep2.Labels = map[string]string{antiaffinityLabelKey: groupValue}
			dep3 = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
			dep3.Labels = map[string]string{affinityLabelKey: groupValue, antiaffinityLabelKey: groupValue}

			targetCluster = framework.ClusterNames()[0]

			policy1 = testhelper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       dep1.Name,
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity: &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: affinityLabelKey},
					},
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{targetCluster}},
				})
			policy2 = testhelper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       dep2.Name,
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: antiaffinityLabelKey},
					},
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{targetCluster}},
				})
			policy3 = testhelper.NewPropagationPolicy(testNamespace, ppNamePrefix+rand.String(RandomStrLength),
				[]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       dep3.Name,
				}},
				policyv1alpha1.Placement{
					WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
						Affinity:     &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: affinityLabelKey},
						AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: antiaffinityLabelKey},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				})
		})

		ginkgo.It("One workload schedules successfully, the other remains unscheduled due to workloadAffinity", func() {
			ginkgo.DeferCleanup(func() {
				framework.RemoveDeployment(kubeClient, dep1.Namespace, dep1.Name)
				framework.RemoveDeployment(kubeClient, dep2.Namespace, dep2.Name)
				framework.RemoveDeployment(kubeClient, dep3.Namespace, dep3.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy1.Namespace, policy1.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy2.Namespace, policy2.Name)
				framework.RemovePropagationPolicy(karmadaClient, policy3.Namespace, policy3.Name)
			})

			framework.CreatePropagationPolicy(karmadaClient, policy1)
			framework.CreateDeployment(kubeClient, dep1)
			// TODO: the waiting logic can be removed after the scheduler builds
			// a cache to get other binding objects in a timely manner
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, dep1.Namespace, dep1.Name, func(*appsv1.Deployment) bool { return true })

			framework.CreatePropagationPolicy(karmadaClient, policy2)
			framework.CreateDeployment(kubeClient, dep2)
			// TODO: the waiting logic can be removed after the scheduler builds
			// a cache to get other binding objects in a timely manner
			framework.WaitDeploymentPresentOnClusterFitWith(targetCluster, dep2.Namespace, dep2.Name, func(*appsv1.Deployment) bool { return true })

			framework.CreatePropagationPolicy(karmadaClient, policy3)
			framework.CreateDeployment(kubeClient, dep3)
			c3 := eventuallyGetBindingClusters(dep3.Namespace, dep3.Name)
			gomega.Expect(c3).Should(gomega.BeEmpty())
		})
	})
})
