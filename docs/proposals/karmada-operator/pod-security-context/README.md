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

This proposal adds a `PodSecurityContext` field to the `CommonSettings` struct in the `Karmada` CRD. This allows users to configure [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) settings for all control plane components provisioned by the operator, enabling deployments in environments that enforce Kubernetes Pod Security Standards, CIS benchmarks, and organizational compliance policies.


## Motivation

Organizations deploying Karmada in production are increasingly subject to strict security policies enforced at the cluster level. Kubernetes Pod Security Standards (PSS) — particularly the **Restricted** profile — require that pods run as non-root, disallow privilege escalation, use a restrictive seccomp profile, and set explicit user/group IDs. Starting with Kubernetes 1.25, Pod Security Admission (PSA) is GA and many clusters enforce it at the namespace level. When an administrator configures the `karmada-system` namespace with `pod-security.kubernetes.io/enforce: restricted`, the operator's control plane pods are rejected outright because they lack security context configuration.

Beyond PSA, many organizations operating in regulated industries (finance, healthcare, government) must demonstrate compliance with the CIS Kubernetes Benchmark, which mandates that all workloads — including infrastructure components — run as non-root, drop all capabilities, disable privilege escalation, and set seccomp profiles. Without native security context support in the operator, users cannot bring Karmada control planes into compliance without manual post-deployment patching — which the operator will revert on the next reconciliation loop.



## Goals

- Allow users to configure pod-level security contexts on any Karmada control plane component via the `Karmada` CR.
- Follow the existing patcher pattern used for `Tolerations` and `Affinity`.
- Expose the API as an opt-in configuration surface that defers all policy decisions to the user; the operator does not set defaults or validate values beyond what the Kubernetes API server enforces.


## Non-Goals

- Exposing container-level `SecurityContext` (as distinct from pod-level `PodSecurityContext`) in this initial iteration. Container-level settings such as `capabilities`, `readOnlyRootFilesystem`, and `allowPrivilegeEscalation` are valuable but may be addressed in a follow-up proposal.
- Providing defaulting logic that automatically sets security context fields. Components that currently ship without security context settings will continue that behavior as the baseline. Introducing operator-managed defaults is a broader effort that requires per-component compatibility testing and should be addressed in a dedicated follow-up proposal.
- Enforcing or validating security context values against any specific Pod Security Standard. If the user specifies `podSecurityContext`, their value is used as-is (consistent with how `Tolerations` and `Affinity` work today).
- Guaranteeing that every combination of pod security context settings is compatible with every Karmada component. Some settings (e.g., `runAsUser`, `runAsNonRoot`) may require component-level changes (such as adjusting file permissions or default listening ports) to function correctly. This proposal provides the configuration surface; component-level hardening is tracked separately.


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
         podSecurityContext:
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
           podSecurityContext:
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
// PodSecurityContext holds pod-level security attributes for the component's pods.
// When specified, the operator applies these settings to the PodSpec of the workload
// (Deployment or StatefulSet) managing the component. This enables compliance with
// Pod Security Standards (e.g., Restricted profile), CIS benchmarks, and
// organizational security policies.
// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
// +optional
PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
```

#### Patcher Change

Add a `WithPodSecurityContext` builder method to the patcher and apply it in both `ForDeployment` and `ForStatefulSet`:

```go
func (p *Patcher) WithPodSecurityContext(sc *corev1.PodSecurityContext) *Patcher {
    p.podSecurityContext = sc
    return p
}
```

```go
// In ForDeployment and ForStatefulSet:
if p.podSecurityContext != nil {
    podSpec.SecurityContext = p.podSecurityContext
}
```


### Risks and Mitigations

**Risk: Certain pod security context settings may cause component failures.**
Some Karmada components may not function correctly under specific security settings. For example, prior testing has shown that `runAsUser` and `runAsNonRoot` can cause functional issues with certain components that assume root privileges or write to directories owned by root.

*Mitigation:* This proposal intentionally exposes `PodSecurityContext` as an opt-in, per-component field with no operator-managed defaults. Users who configure these settings are expected to validate their values against the target component. The operator does not apply any security context unless explicitly specified, preserving the existing behavior as the baseline. Known compatibility constraints should be documented as they are identified. A future proposal can introduce vetted defaults for individual components once the necessary component-level changes (e.g., adjusting file permissions, non-root-compatible base images, binding to non-privileged ports) have been completed and validated.

### Test Plan

- **Unit tests:** Extend patcher unit tests to verify `PodSecurityContext` is correctly applied to both Deployment and StatefulSet pod specs, and that a nil value results in no change.
- **Integration tests:** Add test cases for at least one Deployment-based component and the etcd StatefulSet to verify end-to-end wiring.
