/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rbac

const (
	// KarmadaResourceViewClusterRole clusterrole for view karmada resources
	KarmadaResourceViewClusterRole = `
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
`
	// KarmadaResourceEditClusterRole clusterrole for edit karmada resources
	KarmadaResourceEditClusterRole = `
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
`
	// ClusterProxyAdminClusterRole role to proxy member clusters
	ClusterProxyAdminClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-proxy-admin
rules:
- apiGroups:
  - 'cluster.karmada.io'
  resources:
  - clusters/proxy
  verbs:
  - '*'
`
	// ClusterProxyAdminClusterRoleBinding authorize system:admin to proxy member clusters
	ClusterProxyAdminClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-proxy-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-proxy-admin
subjects:
  - kind: User
    name: "system:admin"
`
)
