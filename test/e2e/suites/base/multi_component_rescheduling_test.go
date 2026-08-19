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
	_ "embed"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var (
	//go:embed manifest/rayclusters.ray.io-v1.yaml
	rayClusterCRDYAML string

	//go:embed manifest/raycluster-cr.yaml
	rayClusterCRYAML string
)

const (
	componentRevisionMarkerAnnotation = "e2e.karmada.io/component-revision"
	componentDeliveryFreezeWindow     = 10 * time.Second
)

const volcanoComponentRevision = `
function ReviseComponents(desiredObj, components)
  if desiredObj.spec == nil or desiredObj.spec.tasks == nil then
    error("Volcano Job has no tasks")
  end
  if components == nil or #components ~= #desiredObj.spec.tasks then
    error("expected one component result for every Volcano Job task")
  end

  local replicas = {}
  for i = 1, #components do
    local component = components[i]
    if component.name == nil or replicas[component.name] ~= nil then
      error("invalid or duplicate Volcano Job component")
    end
    replicas[component.name] = component.replicas
  end

  local total = 0
  for i = 1, #desiredObj.spec.tasks do
    local task = desiredObj.spec.tasks[i]
    local assigned = replicas[task.name]
    if assigned == nil then
      error("missing Volcano Job task component: " .. tostring(task.name))
    end
    task.minAvailable = assigned
    task.replicas = assigned
    total = total + assigned
    replicas[task.name] = nil
  end
  for name, _ in pairs(replicas) do
    error("unknown Volcano Job task component: " .. tostring(name))
  end
  desiredObj.spec.minAvailable = total
  if desiredObj.metadata.annotations == nil then
    desiredObj.metadata.annotations = {}
  end
  desiredObj.metadata.annotations["e2e.karmada.io/component-revision"] = "volcano"
  return desiredObj
end
`

const rayClusterComponentRevision = `
function ReviseComponents(desiredObj, components)
  if desiredObj.spec == nil or desiredObj.spec.workerGroupSpecs == nil then
    error("RayCluster has no worker groups")
  end
  if components == nil or #components ~= #desiredObj.spec.workerGroupSpecs + 1 then
    error("expected a complete RayCluster component result")
  end

  local replicas = {}
  for i = 1, #components do
    local component = components[i]
    if component.name == nil or replicas[component.name] ~= nil then
      error("invalid or duplicate RayCluster component")
    end
    replicas[component.name] = component.replicas
  end
  if replicas["ray-head"] ~= 1 then
    error("RayCluster head must have exactly one replica")
  end
  replicas["ray-head"] = nil

  for i = 1, #desiredObj.spec.workerGroupSpecs do
    local worker = desiredObj.spec.workerGroupSpecs[i]
    local name = worker.groupName
    if name == nil or name == "" then
      name = "worker-" .. tostring(i)
    end
    local assigned = replicas[name]
    if assigned == nil then
      error("missing RayCluster worker component: " .. name)
    end
    worker.replicas = assigned
    replicas[name] = nil
  end
  for name, _ in pairs(replicas) do
    error("unknown RayCluster worker component: " .. tostring(name))
  end
  if desiredObj.metadata.annotations == nil then
    desiredObj.metadata.annotations = {}
  end
  desiredObj.metadata.annotations["e2e.karmada.io/component-revision"] = "ray"
  return desiredObj
end
`

type componentE2EExpectation struct {
	replicas int32
	cpu      string
}

type componentE2EDeliveredRevision struct {
	memberUID                string
	memberGeneration         int64
	workUID                  string
	workGeneration           int64
	workManifest             string
	acceptedRequirementsHash string
}

func installMultiComponentCRD(crd *apiextensionsv1.CustomResourceDefinition, version string) {
	clusters := framework.ClusterNames()
	apiVersion := fmt.Sprintf("%s/%s", crd.Spec.Group, version)
	kind := crd.Spec.Names.Kind
	name := crd.Name

	framework.CreateCRD(dynamicClient, crd)
	ginkgo.DeferCleanup(func() {
		framework.RemoveCRD(dynamicClient, name)
		framework.WaitCRDDisappeared(dynamicClient, name)
		framework.WaitCRDDisappearedOnClusters(clusters, name)
		framework.WaitCRDDisappearedFromClusterStatus(karmadaClient, clusters, apiVersion, kind)
	})
	framework.WaitCRDEstablished(dynamicClient, name)

	policy := testhelper.NewClusterPropagationPolicy("multi-component-crd-"+rand.String(RandomStrLength), []policyv1alpha1.ResourceSelector{{
		APIVersion: crd.APIVersion,
		Kind:       crd.Kind,
		Name:       name,
	}}, policyv1alpha1.Placement{ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: clusters}})
	framework.CreateClusterPropagationPolicy(karmadaClient, policy)
	ginkgo.DeferCleanup(func() {
		framework.RemoveClusterPropagationPolicy(karmadaClient, policy.Name)
	})
	framework.WaitCRDPresentOnClusters(karmadaClient, clusters, apiVersion, kind)
}

func setupMultiComponentNamespace(namespace string) {
	gomega.Expect(setupTestNamespace(namespace, kubeClient)).Should(gomega.Succeed())
	ginkgo.DeferCleanup(func() {
		gomega.Expect(cleanupTestNamespace(namespace, kubeClient)).Should(gomega.Succeed())
	})
	framework.WaitNamespacePresentOnClusters(framework.ClusterNames(), namespace)
}

