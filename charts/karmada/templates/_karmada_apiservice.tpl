{{- define "karmada.apiservice" -}}
{{- $name := include "karmada.name" . -}}
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
    namespace: {{ include "karmada.namespace" . }}
  version: v1alpha1
  versionPriority: 10
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
    namespace: {{ include "karmada.namespace" . }}
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
    namespace: {{ include "karmada.namespace" . }}
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
    namespace: {{ include "karmada.namespace" . }}
  version: v1beta1
  versionPriority: 200
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
    namespace: {{ include "karmada.namespace" . }}
  version: v1alpha1
  versionPriority: 10
{{- end }}
{{- end -}}
