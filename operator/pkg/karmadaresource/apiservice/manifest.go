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

package apiservice

const (
	// KarmadaAggregatedAPIService is karmada aggregated apiserver APIService manifest
	KarmadaAggregatedAPIService = `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  labels:
    apiserver: "true"
    app: karmada-aggregated-apiserver
  name: v1alpha1.cluster.karmada.io
spec:
  group: cluster.karmada.io
  groupPriorityMinimum: 2000
  caBundle: {{ .CABundle }}
  service:
    name: {{ .ServiceName }}
    namespace: {{ .Namespace }}
  version: v1alpha1
  versionPriority: 10
`

	// KarmadaAggregatedApiserverService is karmada aggregated apiserver service manifest
	KarmadaAggregatedApiserverService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  type: ExternalName
  externalName: {{ .HostClusterServiceName }}.{{ .HostClusterNamespace }}.svc
`

	// KarmadaMetricsAdapterAPIService is karmada-metrics-adapter APIService manifest
	KarmadaMetricsAdapterAPIService = `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: {{ .Name }}
spec:
  service:
    name: {{ .ServiceName }}
    namespace: {{ .Namespace }}
  group: {{ .Group }}
  version: {{ .Version }}
  caBundle: {{ .CABundle }}
  groupPriorityMinimum: 100
  versionPriority: 200
`

	// KarmadaMetricsAdapterService is karmada-metrics-adapter service manifest
	KarmadaMetricsAdapterService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  type: ExternalName
  externalName: {{ .HostClusterServiceName }}.{{ .HostClusterNamespace }}.svc
`

	// KarmadaSearchAPIService is karmada-search APIService manifest
	KarmadaSearchAPIService = `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1alpha1.search.karmada.io
  labels:
    app: karmada-search
    apiserver: "true"
spec:
  caBundle: {{ .CABundle }}
  group: search.karmada.io
  groupPriorityMinimum: 2000
  service:
    name: {{ .ServiceName }}
    namespace: {{ .Namespace }}
  version: v1alpha1
  versionPriority: 10
`

	// KarmadaSearchService is karmada-search service manifest
	KarmadaSearchService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  type: ExternalName
  externalName: {{ .HostClusterServiceName }}.{{ .HostClusterNamespace }}.svc
`
)
