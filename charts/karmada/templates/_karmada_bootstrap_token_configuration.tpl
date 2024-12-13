{{- define "karmada.bootstrapToken.configuration" -}}
{{- $name := include "karmada.name" . -}}
{{- $namespace := include "karmada.namespace" . -}}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-info
  namespace: kube-public
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
data:
  kubeconfig: |
    apiVersion: v1
    clusters:
    - cluster:
        {{- include "karmada.kubeconfig.caData" . | nindent 8 }}
        server: https://{{ $name }}-apiserver.{{ $namespace }}.svc.{{ .Values.clusterDomain }}:5443
    kind: Config
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: system:karmada:bootstrap-signer-clusterinfo
  namespace: kube-public
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
rules:
- apiGroups:
  - ""
  resourceNames:
  - cluster-info
  resources:
  - configmaps
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system:karmada:bootstrap-signer-clusterinfo
  namespace: kube-public
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: system:karmada:bootstrap-signer-clusterinfo
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:anonymous
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:karmada:agent-bootstrap
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-bootstrapper
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers:karmada:default-cluster-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:karmada:certificatesigningrequest:autoapprover
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
rules:
- apiGroups:
  - certificates.k8s.io
  resources:
  - certificatesigningrequests/clusteragent
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:karmada:agent-autoapprove-bootstrap
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:certificatesigningrequest:autoapprover
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers:karmada:default-cluster-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:karmada:certificatesigningrequest:selfautoapprover
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
rules:
- apiGroups:
  - certificates.k8s.io
  resources:
  - certificatesigningrequests/selfclusteragent
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:karmada:agent-autoapprove-certificate-rotation
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:certificatesigningrequest:selfautoapprover
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:karmada:agents
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:karmada:agent-rbac-generator
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
rules:
- apiGroups:
  - "*"
  resources:
  - "*"
  verbs:
  - "*"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:karmada:agent-rbac-generator
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:agent-rbac-generator
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:karmada:agent:rbac-generator
{{- end -}}
