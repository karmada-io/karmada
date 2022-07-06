{{- define "karmada.proxyRbac" -}}
{{- $name := include "karmada.name" . -}}

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ $name }}-cluster-proxy-admin
rules:
  - apiGroups:
      - 'cluster.karmada.io'
    resources:
      - clusters/proxy
    verbs:
      - '*'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ $name }}-cluster-proxy-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ $name }}-cluster-proxy-admin
subjects:
  - kind: User
    name: "system:admin"
{{- end -}}

