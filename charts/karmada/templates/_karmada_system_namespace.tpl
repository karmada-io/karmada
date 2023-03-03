{{- define "karmada.systemNamespace" -}}
---
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Values.systemNamespace }}
---
apiVersion: v1
kind: Namespace
metadata:
  name: karmada-cluster
{{- end -}}
