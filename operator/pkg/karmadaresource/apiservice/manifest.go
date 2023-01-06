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
  insecureSkipTLSVerify: true
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
  externalName: {{ .ServiceName }}.{{ .Namespace }}.svc
`
)
