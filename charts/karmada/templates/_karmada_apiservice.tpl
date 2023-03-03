{{- define "karmada.apiservice" -}}
{{- $name := include "karmada.name" . -}}
{{- $systemNamespace := .Values.systemNamespace -}}
{{- if eq .Values.installMode "host" }}
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1alpha1.cluster.karmada.io
  labels:
    app: {{ $name }}-aggregated-apiserver
    apiserver: "true"
spec:
  insecureSkipTLSVerify: true
  group: cluster.karmada.io
  groupPriorityMinimum: 2000
  service:
    name: {{ $name }}-aggregated-apiserver
    namespace: {{ $systemNamespace }}
  version: v1alpha1
  versionPriority: 10
---
apiVersion: v1
kind: Service
metadata:
  name: {{ $name }}-aggregated-apiserver
  namespace: {{ $systemNamespace }}
spec:
  type: ExternalName
  externalName: {{ $name }}-aggregated-apiserver.{{ include "karmada.namespace" . }}.svc.{{ .Values.clusterDomain }}
{{- end }}
{{- if has "search" .Values.components }}
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1alpha1.search.karmada.io
  labels:
    app: {{ $name }}-search
    apiserver: "true"
spec:
  insecureSkipTLSVerify: true
  group: search.karmada.io
  groupPriorityMinimum: 2000
  service:
    name: {{ $name }}-search
    namespace: {{ $systemNamespace }}
  version: v1alpha1
  versionPriority: 10
---
apiVersion: v1
kind: Service
metadata:
  name: {{ $name }}-search
  namespace: {{ $systemNamespace }}
spec:
  type: ExternalName
  externalName: {{ $name }}-search.{{ include "karmada.namespace" . }}.svc.{{ .Values.clusterDomain }}
{{- end }}
{{- end -}}
