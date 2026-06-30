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

	// ClusterInfoRole defines a role with permission to get the cluster-info configmap
	ClusterInfoRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:bootstrap-signer-clusterinfo
  namespace: kube-public
rules:
- apiGroups:
  - ""
  resourceNames:
  - cluster-info
  resources:
  - configmaps
  verbs:
  - get
`

	// ClusterInfoRoleBinding authorizes system:anonymous to get the cluster-info configmap
	ClusterInfoRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:bootstrap-signer-clusterinfo
  namespace: kube-public
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: system:karmada:bootstrap-signer-clusterinfo
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:anonymous
`

	// CSRAutoApproverClusterRole defines a ClusterRole with permissions to automatically approve the agent CSRs when the agentcsrapproving controller is enabled by karmada-controller-manager
	CSRAutoApproverClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:certificatesigningrequest:autoapprover
rules:
  - apiGroups:
      - certificates.k8s.io
    resources:
      - certificatesigningrequests/clusteragent
    verbs:
      - create
`

	// CSRAutoApproverClusterRoleBinding authorizes Group system:bootstrappers:karmada:default-cluster-token to auto approve the agent CSRs
	CSRAutoApproverClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:agent-autoapprove-bootstrap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:certificatesigningrequest:autoapprover
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers:karmada:default-cluster-token
`
	// AgentBootstrapClusterRole defines a ClusterRole with permissions to create and get CSRs.
	AgentBootstrapClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:cluster-bootstrapper
rules:
  - apiGroups:
      - certificates.k8s.io
    resources:
      - certificatesigningrequests
    verbs:
      - create
      - get
`

	// AgentBootstrapClusterRoleBinding authorizes Group system:bootstrappers:karmada:default-cluster-token to obtain the CSRs.
	AgentBootstrapClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:agent-bootstrap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:cluster-bootstrapper
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers:karmada:default-cluster-token
`

	// CSRSelfAutoApproverClusterRole defines a ClusterRole with permissions to automatically approve the agent CSRs
	CSRSelfAutoApproverClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:certificatesigningrequest:selfautoapprover
rules:
  - apiGroups:
      - certificates.k8s.io
    resources:
      - certificatesigningrequests/selfclusteragent
    verbs:
      - create
`

	// CSRSelfAutoApproverClusterRoleBinding authorizes Group system:karmada:agents to automatically approve the agent CSRs
	CSRSelfAutoApproverClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:agent-autoapprove-certificate-rotation
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:certificatesigningrequest:selfautoapprover
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:karmada:agents
`

	// AgentRBACGeneratorClusterRole is not used for the connection between the karmada-agent and the control plane,
	// but is used by karmadactl register to generate the RBAC resources required by the karmada-agent.
	AgentRBACGeneratorClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:agent-rbac-generator
rules:
  - apiGroups: ["cluster.karmada.io"]
    resources: ["clusters"]
    verbs: ["get", "create", "delete", "list", "watch"]
  - apiGroups: ["cluster.karmada.io"]
    resources: ["clusters/status"]
    verbs: ["update"]
  - apiGroups: ["config.karmada.io"]
    resources: ["resourceinterpreterwebhookconfigurations", "resourceinterpretercustomizations"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "create"]
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["list", "watch"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "update", "patch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "create", "patch"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "create", "update"]
  - apiGroups: ["certificates.k8s.io"]
    resources: ["certificatesigningrequests"]
    verbs: ["get", "create", "delete"]
  - apiGroups: ["work.karmada.io"]
    resources: ["works"]
    verbs: ["get", "create", "list", "watch", "update", "delete"]
  - apiGroups: ["work.karmada.io"]
    resources: ["works/status"]
    verbs: ["update", "patch"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
    verbs: ["create", "delete"]
`

	// AgentRBACGeneratorClusterRoleBinding User `system:karmada:agent:rbac-generator` is specifically used during the `karmadactl register` process to generate restricted RBAC resources for the `karmada-agent`
	AgentRBACGeneratorClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    karmada.io/bootstrapping: rbac-defaults
  name: system:karmada:agent-rbac-generator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:agent-rbac-generator
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: system:karmada:agent:rbac-generator
`
)
