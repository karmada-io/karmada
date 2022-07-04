{{/* vim: set filetype=mustache: */}}

{{- define "karmada.name" -}}
{{- default .Release.Name -}}
{{- end -}}

{{- define "karmada.namespace" -}}
{{- default .Release.Namespace -}}
{{- end -}}

{{- define "karmada.apiserver.labels" -}}
{{- if .Values.apiServer.labels }}
{{- range $key, $value := .Values.apiServer.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-apiserver
{{- end }}
{{- end -}}

{{- define "karmada.apiserver.podLabels" -}}
{{- if .Values.apiServer.podLabels }}
{{- range $key, $value := .Values.apiServer.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.aggregatedApiserver.labels" -}}
{{- if .Values.aggregatedApiServer.labels }}
{{- range $key, $value := .Values.aggregatedApiServer.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-aggregated-apiserver
{{- end }}
{{- end -}}

{{- define "karmada.aggregatedApiserver.podLabels" -}}
{{- if .Values.aggregatedApiServer.podLabels }}
{{- range $key, $value := .Values.aggregatedApiServer.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.kube-cm.labels" -}}
{{- if .Values.kubeControllerManager.labels }}
{{- range $key, $value := .Values.kubeControllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-kube-controller-manager
{{- end }}
{{- end -}}

{{- define "karmada.kube-cm.podLabels" -}}
{{- if .Values.kubeControllerManager.podLabels }}
{{- range $key, $value := .Values.kubeControllerManager.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.kubeconfig.volume" -}}
{{- $name := include "karmada.name" . -}}
- name: kubeconfig-secret
  secret:
    secretName: {{ $name }}-kubeconfig
{{- end -}}

{{- define "karmada.kubeconfig.volumeMount" -}}
{{- $name := include "karmada.name" . -}}
- name: kubeconfig-secret
  subPath: kubeconfig
  mountPath: /etc/kubeconfig
{{- end -}}

{{- define "karmada.cm.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.controllerManager.labels -}}
{{- range $key, $value := .Values.controllerManager.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}-controller-manager
{{- end -}}
{{- end -}}

{{- define "karmada.cm.podLabels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.controllerManager.podLabels }}
{{- range $key, $value := .Values.controllerManager.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}


{{- define "karmada.scheduler.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.scheduler.labels -}}
{{- range $key, $value := .Values.scheduler.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}-scheduler
{{- end -}}
{{- end -}}

{{- define "karmada.scheduler.podLabels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.scheduler.podLabels }}
{{- range $key, $value := .Values.scheduler.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}


{{- define "karmada.descheduler.labels" -}}
{{ $name :=  include "karmada.name" . }}
{{- if .Values.descheduler.labels -}}
{{- range $key, $value := .Values.descheduler.labels }}
{{ $key }}: {{ $value }}
{{- end -}}
{{- else -}}
app: {{$name}}
{{- end -}}
{{- end -}}

{{- define "karmada.descheduler.podLabels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.descheduler.podLabels }}
{{- range $key, $value := .Values.descheduler.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.descheduler.kubeconfig.volume" -}}
- name: kubeconfig-secret
  secret:
    secretName: karmada-kubeconfig
{{- end -}}


{{- define "karmada.webhook.labels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.webhook.labels }}
{{- range $key, $value := .Values.webhook.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{$name}}-webhook
{{- end }}
{{- end -}}

{{- define "karmada.webhook.podLabels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.webhook.podLabels }}
{{- range $key, $value := .Values.webhook.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.agent.labels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.agent.labels }}
{{- range $key, $value := .Values.agent.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{$name}}
{{- end }}
{{- end -}}

{{- define "karmada.agent.podLabels" -}}
{{ $name :=  include "karmada.name" .}}
{{- if .Values.agent.podLabels }}
{{- range $key, $value := .Values.agent.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.webhook.caBundle" -}}
{{- if eq .Values.certs.mode "auto" }}
caBundle: {{ print "{{ ca_crt }}" }}
{{- end }}
{{- if eq .Values.certs.mode "custom" }}
caBundle: {{ b64enc .Values.certs.custom.caCrt }}
{{- end }}
{{- end -}}

{{- define "karmada.schedulerEstimator.podLabels" -}}
{{- if .Values.schedulerEstimator.podLabels }}
{{- range $key, $value := .Values.schedulerEstimator.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.schedulerEstimator.labels" -}}
{{- if .Values.schedulerEstimator.labels }}
{{- range $key, $value := .Values.schedulerEstimator.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "karmada.search.labels" -}}
{{- if .Values.search.labels }}
{{- range $key, $value := .Values.search.labels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- else}}
app: {{- include "karmada.name" .}}-search
{{- end }}
{{- end -}}

{{- define "karmada.search.podLabels" -}}
{{- if .Values.search.podLabels }}
{{- range $key, $value := .Values.search.podLabels }}
{{ $key }}: {{ $value }}
{{- end }}
{{- end }}
{{- end -}}

{{/*
Return the proper karmada internal etcd image name
*/}}
{{- define "karmada.internal.etcd.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.etcd.internal.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada agent image name
*/}}
{{- define "karmada.agent.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.agent.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada apiServer image name
*/}}
{{- define "karmada.apiServer.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.apiServer.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada controllerManager image name
*/}}
{{- define "karmada.controllerManager.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.controllerManager.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada descheduler image name
*/}}
{{- define "karmada.descheduler.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.descheduler.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada schedulerEstimator image name
*/}}
{{- define "karmada.schedulerEstimator.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.schedulerEstimator.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada scheduler image name
*/}}
{{- define "karmada.scheduler.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.scheduler.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada webhook image name
*/}}
{{- define "karmada.webhook.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.webhook.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada aggregatedApiServer image name
*/}}
{{- define "karmada.aggregatedApiServer.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.aggregatedApiServer.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada search image name
*/}}
{{- define "karmada.search.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.search.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper karmada kubeControllerManager image name
*/}}
{{- define "karmada.kubeControllerManager.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.kubeControllerManager.image "global" .Values.global) }}
{{- end -}}



