{{- define "karmada.clusterrole" -}}
# This configuration is used to grant the admin clusterrole read
# and write permissions for Karmada resources.

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    # refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#auto-reconciliation
    # and https://kubernetes.io/docs/reference/access-authn-authz/rbac/#kubectl-auth-reconcile
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    # refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#default-roles-and-role-bindings
    kubernetes.io/bootstrapping: rbac-defaults
    # used to aggregate rules to view clusterrole
    rbac.authorization.k8s.io/aggregate-to-view: "true"
    {{- include "karmada.commonLabels" . | nindent 4 }}
  name: karmada-view
rules:
  - apiGroups:
      - "autoscaling.karmada.io"
    resources:
      - cronfederatedhpas
      - cronfederatedhpas/status
      - federatedhpas
      - federatedhpas/status
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - "multicluster.x-k8s.io"
    resources:
      - serviceexports
      - serviceexports/status
      - serviceimports
      - serviceimports/status
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - "networking.karmada.io"
    resources:
      - multiclusteringresses
      - multiclusteringresses/status
      - multiclusterservices
      - multiclusterservices/status
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - "policy.karmada.io"
    resources:
      - overridepolicies
      - propagationpolicies
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - "work.karmada.io"
    resources:
      - resourcebindings
      - resourcebindings/status
      - works
      - works/status
    verbs:
      - get
      - list
      - watch

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    # refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#auto-reconciliation
    # and https://kubernetes.io/docs/reference/access-authn-authz/rbac/#kubectl-auth-reconcile
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    # refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#default-roles-and-role-bindings
    kubernetes.io/bootstrapping: rbac-defaults
    # used to aggregate rules to view clusterrole
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    {{- include "karmada.commonLabels" . | nindent 4 }}
  name: karmada-edit
rules:
  - apiGroups:
      - "autoscaling.karmada.io"
    resources:
      - cronfederatedhpas
      - cronfederatedhpas/status
      - federatedhpas
      - federatedhpas/status
    verbs:
      - create
      - delete
      - deletecollection
      - patch
      - update
  - apiGroups:
      - "multicluster.x-k8s.io"
    resources:
      - serviceexports
      - serviceexports/status
      - serviceimports
      - serviceimports/status
    verbs:
      - create
      - delete
      - deletecollection
      - patch
      - update
  - apiGroups:
      - "networking.karmada.io"
    resources:
      - multiclusteringresses
      - multiclusteringresses/status
      - multiclusterservices
      - multiclusterservices/status
    verbs:
      - create
      - delete
      - deletecollection
      - patch
      - update
  - apiGroups:
      - "policy.karmada.io"
    resources:
      - overridepolicies
      - propagationpolicies
    verbs:
      - create
      - delete
      - deletecollection
      - patch
      - update
  - apiGroups:
      - "work.karmada.io"
    resources:
      - resourcebindings
      - resourcebindings/status
      - works
      - works/status
    verbs:
      - create
      - delete
      - deletecollection
      - patch
      - update
{{- end -}}
