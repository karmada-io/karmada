package lib

import (
	"encoding/json"
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/common/restfuladapter"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// ResourceWithNamespaceScoped contain gvr and NamespaceScoped of a resource.
type ResourceWithNamespaceScoped struct {
	GVR             schema.GroupVersionResource
	NamespaceScoped bool
}

// Config is used to configure swagger information.
type Config struct {
	Scheme             *runtime.Scheme
	Codecs             serializer.CodecFactory
	Info               spec.InfoProps
	OpenAPIDefinitions []common.GetOpenAPIDefinitions
	Resources          []ResourceWithNamespaceScoped
	Mapper             *meta.DefaultRESTMapper
}

// GetOpenAPIDefinitions get openapi definitions.
func (c *Config) GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	out := map[string]common.OpenAPIDefinition{}
	for _, def := range c.OpenAPIDefinitions {
		for k, v := range def(ref) {
			out[k] = v
		}
	}
	return out
}

// RenderOpenAPISpec create openapi spec of swagger.
func RenderOpenAPISpec(cfg Config) (string, error) {
	options := genericoptions.NewRecommendedOptions("/registry/karmada.io", cfg.Codecs.LegacyCodec())
	options.SecureServing.ServerCert.CertDirectory = "/tmp/karmada-swagger"
	options.SecureServing.BindPort = 6445
	options.Etcd = nil
	options.Authentication = nil
	options.Authorization = nil
	options.CoreAPI = nil
	options.Admission = nil

	if err := options.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		klog.Fatal(fmt.Errorf("error creating self-signed certificates: %v", err))
	}

	serverConfig := genericapiserver.NewRecommendedConfig(cfg.Codecs)
	if err := options.ApplyTo(serverConfig); err != nil {
		klog.Fatal(err)
		return "", err
	}
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(cfg.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(cfg.Scheme))
	serverConfig.OpenAPIConfig.Info.InfoProps = cfg.Info

	genericServer, err := serverConfig.Complete().New("karmada-openapi-server", genericapiserver.NewEmptyDelegate())
	if err != nil {
		klog.Fatal(err)
		return "", err
	}

	table, err := createRouterTable(&cfg)
	if err != nil {
		klog.Fatal(err)
		return "", err
	}

	for g, resmap := range table {
		apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(g, cfg.Scheme, metav1.ParameterCodec, cfg.Codecs)
		storage := map[string]map[string]rest.Storage{}
		for r, info := range resmap {
			if storage[info.gvk.Version] == nil {
				storage[info.gvk.Version] = map[string]rest.Storage{}
			}
			storage[info.gvk.Version][r.Resource] = &StandardREST{info}
			// Add status router for all resources.
			storage[info.gvk.Version][r.Resource+"/status"] = &StatusREST{StatusInfo{
				gvk: info.gvk,
				obj: info.obj,
			}}

			// To define additional endpoints for CRD resources, we need to
			// implement our own REST interface and add it to our custom path.
			if r.Resource == "clusters" {
				storage[info.gvk.Version][r.Resource+"/proxy"] = &ProxyREST{}
			}
		}

		for version, s := range storage {
			apiGroupInfo.VersionedResourcesStorageMap[version] = s
		}

		// Install api to apiserver.
		if err := genericServer.InstallAPIGroup(&apiGroupInfo); err != nil {
			klog.Fatal(err)
			return "", err
		}
	}

	// Create Swagger Spec.
	spec, err := builder.BuildOpenAPISpecFromRoutes(restfuladapter.AdaptWebServices(genericServer.Handler.GoRestfulContainer.RegisteredWebServices()), serverConfig.OpenAPIConfig)
	if err != nil {
		klog.Fatal(err)
		return "", err
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		klog.Fatal(err)
		return "", err
	}
	return string(data), nil
}

// createRouterTable create router map for every resource.
func createRouterTable(cfg *Config) (map[string]map[schema.GroupVersionResource]ResourceInfo, error) {
	table := map[string]map[schema.GroupVersionResource]ResourceInfo{}
	// Create router map for every resource
	for _, ti := range cfg.Resources {
		var resmap map[schema.GroupVersionResource]ResourceInfo
		if m, found := table[ti.GVR.Group]; found {
			resmap = m
		} else {
			resmap = map[schema.GroupVersionResource]ResourceInfo{}
			table[ti.GVR.Group] = resmap
		}

		gvk, err := cfg.Mapper.KindFor(ti.GVR)
		if err != nil {
			klog.Fatal(err)
			return nil, err
		}
		obj, err := cfg.Scheme.New(gvk)

		if err != nil {
			klog.Fatal(err)
			return nil, err
		}

		list, err := cfg.Scheme.New(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		if err != nil {
			klog.Fatal(err)
			return nil, err
		}

		resmap[ti.GVR] = ResourceInfo{
			gvk:             gvk,
			obj:             obj,
			list:            list,
			namespaceScoped: ti.NamespaceScoped,
		}
	}

	return table, nil
}