func limitMultiComponentCPU(ctx context.Context, namespace, quotaName, cpu string) {
	expectedCPU := resource.MustParse(cpu)
	for _, clusterName := range framework.ClusterNames() {
		clusterClient := framework.GetClusterClient(clusterName)
		gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
		framework.CreateResourceQuota(clusterClient, &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
				corev1.ResourceCPU: expectedCPU,
			}},
		})
		currentClusterName := clusterName
		ginkgo.DeferCleanup(func() {
			framework.RemoveResourceQuota(framework.GetClusterClient(currentClusterName), namespace, quotaName)
		})

		gomega.Eventually(func(g gomega.Gomega) {
			quota, err := clusterClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
			g.Expect(err).ShouldNot(gomega.HaveOccurred())
			actual, found := quota.Status.Hard[corev1.ResourceCPU]
			g.Expect(found).Should(gomega.BeTrue())
			g.Expect(actual.Cmp(expectedCPU)).Should(gomega.Equal(0))
		}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
	}
}

func setMultiComponentCPUQuota(ctx context.Context, clusterName, namespace, quotaName, cpu string) {
	expectedCPU := resource.MustParse(cpu)
	clusterClient := framework.GetClusterClient(clusterName)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
	gomega.Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		quota, err := clusterClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		quota.Spec.Hard[corev1.ResourceCPU] = expectedCPU
		_, err = clusterClient.CoreV1().ResourceQuotas(namespace).Update(ctx, quota, metav1.UpdateOptions{})
		return err
	})).Should(gomega.Succeed())

	gomega.Eventually(func(g gomega.Gomega) {
		quota, err := clusterClient.CoreV1().ResourceQuotas(namespace).Get(ctx, quotaName, metav1.GetOptions{})
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		actual, found := quota.Status.Hard[corev1.ResourceCPU]
		g.Expect(found).Should(gomega.BeTrue())
		g.Expect(actual.Cmp(expectedCPU)).Should(gomega.Equal(0))
	}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
}

func setMultiComponentClusterLabel(ctx context.Context, clusterName, key, value string, present bool) {
	gomega.Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(ctx, clusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cluster.Labels == nil {
			cluster.Labels = make(map[string]string)
		}
		if present {
			cluster.Labels[key] = value
		} else {
			delete(cluster.Labels, key)
		}
		_, err = karmadaClient.ClusterV1alpha1().Clusters().Update(ctx, cluster, metav1.UpdateOptions{})
		return err
	})).Should(gomega.Succeed())
}

func installComponentRevision(apiVersion, kind, script string) {
	customization := testhelper.NewResourceInterpreterCustomization(
		"multi-component-revision-"+rand.String(RandomStrLength),
		configv1alpha1.CustomizationTarget{APIVersion: apiVersion, Kind: kind},
		configv1alpha1.CustomizationRules{ComponentRevision: &configv1alpha1.ComponentRevision{LuaScript: script}},
	)
	framework.CreateResourceInterpreterCustomization(karmadaClient, customization)
	ginkgo.DeferCleanup(func() {
		framework.DeleteResourceInterpreterCustomization(karmadaClient, customization.Name)
	})
	// ResourceInterpreterCustomization has no observed status to wait on.
	time.Sleep(time.Second)
}

func createMultiComponentWorkload(ctx context.Context, manifest, namespace, name, resourceName string) (*unstructured.Unstructured, schema.GroupVersionResource) {
	workload := &unstructured.Unstructured{}
	gomega.Expect(yaml.Unmarshal([]byte(manifest), workload)).Should(gomega.Succeed())
	workload.SetNamespace(namespace)
	workload.SetName(name)
	gvr := schema.GroupVersionResource{
		Group: workload.GroupVersionKind().Group, Version: workload.GroupVersionKind().Version, Resource: resourceName,
	}
	created, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, workload, metav1.CreateOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	ginkgo.DeferCleanup(func() {
		err := dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		gomega.Expect(err == nil || apierrors.IsNotFound(err)).Should(gomega.BeTrue())
	})
	return created, gvr
}

func createMultiComponentPolicy(namespace string, workload *unstructured.Unstructured, clusters []string) {
	createMultiComponentPolicyWithSchedulingType(namespace, workload, clusters, policyv1alpha1.ReplicaSchedulingTypeDivided)
}

func createMultiComponentPolicyWithSchedulingType(namespace string, workload *unstructured.Unstructured, clusters []string,
	schedulingType policyv1alpha1.ReplicaSchedulingType,
) {
	policy := testhelper.NewPropagationPolicy(namespace, "multi-component-"+rand.String(RandomStrLength), []policyv1alpha1.ResourceSelector{{
		APIVersion: workload.GetAPIVersion(), Kind: workload.GetKind(), Name: workload.GetName(),
	}}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: clusters},
		SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}},
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     schedulingType,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
		},
	})
	framework.CreatePropagationPolicy(karmadaClient, policy)
	ginkgo.DeferCleanup(func() {
		framework.RemovePropagationPolicy(karmadaClient, namespace, policy.Name)
	})
}

func createMultiComponentLabelPolicy(namespace string, workload *unstructured.Unstructured, labelKey, labelValue string,
	schedulingType policyv1alpha1.ReplicaSchedulingType,
) {
	policy := testhelper.NewPropagationPolicy(namespace, "multi-component-"+rand.String(RandomStrLength), []policyv1alpha1.ResourceSelector{{
		APIVersion: workload.GetAPIVersion(), Kind: workload.GetKind(), Name: workload.GetName(),
	}}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}},
		SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}},
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     schedulingType,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
		},
	})
	framework.CreatePropagationPolicy(karmadaClient, policy)
	ginkgo.DeferCleanup(func() {
		framework.RemovePropagationPolicy(karmadaClient, namespace, policy.Name)
	})
}

func updateMultiComponentWorkload(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, mutate func(*unstructured.Unstructured) error) {
	gomega.Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err = mutate(current); err != nil {
			return err
		}
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, current, metav1.UpdateOptions{})
		return err
	})).Should(gomega.Succeed())
}

