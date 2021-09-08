{{- define "karmada.systemNamespace" -}}
---
apiVersion: v1
kind: Namespace
metadata:
  name: karmada-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: karmada-cluster
{{- end -}}
