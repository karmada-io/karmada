{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "karmada.operator.fullname" -}}
{{- printf (include "common.names.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Return the proper karmada operator image name
*/}}
{{- define "karmada.operator.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.operator.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "karmada.operator.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.operator.image) "global" .Values.global) }}
{{- end -}}