func waitForComponentSourceSnapshot(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, bindingName, previousSourceResourceVersion string, previousBindingGeneration int64) {
	gomega.Eventually(func(g gomega.Gomega) {
		source, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(source.GetResourceVersion()).ShouldNot(gomega.Equal(previousSourceResourceVersion))

		binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(binding.Generation).Should(gomega.BeNumerically(">", previousBindingGeneration))
		g.Expect(binding.Spec.Resource.ResourceVersion).Should(gomega.Equal(source.GetResourceVersion()))
	}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
}

func assertComponentBinding(ctx context.Context, g gomega.Gomega, namespace, bindingName, targetCluster string,
	desired, accepted map[string]componentE2EExpectation, status metav1.ConditionStatus, reason, message string,
) *workv1alpha2.ResourceBinding {
	binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(binding.Spec.Components).Should(gomega.HaveLen(len(desired)))
	seenDesired := make(map[string]struct{}, len(desired))
	for i := range binding.Spec.Components {
		component := binding.Spec.Components[i]
		expected, found := desired[component.Name]
		g.Expect(found).Should(gomega.BeTrue(), "unexpected desired component %q", component.Name)
		_, duplicate := seenDesired[component.Name]
		g.Expect(duplicate).Should(gomega.BeFalse(), "duplicate desired component %q", component.Name)
		seenDesired[component.Name] = struct{}{}
		g.Expect(component.Replicas).Should(gomega.Equal(expected.replicas))
		if expected.cpu != "" {
			g.Expect(component.ReplicaRequirements).ShouldNot(gomega.BeNil())
			actualCPU, found := component.ReplicaRequirements.ResourceRequest[corev1.ResourceCPU]
			g.Expect(found).Should(gomega.BeTrue())
			g.Expect(actualCPU.Cmp(resource.MustParse(expected.cpu))).Should(gomega.Equal(0))
		}
	}

	g.Expect(binding.Spec.Clusters).Should(gomega.HaveLen(1))
	if targetCluster != "" {
		g.Expect(binding.Spec.Clusters[0].Name).Should(gomega.Equal(targetCluster))
	}
	g.Expect(binding.Spec.Clusters[0].Components).Should(gomega.HaveLen(len(accepted)))
	seenAccepted := make(map[string]struct{}, len(accepted))
	for i := range binding.Spec.Clusters[0].Components {
		component := binding.Spec.Clusters[0].Components[i]
		expected, found := accepted[component.Name]
		g.Expect(found).Should(gomega.BeTrue(), "unexpected accepted component %q", component.Name)
		_, duplicate := seenAccepted[component.Name]
		g.Expect(duplicate).Should(gomega.BeFalse(), "duplicate accepted component %q", component.Name)
		seenAccepted[component.Name] = struct{}{}
		g.Expect(component.Replicas).Should(gomega.Equal(expected.replicas))
	}

	condition := meta.FindStatusCondition(binding.Status.Conditions, workv1alpha2.Scheduled)
	g.Expect(condition).ShouldNot(gomega.BeNil())
	g.Expect(condition.Status).Should(gomega.Equal(status))
	if reason != "" {
		g.Expect(condition.Reason).Should(gomega.Equal(reason))
	}
	if message != "" {
		g.Expect(condition.Message).Should(gomega.ContainSubstring(message))
	}
	if status == metav1.ConditionTrue {
		g.Expect(binding.Status.SchedulerObservedGeneration).Should(gomega.Equal(binding.Generation))
	} else {
		g.Expect(binding.Status.SchedulerObservedGeneration).Should(gomega.BeNumerically("<", binding.Generation))
	}
	return binding
}

func waitForComponentBinding(ctx context.Context, namespace, bindingName, targetCluster string,
	desired, accepted map[string]componentE2EExpectation, status metav1.ConditionStatus, reason, message string,
) *workv1alpha2.ResourceBinding {
	var binding *workv1alpha2.ResourceBinding
	gomega.Eventually(func(g gomega.Gomega) {
		binding = assertComponentBinding(ctx, g, namespace, bindingName, targetCluster, desired, accepted, status, reason, message)
	}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
	return binding
}

func assertComponentDelivery(ctx context.Context, g gomega.Gomega, gvr schema.GroupVersionResource, namespace, name, kind,
	targetCluster, bindingName string, assertObject func(gomega.Gomega, *unstructured.Unstructured),
) componentE2EDeliveredRevision {
	memberClient := framework.GetClusterDynamicClient(targetCluster)
	g.Expect(memberClient).ShouldNot(gomega.BeNil())
	memberObject, err := memberClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	assertObject(g, memberObject)

	workName := names.GenerateWorkName(kind, name, namespace)
	workNamespace := names.GenerateExecutionSpaceName(targetCluster)
	work, err := karmadaClient.WorkV1alpha1().Works(workNamespace).Get(ctx, workName, metav1.GetOptions{})
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(work.Spec.Workload.Manifests).Should(gomega.HaveLen(1))
	workload := &unstructured.Unstructured{}
	g.Expect(workload.UnmarshalJSON(work.Spec.Workload.Manifests[0].Raw)).Should(gomega.Succeed())
	assertObject(g, workload)

	binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	acceptedRequirementsHash := binding.Annotations[util.AcceptedComponentRequirementsHashAnnotation]
	g.Expect(acceptedRequirementsHash).ShouldNot(gomega.BeEmpty())

	return componentE2EDeliveredRevision{
		memberUID:                string(memberObject.GetUID()),
		memberGeneration:         memberObject.GetGeneration(),
		workUID:                  string(work.UID),
		workGeneration:           work.Generation,
		workManifest:             string(work.Spec.Workload.Manifests[0].Raw),
		acceptedRequirementsHash: acceptedRequirementsHash,
	}
}

func waitForComponentDelivery(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, kind, targetCluster, bindingName string,
	assertObject func(gomega.Gomega, *unstructured.Unstructured),
) componentE2EDeliveredRevision {
	var revision componentE2EDeliveredRevision
	gomega.Eventually(func(g gomega.Gomega) {
		revision = assertComponentDelivery(ctx, g, gvr, namespace, name, kind, targetCluster, bindingName, assertObject)
	}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
	return revision
}

func assertComponentDeliveryFrozen(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, kind, targetCluster, bindingName string,
	revision componentE2EDeliveredRevision, assertObject func(gomega.Gomega, *unstructured.Unstructured),
) {
	gomega.Consistently(func(g gomega.Gomega) {
		current := assertComponentDelivery(ctx, g, gvr, namespace, name, kind, targetCluster, bindingName, assertObject)
		g.Expect(current.memberUID).Should(gomega.Equal(revision.memberUID))
		g.Expect(current.memberGeneration).Should(gomega.Equal(revision.memberGeneration))
		g.Expect(current.workUID).Should(gomega.Equal(revision.workUID))
		g.Expect(current.workGeneration).Should(gomega.Equal(revision.workGeneration))
		g.Expect(current.workManifest).Should(gomega.Equal(revision.workManifest))
		g.Expect(current.acceptedRequirementsHash).Should(gomega.Equal(revision.acceptedRequirementsHash))
	}, componentDeliveryFreezeWindow, framework.PollInterval).Should(gomega.Succeed())
}

func waitForComponentDeliveryRemoved(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, kind, clusterName string) {
	workName := names.GenerateWorkName(kind, name, namespace)
	gomega.Eventually(func(g gomega.Gomega) {
		memberClient := framework.GetClusterDynamicClient(clusterName)
		g.Expect(memberClient).ShouldNot(gomega.BeNil())
		_, err := memberClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
		_, err = karmadaClient.WorkV1alpha1().Works(names.GenerateExecutionSpaceName(clusterName)).Get(ctx, workName, metav1.GetOptions{})
		g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
	}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
}

func assertComponentNotDeliveredElsewhere(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, kind, targetCluster string) {
	workName := names.GenerateWorkName(kind, name, namespace)
	for _, clusterName := range framework.ClusterNames() {
		if clusterName == targetCluster {
			continue
		}
		otherCluster := clusterName
		gomega.Consistently(func(g gomega.Gomega) {
			memberClient := framework.GetClusterDynamicClient(otherCluster)
			g.Expect(memberClient).ShouldNot(gomega.BeNil())
			_, err := memberClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
			_, err = karmadaClient.WorkV1alpha1().Works(names.GenerateExecutionSpaceName(otherCluster)).Get(ctx, workName, metav1.GetOptions{})
			g.Expect(apierrors.IsNotFound(err)).Should(gomega.BeTrue())
		}, 2*framework.PollInterval, framework.PollInterval).Should(gomega.Succeed())
	}
}

func triggerExplicitComponentRecovery(ctx context.Context, namespace, bindingName string) {
	gomega.Eventually(func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			binding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			triggeredAt := metav1.Now().Rfc3339Copy()
			if binding.Status.LastScheduledTime != nil && !triggeredAt.After(binding.Status.LastScheduledTime.Time) {
				return fmt.Errorf("current time has not advanced beyond lastScheduledTime")
			}
			binding.Spec.RescheduleTriggeredAt = &triggeredAt
			_, err = karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Update(ctx, binding, metav1.UpdateOptions{})
			return err
		})
	}, framework.PollTimeout, framework.PollInterval).Should(gomega.Succeed())
}

func setVolcanoTaskReplicas(object *unstructured.Unstructured, replicas map[string]int64) error {
	tasks, found, err := unstructured.NestedSlice(object.Object, "spec", "tasks")
	if err != nil || !found {
		return fmt.Errorf("get Volcano Job tasks: found=%t: %w", found, err)
	}
	seen := make(map[string]struct{}, len(replicas))
	var total int64
	for i := range tasks {
		task, ok := tasks[i].(map[string]any)
		if !ok {
			return fmt.Errorf("Volcano Job task %d is not an object", i)
		}
		name, _, _ := unstructured.NestedString(task, "name")
		assigned, exists := replicas[name]
		if !exists {
			return fmt.Errorf("missing replicas for Volcano Job task %q", name)
		}
		task["minAvailable"] = assigned
		task["replicas"] = assigned
		tasks[i] = task
		seen[name] = struct{}{}
		total += assigned
	}
	if len(seen) != len(replicas) {
		return fmt.Errorf("not every Volcano Job task replica was applied")
	}
	if err := unstructured.SetNestedSlice(object.Object, tasks, "spec", "tasks"); err != nil {
		return err
	}
	return unstructured.SetNestedField(object.Object, total, "spec", "minAvailable")
}

func assertVolcanoTaskReplicas(g gomega.Gomega, object *unstructured.Unstructured, expected map[string]int64) {
	g.Expect(object.GetAnnotations()).Should(gomega.HaveKeyWithValue(componentRevisionMarkerAnnotation, "volcano"))
	tasks, found, err := unstructured.NestedSlice(object.Object, "spec", "tasks")
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(found).Should(gomega.BeTrue())
	g.Expect(tasks).Should(gomega.HaveLen(len(expected)))
	var total int64
	for i := range tasks {
		task, ok := tasks[i].(map[string]any)
		g.Expect(ok).Should(gomega.BeTrue())
		name, found, err := unstructured.NestedString(task, "name")
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(found).Should(gomega.BeTrue())
		replicas, exists := expected[name]
		g.Expect(exists).Should(gomega.BeTrue())
		actualMin, found, err := unstructured.NestedInt64(task, "minAvailable")
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(found).Should(gomega.BeTrue())
		g.Expect(actualMin).Should(gomega.Equal(replicas))
		actualReplicas, found, err := unstructured.NestedInt64(task, "replicas")
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(found).Should(gomega.BeTrue())
		g.Expect(actualReplicas).Should(gomega.Equal(replicas))
		total += replicas
	}
	actualTotal, found, err := unstructured.NestedInt64(object.Object, "spec", "minAvailable")
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(found).Should(gomega.BeTrue())
	g.Expect(actualTotal).Should(gomega.Equal(total))
}

func rayWorkerGroup(name string, replicas int64, cpu string) map[string]any {
	return map[string]any{
		"groupName": name,
		"replicas":  replicas,
		"template": map[string]any{"spec": map[string]any{
			"containers": []any{map[string]any{
				"name":  "ray-worker",
				"image": "rayproject/ray:2.9.0",
				"resources": map[string]any{"requests": map[string]any{
					"cpu": cpu, "memory": "100Mi",
				}},
			}},
		}},
	}
}

func updateRayWorkerGroups(object *unstructured.Unstructured, mutate func([]any) ([]any, error)) error {
	groups, found, err := unstructured.NestedSlice(object.Object, "spec", "workerGroupSpecs")
	if err != nil || !found {
		return fmt.Errorf("get RayCluster worker groups: found=%t: %w", found, err)
	}
	groups, err = mutate(groups)
	if err != nil {
		return err
	}
	return unstructured.SetNestedSlice(object.Object, groups, "spec", "workerGroupSpecs")
}

func setRayWorkerReplicas(groups []any, replicas map[string]int64) ([]any, error) {
	seen := make(map[string]struct{}, len(replicas))
	for i := range groups {
		group, ok := groups[i].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("RayCluster worker group %d is not an object", i)
		}
		name, _, _ := unstructured.NestedString(group, "groupName")
		assigned, exists := replicas[name]
		if exists {
			group["replicas"] = assigned
			groups[i] = group
			seen[name] = struct{}{}
		}
	}
	if len(seen) != len(replicas) {
		return nil, fmt.Errorf("not every RayCluster worker replica was applied")
	}
	return groups, nil
}

