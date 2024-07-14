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
  name: karmada:bootstrap-signer-clusterinfo
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
  name: karmada:bootstrap-signer-clusterinfo
  namespace: kube-public
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: karmada:bootstrap-signer-clusterinfo
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:anonymous
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: karmada:agent-bootstrap
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
kind: ClusterRoleBinding
metadata:
  name: karmada:agent-autoapprove-bootstrap
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers:karmada:default-cluster-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: karmada:agent-autoapprove-certificate-rotation
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:karmada:agent
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
rules:
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups:
  - cluster.karmada.io
  resources:
  - clusters
  verbs:
  - create
  - get
  - list
  - watch
  - patch
  - update
- apiGroups:
  - cluster.karmada.io
  resources:
  - clusters/status
  verbs:
  - patch
  - update
- apiGroups:
  - work.karmada.io
  resources:
  - works
  verbs:
  - create
  - get
  - list
  - watch
  - update
  - delete
- apiGroups:
  - work.karmada.io
  resources:
  - works/status
  verbs:
  - patch
  - update
- apiGroups:
  - config.karmada.io
  resources:
  - resourceinterpreterwebhookconfigurations
  - resourceinterpretercustomizations
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - list
  - watch
  - create
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - list
  - watch
  - create
  - patch
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
  - delete
  - get
  - patch
  - update
- apiGroups:
  - certificates.k8s.io
  resources:
  - certificatesigningrequests
  verbs:
  - create
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:karmada:agent
  {{- if "karmada.commonLabels" }}
  labels:
    {{- include "karmada.commonLabels" . | nindent 4 }}
  {{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:agent
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
{{- end -}}
