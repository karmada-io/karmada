---
title: Cluster(karmada-agent) Authorization
authors:
- "@everpeace"
reviewers:
- T.B.D.
approvers:
- T.B.D.

creation-date: 2024-01-29
update-date: 2024-01-29

---

# Cluster(karmada-agent) Authorization

## Summary

_Cluster Authorization_ allows karmada-agents(member clusters) permissions to be managed more strictly. By enabling this feature, a karmada-agents will be restricted to the objects it is associated with. For example, a karmada-agent can only access the Secret referenced by the Cluster to which it is associated. This minimizes the security risk of a karmada-agent's credentials being compromised.


## Motivation

As of karmada v1.8, karmada-agent uses `user: system:node:<cluster-name>, groups: [system:nodes]` identity when accessing to the karmada control plane. This is because it relies on the Kubernetes auto approving controller to ensure long-term, secure and stable access from karmada-agent to the control plane (ref: https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/#kubernetes-signers).

Since the `system:nodes` group has strong cluster-wide privileges([ClusterRole and ClusterRoleBinding](https://github.com/karmada-io/karmada/blob/v1.8.0/charts/karmada/templates/_karmada_bootstrap_token_configuration.tpl#L87-L201)), any karmada-agent can have unrestricted access to `Cluster, Lease, Secret`, etc. So any karmada-agent can modifies/deletes critical resources owned by other karmada-agents.

Moreover, because any agents registers member cluster's token and impersonate token in control plane by default, any karmada-agents can invoke impersonate access to any member clusters if network is reachable.  It's dangerous when member clusters are managed by other teams/organizations from control plane. In this situation, whole clusters(all the members, and control plane) would be taken by malicious attacker even if only one member cluster is compromised.

Thus, karmada should introduce a way such that karmada-agent can run with the least privilege.

## Proposal

Similar to [Kubernetes Node Authorization](https://kubernetes.io/docs/reference/access-authn-authz/node/), Cluster Authorization consists of two components:

- Mutating Admission Webhook:
  - when a karmada-agent(`system:node:member-1`) creates some resources in control plane, this webhook puts `clusterrestriction.karmada.io/owner: "system:node:member-1"`
  - when a karmada-agent update/delete some resources in control plane, this webhook prevent from modifying the annotations.
- Authorization Webhook:
  - when a karmada-agent(`system:node:member-1`) reads/writes some resources, this webhook only allows the operation only on resources with owned by itself

## Design Details

### Mutating Admission Webhook

Introducing mutating admission webhook on karmada-webhook such that:

- the server is only interested in requests from karmada-agents (i.e. `SubjectAccessReview.Spec.User==system:node:...` )
- the server puts `clusterrestriction.karmada.io/owner: "system:node:..."` in `CREATE` requests
- the server prohibits modifying/deleting `clusterrestriction.karmada.io/owner` annotations in `UPDATE`, `DELETE` and `CONNECT` requests

### Authorization Webhook

Adding authorization webhook on karmada control plane's kube-apiserver such that:

- the server watches resources in karmada control plane, 
- the server is only interested in requests from karmada-agents (i.e. `SubjectAccessReview.Spec.User==system:node:...` )
- when receiving `SubjectAccessReview` requests with `resourceAttributes` and `verb=get/update/delete`,
- the server checks the target resource have `clusterrestriction.karmada.io/owner` annotation,
- allow the access only when the annotation value matches on `SubjectAccessReview.Spec.User`

## Alternatives

### Generating fine-grained RBAC entries for karmada-agent

We can consider alternative architecture generating fine-grained RBAC entries for karmada-agents. When `Cluster` resource created, fine grained RBAC entries with `resourceNames` by some controllers. Assume `system:node:member-1` creates `Cluster` resource on control plane belows:

```yaml
apiVersion: cluster.karmada.io/v1alpha1
kind: Cluster
metadata:
  name: member-1
spec:
  id: 3da8b942-daae-4f45-9079-19e6a81a8660
  impersonatorSecretRef:
    name: member-1-impersonator
    namespace: karmada-system
  secretRef:
    name: member-1
    namespace: karmada-system
  syncMode: Pull
...
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:karmada:agent:member-1
rules:
- apiGroups:
  - cluster.karmada.io
  resources:
  - clusters/status
  resourceNames:
  - member-1
  verbs:
  - create
  - get
  - list
  - watch
  - patch
  - update
  - delete
- apiGroups:
  - cluster.karmada.io
  resources:
  - clusters
  resourceNames:
  - member-1
  verbs:
  - create
  - get
  - list
  - watch
  - patch
  - update
  - delete
- apiGroups:
  - ""
  resources:
  - namespaces
  resourceNames:
  - karmada-es-member-1
  verbs:
  - create
  - get
  - list
  - watch
  - patch
  - update
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:karmada:agent:member-1
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:agent:member-1
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:node:member-1
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: system:karmada:agent:member-1
  namespace: karmada-system
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  resourceNames:
  - member-1
  - member-1-impersonates
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  resourceNames:
  - karmada-agent-member-1
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system:karmada:agent:member-1
  namespace: karmada-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:karmada:agent:member-1
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:node:member-1
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: system:karmada:agent:member-1
  namespace: karmada-cluster
rules:
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  resourceNames:
  - member-1
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system:karmada:agent-lease-$c
  namespace: karmada-cluster
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: system:karmada:agent:member-1
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:node:member-1
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: system:karmada:agent:member-1
  namespace: karmada-es-member-1
rules:
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
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system:karmada:agent:member-1
  namespace: karmada-es-member-1
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: system:karmada:agent:member-1
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: system:node:member-1
```

However, it would generate so many RBAC entries in control plane, and would be very hard to manage for control plane administrators. So, we don't take this way.