func reverseRayWorkerGroups(groups []any) ([]any, error) {
	if len(groups) != 2 {
		return nil, fmt.Errorf("expected two RayCluster worker groups, got %d", len(groups))
	}
	groups[0], groups[1] = groups[1], groups[0]
	return groups, nil
}

func renameRayWorkerGroup(groups []any, oldName, newName string) ([]any, error) {
	found := false
	for i := range groups {
		group, ok := groups[i].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("RayCluster worker group %d is not an object", i)
		}
		name, _, _ := unstructured.NestedString(group, "groupName")
		if name == newName {
			return nil, fmt.Errorf("RayCluster worker group %q already exists", newName)
		}
		if name != oldName {
			continue
		}
		group["groupName"] = newName
		groups[i] = group
		found = true
	}
	if !found {
		return nil, fmt.Errorf("RayCluster worker group %q not found", oldName)
	}
	return groups, nil
}

func setRayWorkerCPU(groups []any, name, cpu string) ([]any, error) {
	for i := range groups {
		group, ok := groups[i].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("RayCluster worker group %d is not an object", i)
		}
		groupName, _, _ := unstructured.NestedString(group, "groupName")
		if groupName != name {
			continue
		}
		containers, found, err := unstructured.NestedSlice(group, "template", "spec", "containers")
		if err != nil || !found || len(containers) == 0 {
			return nil, fmt.Errorf("get RayCluster worker %q containers: found=%t: %w", name, found, err)
		}
		container, ok := containers[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("RayCluster worker %q container is not an object", name)
		}
		if err := unstructured.SetNestedField(container, cpu, "resources", "requests", "cpu"); err != nil {
			return nil, err
		}
		containers[0] = container
		if err := unstructured.SetNestedSlice(group, containers, "template", "spec", "containers"); err != nil {
			return nil, err
		}
		groups[i] = group
		return groups, nil
	}
	return nil, fmt.Errorf("RayCluster worker %q not found", name)
}

