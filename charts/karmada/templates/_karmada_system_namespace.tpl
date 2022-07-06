{{- define "karmada.systemNamespace" -}}
---
apiVersion: v1
kind: Namespace
metadata:
  name: {{ include "karmada.namespace" . }}
---
apiVersion: v1
kind: Namespace
metadata:
  name: karmada-cluster
{{- end -}}
