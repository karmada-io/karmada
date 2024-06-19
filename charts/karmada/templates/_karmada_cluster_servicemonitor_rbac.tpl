{{- define "karmada.serviceMonitorRbac" -}}
{{- $name := include "karmada.name" . -}}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ $name }}-servicemonitor
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
rules:
  - nonResourceURLs:
    - "/metrics"
    verbs:
      - 'get'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ $name }}-servicemonitor
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ $name }}-servicemonitor
subjects:
  - kind: User
    name: "servicemonitor"
{{- end -}}