func keepRayWorkerGroups(groups []any, namesToKeep ...string) ([]any, error) {
	wanted := make(map[string]struct{}, len(namesToKeep))
	for _, name := range namesToKeep {
		wanted[name] = struct{}{}
	}
	kept := make([]any, 0, len(namesToKeep))
	for i := range groups {
		group, ok := groups[i].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("RayCluster worker group %d is not an object", i)
		}
		name, _, _ := unstructured.NestedString(group, "groupName")
		if _, exists := wanted[name]; exists {
			kept = append(kept, group)
			delete(wanted, name)
		}
	}
	if len(wanted) != 0 {
		return nil, fmt.Errorf("not every requested RayCluster worker group exists")
	}
	return kept, nil
}

func assertRayCluster(g gomega.Gomega, object *unstructured.Unstructured, expected map[string]componentE2EExpectation) {
	g.Expect(object.GetAnnotations()).Should(gomega.HaveKeyWithValue(componentRevisionMarkerAnnotation, "ray"))
	g.Expect(expected).Should(gomega.HaveKey("ray-head"))
	assertPodTemplateCPU(g, object.Object, expected["ray-head"].cpu, "spec", "headGroupSpec", "template")

	groups, found, err := unstructured.NestedSlice(object.Object, "spec", "workerGroupSpecs")
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(found).Should(gomega.BeTrue())
	g.Expect(groups).Should(gomega.HaveLen(len(expected) - 1))
	for i := range groups {
		group, ok := groups[i].(map[string]any)
		g.Expect(ok).Should(gomega.BeTrue())
		name, found, err := unstructured.NestedString(group, "groupName")
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(found).Should(gomega.BeTrue())
		component, exists := expected[name]
		g.Expect(exists).Should(gomega.BeTrue(), "unexpected RayCluster worker group %q", name)
		replicas, found, err := unstructured.NestedInt64(group, "replicas")
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(found).Should(gomega.BeTrue())
		g.Expect(replicas).Should(gomega.Equal(int64(component.replicas)))
		assertPodTemplateCPU(g, group, component.cpu, "template")
	}
}

