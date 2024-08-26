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
    {{- include "karmada.commonLabels" . | nindent 4 }}
spec:
  {{- include "karmada.apiserver.caBundle" . | nindent 2 }}
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
  labels:
    app: {{ $name }}-aggregated-apiserver
    {{- include "karmada.commonLabels" . | nindent 4 }}
spec:
  type: ExternalName
  externalName: {{ $name }}-aggregated-apiserver.{{ include "karmada.namespace" . }}.svc.{{ .Values.clusterDomain }}
{{- end }}
{{- if has "metricsAdapter" .Values.components }}
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
  labels:
    app: {{ $name }}-metrics-adapter
    apiserver: "true"
spec:
  {{- include "karmada.apiserver.caBundle" . | nindent 2 }}
  group: metrics.k8s.io
  groupPriorityMinimum: 100
  service:
    name: {{ $name }}-metrics-adapter
    namespace: {{ $systemNamespace }}
  version: v1beta1
  versionPriority: 200
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta2.custom.metrics.k8s.io
  labels:
    app: {{ $name }}-metrics-adapter
    apiserver: "true"
spec:
  {{- include "karmada.apiserver.caBundle" . | nindent 2 }}
  group: custom.metrics.k8s.io
  groupPriorityMinimum: 100
  service:
    name: {{ $name }}-metrics-adapter
    namespace: {{ $systemNamespace }}
  version: v1beta2
  versionPriority: 200
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.custom.metrics.k8s.io
  labels:
    app: {{ $name }}-metrics-adapter
    apiserver: "true"
spec:
  {{- include "karmada.apiserver.caBundle" . | nindent 2 }}
  group: custom.metrics.k8s.io
  groupPriorityMinimum: 100
  service:
    name: {{ $name }}-metrics-adapter
    namespace: {{ $systemNamespace }}
  version: v1beta1
  versionPriority: 200
---
apiVersion: v1
kind: Service
metadata:
  name: {{ $name }}-metrics-adapter
  namespace: {{ $systemNamespace }}
spec:
  type: ExternalName
  externalName: {{ $name }}-metrics-adapter.{{ include "karmada.namespace" . }}.svc.{{ .Values.clusterDomain }}
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
    {{- include "karmada.commonLabels" . | nindent 4 }}
spec:
  {{- include "karmada.apiserver.caBundle" . | nindent 2 }}
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
  labels:
    app: {{ $name }}-search
    {{- include "karmada.commonLabels" . | nindent 4 }}
spec:
  type: ExternalName
  externalName: {{ $name }}-search.{{ include "karmada.namespace" . }}.svc.{{ .Values.clusterDomain }}
{{- end }}
{{- end -}}
