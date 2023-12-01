{{- define "karmada.systemNamespace" -}}
---
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Values.systemNamespace }}
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
---
apiVersion: v1
kind: Namespace
metadata:
  name: karmada-cluster
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
{{- end -}}