func assertPodTemplateCPU(g gomega.Gomega, object map[string]any, expected string, fields ...string) {
	template, found, err := unstructured.NestedMap(object, fields...)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(found).Should(gomega.BeTrue())
	containers, found, err := unstructured.NestedSlice(template, "spec", "containers")
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(found).Should(gomega.BeTrue())
	g.Expect(containers).ShouldNot(gomega.BeEmpty())
	container, ok := containers[0].(map[string]any)
	g.Expect(ok).Should(gomega.BeTrue())
	actualCPU, found, err := unstructured.NestedString(container, "resources", "requests", "cpu")
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(found).Should(gomega.BeTrue())
	actualQuantity, err := resource.ParseQuantity(actualCPU)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(actualQuantity.Cmp(resource.MustParse(expected))).Should(gomega.Equal(0))
}

var _ = framework.SerialDescribe("[MultiComponentRescheduling] focused workload matrix", func() {
	ginkgo.It("reschedules Volcano Job tasks by component name and preserves complete results", func(ctx context.Context) {
		var crd apiextensionsv1.CustomResourceDefinition
		gomega.Expect(yaml.Unmarshal([]byte(volcanoJobCRDYAML), &crd)).Should(gomega.Succeed())
		installMultiComponentCRD(&crd, "v1alpha1")
		installComponentRevision("batch.volcano.sh/v1alpha1", "Job", volcanoComponentRevision)

		namespace := "karmadatest-volcano-scale-" + rand.String(RandomStrLength)
		name := "volcanojob-" + rand.String(RandomStrLength)
		setupMultiComponentNamespace(namespace)
		workload, gvr := createMultiComponentWorkload(ctx, volcanoJobCRYAML, namespace, name, "jobs")
		updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
			return setVolcanoTaskReplicas(object, map[string]int64{"job-nginx1": 1, "job-nginx2": 2})
		})
		createMultiComponentPolicy(namespace, workload, framework.ClusterNames())

		bindingName := names.GenerateBindingName(workload.GetKind(), name)
		components := func(first, second int32) map[string]componentE2EExpectation {
			return map[string]componentE2EExpectation{
				"job-nginx1": {replicas: first, cpu: "200m"},
				"job-nginx2": {replicas: second, cpu: "100m"},
			}
		}
		assertTasks := func(expected map[string]int64) func(gomega.Gomega, *unstructured.Unstructured) {
			return func(g gomega.Gomega, object *unstructured.Unstructured) {
				assertVolcanoTaskReplicas(g, object, expected)
			}
		}

		ginkgo.By("accept and deliver the initial two-task batch result", func() {
			binding := waitForComponentBinding(ctx, namespace, bindingName, "", components(1, 2), components(1, 2),
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			targetCluster := binding.Spec.Clusters[0].Name
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName,
				assertTasks(map[string]int64{"job-nginx1": 1, "job-nginx2": 2}))
			assertComponentNotDeliveredElsewhere(ctx, gvr, namespace, name, workload.GetKind(), targetCluster)

			ginkgo.By("scale both tasks up from 1/2 to 2/4", func() {
				updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
					return setVolcanoTaskReplicas(object, map[string]int64{"job-nginx1": 2, "job-nginx2": 4})
				})
				waitForComponentBinding(ctx, namespace, bindingName, targetCluster, components(2, 4), components(2, 4),
					metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName,
					assertTasks(map[string]int64{"job-nginx1": 2, "job-nginx2": 4}))
			})

			ginkgo.By("scale both tasks down to zero without losing either component", func() {
				updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
					return setVolcanoTaskReplicas(object, map[string]int64{"job-nginx1": 0, "job-nginx2": 0})
				})
				waitForComponentBinding(ctx, namespace, bindingName, targetCluster, components(0, 0), components(0, 0),
					metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
				waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName,
					assertTasks(map[string]int64{"job-nginx1": 0, "job-nginx2": 0}))
			})
		})
	})

	ginkgo.It("protects RayCluster results across scale, shape, and requirement transitions", func(ctx context.Context) {
		var crd apiextensionsv1.CustomResourceDefinition
		gomega.Expect(yaml.Unmarshal([]byte(rayClusterCRDYAML), &crd)).Should(gomega.Succeed())
		installMultiComponentCRD(&crd, "v1")
		installComponentRevision("ray.io/v1", "RayCluster", rayClusterComponentRevision)

		namespace := "karmadatest-ray-scale-" + rand.String(RandomStrLength)
		name := "raycluster-" + rand.String(RandomStrLength)
		setupMultiComponentNamespace(namespace)
		limitMultiComponentCPU(ctx, namespace, "ray-component-scale", "500m")
		workload, gvr := createMultiComponentWorkload(ctx, rayClusterCRYAML, namespace, name, "rayclusters")

		candidates := framework.ClusterNamesWithSyncMode(clusterv1alpha1.Pull)
		if len(candidates) == 0 {
			candidates = framework.ClusterNames()[:1]
		} else {
			candidates = candidates[:1]
		}
		createMultiComponentPolicy(namespace, workload, candidates)

		bindingName := names.GenerateBindingName(workload.GetKind(), name)
		rayComponents := func(workerA, workerB, workerC *componentE2EExpectation) map[string]componentE2EExpectation {
			components := map[string]componentE2EExpectation{"ray-head": {replicas: 1, cpu: "50m"}}
			if workerA != nil {
				components["worker-a"] = *workerA
			}
			if workerB != nil {
				components["worker-b"] = *workerB
			}
			if workerC != nil {
				components["worker-c"] = *workerC
			}
			return components
		}
		worker := func(replicas int32, cpu string) *componentE2EExpectation {
			return &componentE2EExpectation{replicas: replicas, cpu: cpu}
		}
		assertRay := func(expected map[string]componentE2EExpectation) func(gomega.Gomega, *unstructured.Unstructured) {
			return func(g gomega.Gomega, object *unstructured.Unstructured) { assertRayCluster(g, object, expected) }
		}

		initial := rayComponents(worker(1, "100m"), worker(1, "100m"), nil)
		binding := waitForComponentBinding(ctx, namespace, bindingName, candidates[0], initial, initial,
			metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
		targetCluster := binding.Spec.Clusters[0].Name
		waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(initial))
		assertComponentNotDeliveredElsewhere(ctx, gvr, namespace, name, workload.GetKind(), targetCluster)

		ginkgo.By("reorder worker groups without changing the name-keyed accepted result", func() {
			beforeSource, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			beforeBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(namespace).Get(ctx, bindingName, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, reverseRayWorkerGroups)
			})
			waitForComponentSourceSnapshot(ctx, gvr, namespace, name, bindingName, beforeSource.GetResourceVersion(), beforeBinding.Generation)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, initial, initial,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(initial))
		})

		ginkgo.By("scale worker-a up from 1 to 2", func() {
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 2})
				})
			})
			expected := rayComponents(worker(2, "100m"), worker(1, "100m"), nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, expected, expected,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(expected))
		})

		ginkgo.By("scale worker-a down from 2 to 0", func() {
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 0})
				})
			})
			expected := rayComponents(worker(0, "100m"), worker(1, "100m"), nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, expected, expected,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(expected))
		})

		ginkgo.By("reject a mixed transition and freeze the accepted three-component Work", func() {
			accepted := rayComponents(worker(0, "100m"), worker(1, "100m"), nil)
			revision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(accepted))
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 1, "worker-b": 0})
				})
			})
			desired := rayComponents(worker(1, "100m"), worker(0, "100m"), nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, accepted,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "multi-component transition failed")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, revision, assertRay(accepted))

			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 0, "worker-b": 1})
				})
			})
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, accepted, accepted,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
		})

		ginkgo.By("reject a fourth component, then accept it through explicit full recovery", func() {
			accepted := rayComponents(worker(0, "100m"), worker(1, "100m"), nil)
			revision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(accepted))
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return append(groups, rayWorkerGroup("worker-c", 1, "100m")), nil
				})
			})
			desired := rayComponents(worker(0, "100m"), worker(1, "100m"), worker(1, "100m"))
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, accepted,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "multi-component transition failed")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, revision, assertRay(accepted))

			triggerExplicitComponentRecovery(ctx, namespace, bindingName)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, desired,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(desired))
		})

		ginkgo.By("reject removal from four to two components, then recover explicitly", func() {
			accepted := rayComponents(worker(0, "100m"), worker(1, "100m"), worker(1, "100m"))
			revision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(accepted))
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return keepRayWorkerGroups(groups, "worker-a")
				})
			})
			desired := rayComponents(worker(0, "100m"), nil, nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, accepted,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "multi-component transition failed")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, revision, assertRay(accepted))

			triggerExplicitComponentRecovery(ctx, namespace, bindingName)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, desired,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(desired))
		})

		ginkgo.By("accept a two-component scale-up from 0 to 2", func() {
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 2})
				})
			})
			expected := rayComponents(worker(2, "100m"), nil, nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, expected, expected,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(expected))
		})

		ginkgo.By("reject a worker CPU requirement change and retain the accepted template", func() {
			accepted := rayComponents(worker(2, "100m"), nil, nil)
			revision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(accepted))
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerCPU(groups, "worker-a", "200m")
				})
			})
			desired := rayComponents(worker(2, "200m"), nil, nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, accepted,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "multi-component transition failed")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, revision, assertRay(accepted))

			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerCPU(groups, "worker-a", "100m")
				})
			})
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, accepted, accepted,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
		})

		ginkgo.By("reject a 2-to-8 scale-up whose 600m delta exceeds the 500m estimator quota", func() {
			accepted := rayComponents(worker(2, "100m"), nil, nil)
			revision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(accepted))
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 8})
				})
			})
			desired := rayComponents(worker(8, "100m"), nil, nil)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, accepted,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable,
				"the current target cluster has insufficient resource for component scale")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, revision, assertRay(accepted))

			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return setRayWorkerReplicas(groups, map[string]int64{"worker-a": 2})
				})
			})
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, accepted, accepted,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
		})

		ginkgo.By("reject a component rename, then accept the new name through explicit full recovery", func() {
			accepted := rayComponents(worker(2, "100m"), nil, nil)
			revision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(accepted))
			updateMultiComponentWorkload(ctx, gvr, namespace, name, func(object *unstructured.Unstructured) error {
				return updateRayWorkerGroups(object, func(groups []any) ([]any, error) {
					return renameRayWorkerGroup(groups, "worker-a", "worker-renamed")
				})
			})
			desired := map[string]componentE2EExpectation{
				"ray-head":       {replicas: 1, cpu: "50m"},
				"worker-renamed": {replicas: 2, cpu: "100m"},
			}
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, accepted,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "component name changes")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, revision, assertRay(accepted))

			triggerExplicitComponentRecovery(ctx, namespace, bindingName)
			waitForComponentBinding(ctx, namespace, bindingName, targetCluster, desired, desired,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), targetCluster, bindingName, assertRay(desired))
		})
	})

	ginkgo.It("recovers a complete RayCluster result after the accepted target becomes ineligible", func(ctx context.Context) {
		var crd apiextensionsv1.CustomResourceDefinition
		gomega.Expect(yaml.Unmarshal([]byte(rayClusterCRDYAML), &crd)).Should(gomega.Succeed())
		installMultiComponentCRD(&crd, "v1")
		installComponentRevision("ray.io/v1", "RayCluster", rayClusterComponentRevision)

		namespace := "karmadatest-ray-failover-" + rand.String(RandomStrLength)
		name := "raycluster-" + rand.String(RandomStrLength)
		setupMultiComponentNamespace(namespace)
		const quotaName = "ray-component-failover"
		limitMultiComponentCPU(ctx, namespace, quotaName, "500m")
		workload, gvr := createMultiComponentWorkload(ctx, rayClusterCRYAML, namespace, name, "rayclusters")

		allClusters := framework.ClusterNames()
		gomega.Expect(len(allClusters)).Should(gomega.BeNumerically(">=", 2))
		candidates := allClusters[:2]
		failoverLabelKey := "e2e.karmada.io/multi-component-failover-" + rand.String(RandomStrLength)
		failoverLabelValue := "eligible"
		for _, candidate := range candidates {
			candidateName := candidate
			ginkgo.DeferCleanup(func() {
				setMultiComponentClusterLabel(context.Background(), candidateName, failoverLabelKey, failoverLabelValue, false)
			})
			setMultiComponentClusterLabel(ctx, candidateName, failoverLabelKey, failoverLabelValue, true)
		}
		createMultiComponentLabelPolicy(namespace, workload, failoverLabelKey, failoverLabelValue, policyv1alpha1.ReplicaSchedulingTypeDuplicated)

		bindingName := names.GenerateBindingName(workload.GetKind(), name)
		expected := map[string]componentE2EExpectation{
			"ray-head": {replicas: 1, cpu: "50m"},
			"worker-a": {replicas: 1, cpu: "100m"},
			"worker-b": {replicas: 1, cpu: "100m"},
		}
		assertRay := func(g gomega.Gomega, object *unstructured.Unstructured) { assertRayCluster(g, object, expected) }

		binding := waitForComponentBinding(ctx, namespace, bindingName, "", expected, expected,
			metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
		originalTarget := binding.Spec.Clusters[0].Name
		alternativeTarget := candidates[0]
		if alternativeTarget == originalTarget {
			alternativeTarget = candidates[1]
		}
		originalRevision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), originalTarget, bindingName, assertRay)
		assertComponentNotDeliveredElsewhere(ctx, gvr, namespace, name, workload.GetKind(), originalTarget)

		setMultiComponentClusterLabel(ctx, originalTarget, failoverLabelKey, failoverLabelValue, false)

		ginkgo.By("explicitly recover the complete result on the eligible alternative", func() {
			// Ordinary Duplicated reconciliation does not promise migration for a
			// taint-only Cluster update. An explicit recovery is the supported
			// full-scheduling escape hatch when the accepted target is still present.
			triggerExplicitComponentRecovery(ctx, namespace, bindingName)
			waitForComponentBinding(ctx, namespace, bindingName, alternativeTarget, expected, expected,
				metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
			migrated := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), alternativeTarget, bindingName, assertRay)
			gomega.Expect(migrated.acceptedRequirementsHash).Should(gomega.Equal(originalRevision.acceptedRequirementsHash))
			waitForComponentDeliveryRemoved(ctx, gvr, namespace, name, workload.GetKind(), originalTarget)
			assertComponentNotDeliveredElsewhere(ctx, gvr, namespace, name, workload.GetKind(), alternativeTarget)
		})

		setMultiComponentClusterLabel(ctx, originalTarget, failoverLabelKey, failoverLabelValue, true)
		setMultiComponentCPUQuota(ctx, originalTarget, namespace, quotaName, "200m")

		setMultiComponentClusterLabel(ctx, alternativeTarget, failoverLabelKey, failoverLabelValue, false)
		acceptedRevision := waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), alternativeTarget, bindingName, assertRay)

		ginkgo.By("retain the accepted result when explicit recovery has no fitting alternative", func() {
			triggerExplicitComponentRecovery(ctx, namespace, bindingName)
			waitForComponentBinding(ctx, namespace, bindingName, alternativeTarget, expected, expected,
				metav1.ConditionFalse, workv1alpha2.BindingReasonUnschedulable, "zero component sets")
			assertComponentDeliveryFrozen(ctx, gvr, namespace, name, workload.GetKind(), alternativeTarget, bindingName, acceptedRevision, assertRay)
			assertComponentNotDeliveredElsewhere(ctx, gvr, namespace, name, workload.GetKind(), alternativeTarget)
		})

		setMultiComponentClusterLabel(ctx, alternativeTarget, failoverLabelKey, failoverLabelValue, true)
		setMultiComponentCPUQuota(ctx, originalTarget, namespace, quotaName, "500m")
		triggerExplicitComponentRecovery(ctx, namespace, bindingName)
		recovered := waitForComponentBinding(ctx, namespace, bindingName, "", expected, expected,
			metav1.ConditionTrue, workv1alpha2.BindingReasonSuccess, "")
		waitForComponentDelivery(ctx, gvr, namespace, name, workload.GetKind(), recovered.Spec.Clusters[0].Name, bindingName, assertRay)
	})
})
