/*
Copyright 2022 The Karmada Authors.

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

package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/karmada-io/karmada/hack/tools/swagger/lib"
	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func main() {
	Scheme := gclient.NewSchema()

	mapper := meta.NewDefaultRESTMapper(nil)

	mapper.AddSpecific(schema.GroupVersion{Group: clusterv1alpha1.GroupVersion.Group, Version: clusterv1alpha1.GroupVersion.Version}.WithKind(clusterv1alpha1.ResourceKindCluster),
		schema.GroupVersion{Group: clusterv1alpha1.GroupVersion.Group, Version: clusterv1alpha1.GroupVersion.Version}.WithResource(clusterv1alpha1.ResourcePluralCluster),
		schema.GroupVersion{Group: clusterv1alpha1.GroupVersion.Group, Version: clusterv1alpha1.GroupVersion.Version}.WithResource(clusterv1alpha1.ResourceSingularCluster),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithKind(configv1alpha1.ResourceKindResourceInterpreterWebhookConfiguration),
		schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithResource(configv1alpha1.ResourcePluralResourceInterpreterWebhookConfiguration),
		schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithResource(configv1alpha1.ResourceSingularResourceInterpreterWebhookConfiguration),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithKind(configv1alpha1.ResourceKindResourceInterpreterCustomization),
		schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithResource(configv1alpha1.ResourcePluralResourceInterpreterCustomization),
		schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithResource(configv1alpha1.ResourceSingularResourceInterpreterCustomization),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithKind(networkingv1alpha1.ResourceKindMultiClusterIngress),
		schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithResource(networkingv1alpha1.ResourcePluralMultiClusterIngress),
		schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithResource(networkingv1alpha1.ResourceSingularMultiClusterIngress),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithKind(networkingv1alpha1.ResourceKindMultiClusterService),
		schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithResource(networkingv1alpha1.ResourcePluralMultiClusterService),
		schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithResource(networkingv1alpha1.ResourceSingularMultiClusterService), meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithKind(policyv1alpha1.ResourceKindPropagationPolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralPropagationPolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourceSingularPropagationPolicy),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithKind(policyv1alpha1.ResourceKindClusterPropagationPolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralClusterPropagationPolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourceSingularClusterPropagationPolicy), meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithKind(policyv1alpha1.ResourceKindOverridePolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralOverridePolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourceSingularOverridePolicy),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithKind(policyv1alpha1.ResourceKindClusterOverridePolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralClusterOverridePolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourceSingularClusterOverridePolicy),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithKind(policyv1alpha1.ResourceKindFederatedResourceQuota),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralFederatedResourceQuota),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourceSingularFederatedResourceQuota),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithKind(policyv1alpha1.ResourceKindClusterTaintPolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralClusterTaintPolicy),
		schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourceSingularClusterTaintPolicy),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: workv1alpha1.GroupVersion.Group, Version: workv1alpha1.GroupVersion.Version}.WithKind(workv1alpha1.ResourceKindWork),
		schema.GroupVersion{Group: workv1alpha1.GroupVersion.Group, Version: workv1alpha1.GroupVersion.Version}.WithResource(workv1alpha1.ResourcePluralWork),
		schema.GroupVersion{Group: workv1alpha1.GroupVersion.Group, Version: workv1alpha1.GroupVersion.Version}.WithResource(workv1alpha1.ResourceSingularWork),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithKind(workv1alpha2.ResourceKindResourceBinding),
		schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithResource(workv1alpha2.ResourcePluralResourceBinding),
		schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithResource(workv1alpha2.ResourceSingularResourceBinding),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithKind(workv1alpha2.ResourceKindClusterResourceBinding),
		schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithResource(workv1alpha2.ResourcePluralClusterResourceBinding),
		schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithResource(workv1alpha2.ResourceSingularClusterResourceBinding),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version}.WithKind(searchv1alpha1.ResourceKindResourceRegistry),
		schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version}.WithResource(searchv1alpha1.ResourcePluralResourceRegistry),
		schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version}.WithResource(searchv1alpha1.ResourceSingularResourceRegistry),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithKind(autoscalingv1alpha1.FederatedHPAKind),
		schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithResource(autoscalingv1alpha1.ResourcePluralFederatedHPA),
		schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithResource(autoscalingv1alpha1.ResourceSingularFederatedHPA),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithKind(autoscalingv1alpha1.ResourceKindCronFederatedHPA),
		schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithResource(autoscalingv1alpha1.ResourcePluralCronFederatedHPA),
		schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithResource(autoscalingv1alpha1.ResourceSingularCronFederatedHPA),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: remedyv1alpha1.GroupVersion.Group, Version: remedyv1alpha1.GroupVersion.Version}.WithKind(remedyv1alpha1.ResourceKindRemedy),
		schema.GroupVersion{Group: remedyv1alpha1.GroupVersion.Group, Version: remedyv1alpha1.GroupVersion.Version}.WithResource(remedyv1alpha1.ResourcePluralRemedy),
		schema.GroupVersion{Group: remedyv1alpha1.GroupVersion.Group, Version: remedyv1alpha1.GroupVersion.Version}.WithResource(remedyv1alpha1.ResourceSingularRemedy),
		meta.RESTScopeRoot)

	mapper.AddSpecific(schema.GroupVersion{Group: appsv1alpha1.GroupVersion.Group, Version: appsv1alpha1.GroupVersion.Version}.WithKind(appsv1alpha1.ResourceKindWorkloadRebalancer),
		schema.GroupVersion{Group: appsv1alpha1.GroupVersion.Group, Version: appsv1alpha1.GroupVersion.Version}.WithResource(appsv1alpha1.ResourcePluralWorkloadRebalancer),
		schema.GroupVersion{Group: appsv1alpha1.GroupVersion.Group, Version: appsv1alpha1.GroupVersion.Version}.WithResource(appsv1alpha1.ResourceSingularWorkloadRebalancer),
		meta.RESTScopeRoot)

	openAPISpec, err := lib.RenderOpenAPISpec(lib.Config{
		Info: spec.InfoProps{
			Title:       "Karmada OpenAPI",
			Version:     "unversioned",
			Description: "Karmada is Open, Multi-Cloud, Multi-Cluster Kubernetes Orchestration System. For more information, please see https://github.com/karmada-io/karmada.",
			License: &spec.License{
				Name: "Apache 2.0",
				URL:  "https://www.apache.org/licenses/LICENSE-2.0.html",
			},
		},
		Scheme: Scheme,
		Codecs: serializer.NewCodecFactory(Scheme),
		OpenAPIDefinitions: []common.GetOpenAPIDefinitions{
			generatedopenapi.GetOpenAPIDefinitions,
		},
		Resources: []lib.ResourceWithNamespaceScoped{
			{GVR: schema.GroupVersion{Group: clusterv1alpha1.GroupVersion.Group, Version: clusterv1alpha1.GroupVersion.Version}.WithResource(clusterv1alpha1.ResourcePluralCluster), NamespaceScoped: clusterv1alpha1.ResourceNamespaceScopedCluster},
			{GVR: schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithResource(configv1alpha1.ResourcePluralResourceInterpreterWebhookConfiguration), NamespaceScoped: configv1alpha1.ResourceNamespaceScopedResourceInterpreterWebhookConfiguration},
			{GVR: schema.GroupVersion{Group: configv1alpha1.GroupVersion.Group, Version: configv1alpha1.GroupVersion.Version}.WithResource(configv1alpha1.ResourcePluralResourceInterpreterCustomization), NamespaceScoped: configv1alpha1.ResourceNamespaceScopedResourceInterpreterCustomization},
			{GVR: schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithResource(networkingv1alpha1.ResourcePluralMultiClusterIngress), NamespaceScoped: networkingv1alpha1.ResourceNamespaceScopedMultiClusterIngress},
			{GVR: schema.GroupVersion{Group: networkingv1alpha1.GroupVersion.Group, Version: networkingv1alpha1.GroupVersion.Version}.WithResource(networkingv1alpha1.ResourcePluralMultiClusterService), NamespaceScoped: networkingv1alpha1.ResourceNamespaceScopedMultiClusterService},
			{GVR: schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralPropagationPolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedPropagationPolicy},
			{GVR: schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralClusterPropagationPolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedClusterPropagationPolicy},
			{GVR: schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralOverridePolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedOverridePolicy},
			{GVR: schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralClusterOverridePolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedClusterOverridePolicy},
			{GVR: schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralFederatedResourceQuota), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedFederatedResourceQuota},
			{GVR: schema.GroupVersion{Group: policyv1alpha1.GroupVersion.Group, Version: policyv1alpha1.GroupVersion.Version}.WithResource(policyv1alpha1.ResourcePluralClusterTaintPolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedClusterTaintPolicy},
			{GVR: schema.GroupVersion{Group: workv1alpha1.GroupVersion.Group, Version: workv1alpha1.GroupVersion.Version}.WithResource(workv1alpha1.ResourcePluralWork), NamespaceScoped: workv1alpha1.ResourceNamespaceScopedWork},
			{GVR: schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithResource(workv1alpha2.ResourcePluralResourceBinding), NamespaceScoped: workv1alpha2.ResourceNamespaceScopedResourceBinding},
			{GVR: schema.GroupVersion{Group: workv1alpha2.GroupVersion.Group, Version: workv1alpha2.GroupVersion.Version}.WithResource(workv1alpha2.ResourcePluralClusterResourceBinding), NamespaceScoped: workv1alpha2.ResourceNamespaceScopedClusterResourceBinding},
			{GVR: schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version}.WithResource(searchv1alpha1.ResourcePluralResourceRegistry), NamespaceScoped: searchv1alpha1.ResourceNamespaceScopedResourceRegistry},
			{GVR: schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithResource(autoscalingv1alpha1.ResourcePluralFederatedHPA), NamespaceScoped: autoscalingv1alpha1.ResourceNamespaceScopedFederatedHPA},
			{GVR: schema.GroupVersion{Group: autoscalingv1alpha1.GroupVersion.Group, Version: autoscalingv1alpha1.GroupVersion.Version}.WithResource(autoscalingv1alpha1.ResourcePluralCronFederatedHPA), NamespaceScoped: autoscalingv1alpha1.ResourceNamespaceScopedCronFederatedHPA},
			{GVR: schema.GroupVersion{Group: remedyv1alpha1.GroupVersion.Group, Version: remedyv1alpha1.GroupVersion.Version}.WithResource(remedyv1alpha1.ResourcePluralRemedy), NamespaceScoped: remedyv1alpha1.ResourceNamespaceScopedRemedy},
			{GVR: schema.GroupVersion{Group: appsv1alpha1.GroupVersion.Group, Version: appsv1alpha1.GroupVersion.Version}.WithResource(appsv1alpha1.ResourcePluralWorkloadRebalancer), NamespaceScoped: appsv1alpha1.ResourceNamespaceScopedWorkloadRebalancer},
		},
		Mapper: mapper,
	})
	if err != nil {
		klog.Fatal(err.Error())
	}
	fmt.Println(openAPISpec)
}
