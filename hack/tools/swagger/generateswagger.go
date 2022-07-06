package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"github.com/karmada-io/karmada/hack/tools/swagger/lib"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	generatedopenapi "github.com/karmada-io/karmada/pkg/generated/openapi"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func main() {
	Scheme := gclient.NewSchema()

	mapper := meta.NewDefaultRESTMapper(nil)

	mapper.AddSpecific(clusterv1alpha1.SchemeGroupVersion.WithKind(clusterv1alpha1.ResourceKindCluster),
		clusterv1alpha1.SchemeGroupVersion.WithResource(clusterv1alpha1.ResourcePluralCluster),
		clusterv1alpha1.SchemeGroupVersion.WithResource(clusterv1alpha1.ResourceSingularCluster), meta.RESTScopeRoot)

	mapper.AddSpecific(configv1alpha1.SchemeGroupVersion.WithKind(configv1alpha1.ResourceKindResourceInterpreterWebhookConfiguration),
		configv1alpha1.SchemeGroupVersion.WithResource(configv1alpha1.ResourcePluralResourceInterpreterWebhookConfiguration),
		configv1alpha1.SchemeGroupVersion.WithResource(configv1alpha1.ResourceSingularResourceInterpreterWebhookConfiguration), meta.RESTScopeRoot)

	mapper.AddSpecific(networkingv1alpha1.SchemeGroupVersion.WithKind(networkingv1alpha1.ResourceKindMultiClusterIngress),
		networkingv1alpha1.SchemeGroupVersion.WithResource(networkingv1alpha1.ResourcePluralMultiClusterIngress),
		networkingv1alpha1.SchemeGroupVersion.WithResource(networkingv1alpha1.ResourceSingularMultiClusterIngress), meta.RESTScopeRoot)

	mapper.AddSpecific(policyv1alpha1.SchemeGroupVersion.WithKind(policyv1alpha1.ResourceKindPropagationPolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralPropagationPolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourceSingularPropagationPolicy), meta.RESTScopeRoot)

	mapper.AddSpecific(policyv1alpha1.SchemeGroupVersion.WithKind(policyv1alpha1.ResourceKindClusterPropagationPolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralClusterPropagationPolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourceSingularClusterPropagationPolicy), meta.RESTScopeRoot)

	mapper.AddSpecific(policyv1alpha1.SchemeGroupVersion.WithKind(policyv1alpha1.ResourceKindOverridePolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralOverridePolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourceSingularOverridePolicy), meta.RESTScopeRoot)

	mapper.AddSpecific(policyv1alpha1.SchemeGroupVersion.WithKind(policyv1alpha1.ResourceKindClusterOverridePolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralClusterOverridePolicy),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourceSingularClusterOverridePolicy), meta.RESTScopeRoot)

	mapper.AddSpecific(policyv1alpha1.SchemeGroupVersion.WithKind(policyv1alpha1.ResourceKindFederatedResourceQuota),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralFederatedResourceQuota),
		policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourceSingularFederatedResourceQuota), meta.RESTScopeRoot)

	mapper.AddSpecific(workv1alpha1.SchemeGroupVersion.WithKind(workv1alpha1.ResourceKindWork),
		workv1alpha1.SchemeGroupVersion.WithResource(workv1alpha1.ResourcePluralWork),
		workv1alpha1.SchemeGroupVersion.WithResource(workv1alpha1.ResourceSingularWork), meta.RESTScopeRoot)

	mapper.AddSpecific(workv1alpha2.SchemeGroupVersion.WithKind(workv1alpha2.ResourceKindResourceBinding),
		workv1alpha2.SchemeGroupVersion.WithResource(workv1alpha2.ResourcePluralResourceBinding),
		workv1alpha2.SchemeGroupVersion.WithResource(workv1alpha2.ResourceSingularResourceBinding), meta.RESTScopeRoot)

	mapper.AddSpecific(workv1alpha2.SchemeGroupVersion.WithKind(workv1alpha2.ResourceKindClusterResourceBinding),
		workv1alpha2.SchemeGroupVersion.WithResource(workv1alpha2.ResourcePluralClusterResourceBinding),
		workv1alpha2.SchemeGroupVersion.WithResource(workv1alpha2.ResourceSingularClusterResourceBinding), meta.RESTScopeRoot)

	mapper.AddSpecific(searchv1alpha1.SchemeGroupVersion.WithKind(searchv1alpha1.ResourceKindResourceRegistry),
		searchv1alpha1.SchemeGroupVersion.WithResource(searchv1alpha1.ResourcePluralResourceRegistry),
		searchv1alpha1.SchemeGroupVersion.WithResource(searchv1alpha1.ResourceSingularResourceRegistry), meta.RESTScopeRoot)

	spec, err := lib.RenderOpenAPISpec(lib.Config{
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
			{GVR: clusterv1alpha1.SchemeGroupVersion.WithResource(clusterv1alpha1.ResourcePluralCluster), NamespaceScoped: clusterv1alpha1.ResourceNamespaceScopedCluster},
			{GVR: configv1alpha1.SchemeGroupVersion.WithResource(configv1alpha1.ResourcePluralResourceInterpreterWebhookConfiguration), NamespaceScoped: configv1alpha1.ResourceNamespaceScopedResourceInterpreterWebhookConfiguration},
			{GVR: networkingv1alpha1.SchemeGroupVersion.WithResource(networkingv1alpha1.ResourcePluralMultiClusterIngress), NamespaceScoped: networkingv1alpha1.ResourceNamespaceScopedMultiClusterIngress},
			{GVR: policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralPropagationPolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedPropagationPolicy},
			{GVR: policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralClusterPropagationPolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedClusterPropagationPolicy},
			{GVR: policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralOverridePolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedOverridePolicy},
			{GVR: policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralClusterOverridePolicy), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedClusterOverridePolicy},
			{GVR: policyv1alpha1.SchemeGroupVersion.WithResource(policyv1alpha1.ResourcePluralFederatedResourceQuota), NamespaceScoped: policyv1alpha1.ResourceNamespaceScopedFederatedResourceQuota},
			{GVR: workv1alpha1.SchemeGroupVersion.WithResource(workv1alpha1.ResourcePluralWork), NamespaceScoped: workv1alpha1.ResourceNamespaceScopedWork},
			{GVR: workv1alpha2.SchemeGroupVersion.WithResource(workv1alpha2.ResourcePluralResourceBinding), NamespaceScoped: workv1alpha2.ResourceNamespaceScopedResourceBinding},
			{GVR: workv1alpha2.SchemeGroupVersion.WithResource(workv1alpha2.ResourcePluralClusterResourceBinding), NamespaceScoped: workv1alpha2.ResourceNamespaceScopedClusterResourceBinding},
			{GVR: searchv1alpha1.SchemeGroupVersion.WithResource(searchv1alpha1.ResourcePluralResourceRegistry), NamespaceScoped: searchv1alpha1.ResourceNamespaceScopedResourceRegistry},
		},
		Mapper: mapper,
	})
	if err != nil {
		klog.Fatal(err.Error())
	}
	fmt.Println(spec)
}
