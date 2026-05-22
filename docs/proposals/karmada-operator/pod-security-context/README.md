---
title: Add Support for Specifying Pod Security Context for Components Provisioned by the Operator
authors:
  - "@jabellard"
reviewers:
  - "@RainbowMango"
  - "@zhzhuang-zju"
approvers:
  - "@RainbowMango"

creation-date: 2026-05-21

---

# Add Support for Specifying Pod Security Context for Components Provisioned by the Operator

## Summary

This proposal adds a `SecurityContext` field to the `CommonSettings` struct in the `Karmada` CRD. This allows users to configure [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) settings for all control plane components provisioned by the operator, enabling deployments in environments that enforce Kubernetes Pod Security Standards, CIS benchmarks, and organizational compliance policies.


## Motivation

Organizations deploying Karmada in production are increasingly subject to strict security policies enforced at the cluster level. Kubernetes Pod Security Standards (PSS) — particularly the **Restricted** profile — require that pods run as non-root, disallow privilege escalation, use a restrictive seccomp profile, and set explicit user/group IDs. Starting with Kubernetes 1.25, Pod Security Admission (PSA) is GA and many clusters enforce it at the namespace level. When an administrator configures the `karmada-system` namespace with `pod-security.kubernetes.io/enforce: restricted`, the operator's control plane pods are rejected outright because they lack security context configuration.

Beyond PSA, many organizations operating in regulated industries (finance, healthcare, government) must demonstrate compliance with the CIS Kubernetes Benchmark, which mandates that all workloads — including infrastructure components — run as non-root, drop all capabilities, disable privilege escalation, and set seccomp profiles. Without native security context support in the operator, users cannot bring Karmada control planes into compliance without manual post-deployment patching — which the operator will revert on the next reconciliation loop.



## Goals

- Allow users to configure pod-level security contexts on any Karmada control plane component via the `Karmada` CR.
- Follow the existing patcher pattern used for `Tolerations` and `Affinity`.


## Non-Goals

- Exposing container-level `SecurityContext` (as distinct from pod-level `PodSecurityContext`) in this initial iteration. Container-level settings such as `capabilities`, `readOnlyRootFilesystem`, and `allowPrivilegeEscalation` are valuable but may be addressed in a follow-up proposal.
- Providing defaulting logic that automatically sets security context fields. Components that currently ship without security context settings will continue that behavior as the baseline.
- Enforcing or validating security context values against any specific Pod Security Standard. If the user specifies `securityContext`, their value is used as-is (consistent with how `Tolerations` and `Affinity` work today).


## Proposal

### User Stories

1. **Deploying in a PSA-enforced namespace**
   As a platform engineer, my cluster enforces the `restricted` Pod Security Standard at the namespace level via Pod Security Admission. I need to configure security contexts for all Karmada control plane components so that the operator can successfully provision pods without being rejected by the admission controller.

   ```yaml
   apiVersion: operator.karmada.io/v1alpha1
   kind: Karmada
   metadata:
     name: karmada
     namespace: karmada-system
   spec:
     components:
       karmadaAPIServer:
         securityContext:
           runAsUser: 1000
           runAsGroup: 3000
           runAsNonRoot: true
           seccompProfile:
             type: RuntimeDefault
   ```

2. **Volume permission control for etcd data**
   As a platform engineer running local etcd through the operator, I need to set `fsGroup` on the etcd pods so that the persistent volume used for etcd data is owned by the correct group. Without this, etcd processes running as non-root cannot write to their data directory.

   ```yaml
   apiVersion: operator.karmada.io/v1alpha1
   kind: Karmada
   metadata:
     name: karmada
     namespace: karmada-system
   spec:
     components:
       etcd:
         local:
           securityContext:
             runAsUser: 1000
             fsGroup: 1000
             runAsNonRoot: true
   ```

3. **Meeting CIS benchmark compliance**
   As a security engineer responsible for CIS benchmark compliance, I need all workloads — including infrastructure components — to run as non-root with restrictive seccomp profiles to pass automated compliance scanning tools.

### Design Details

#### API Change

Add the following field to `CommonSettings`:

```go
// SecurityContext holds pod-level security attributes and common container settings
// for the component's pods. When specified, the operator applies these settings to the
// PodSpec of the workload (Deployment or StatefulSet) managing the component.
// This enables compliance with Pod Security Standards (e.g., Restricted profile),
// CIS benchmarks, and organizational security policies.
// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
// +optional
SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
```

#### Patcher Change

Add a `WithSecurityContext` builder method to the patcher and apply it in both `ForDeployment` and `ForStatefulSet`:

```go
func (p *Patcher) WithSecurityContext(sc *corev1.PodSecurityContext) *Patcher {
    p.securityContext = sc
    return p
}
```

```go
// In ForDeployment and ForStatefulSet:
if p.securityContext != nil {
    podSpec.SecurityContext = p.securityContext
}
```


### Test Plan

- **Unit tests:** Extend patcher unit tests to verify `SecurityContext` is correctly applied to both Deployment and StatefulSet pod specs, and that a nil value results in no change.
- **Integration tests:** Add test cases for at least one Deployment-based component and the etcd StatefulSet to verify end-to-end wiring.
