# Propagation Troubleshooting Catalog

Each entry has a symptom phrased as the user would phrase it, the diagnostic walk along
the `Object → Policy → ResourceBinding → Work → member` pipeline, and the most common
root cause. The `karmada-debug-propagation` skill walks these in order.

## T-NoRB — "I applied my policy and nothing happened"

**Walk:**
1. `karmadactl get propagationpolicy -n <ns>` — does the policy exist? Right namespace?
2. `karmadactl get resourcebinding -n <ns>` — is there an RB whose name starts with the
   resource template's name? If no RB, the policy did not match.
3. Check `spec.resourceSelectors` against the actual resource:
   - `apiVersion` must match exactly (`apps/v1`, not `apps`).
   - `kind` is case-sensitive.
   - If `name:` is set, `labelSelector` is ignored — confirmed in `propagation_types.go`.
4. Check for a higher-priority policy already owning the resource. Look at
   `metadata.labels` on the resource template for `propagationpolicy.karmada.io/name`.

**Most common root cause:** typo in `apiVersion` or `kind`.

## T-NoWork — "RB exists but member clusters are empty"

**Walk:**
1. `karmadactl describe rb <name>` — read `status.conditions`.
   - `Scheduled=False` means the scheduler rejected the placement. See T-NoSchedule.
   - `Scheduled=True, FullyApplied=False` means dispatch failed. Continue.
2. `karmadactl get work -A | grep <rb-name>` — one Work per target cluster.
3. `karmadactl describe work <name> -n karmada-es-<cluster>` — read `status.conditions`
   and `status.manifestStatuses`.

**Most common root cause:** the resource is well-formed in the control plane but the
member cluster rejected it on apply (RBAC, admission webhook, OPA policy). The Work's
`status.manifestStatuses[*].health` carries the member-side error verbatim.

## T-NoSchedule — "scheduler says no clusters match"

**Walk:**
1. `karmadactl describe rb <name>` — `Scheduled=False` with a reason like
   `NoClusterFit` or `ClusterAffinityNotMatch`.
2. Re-read `spec.placement` against the actual `Cluster` objects:
   - `karmadactl get clusters --show-labels`
   - Verify the label selector matches what you expected.
3. For `clusterAffinities` (plural), the scheduler tries each affinity group in order
   and stops at the first that yields any cluster. If you expected the second group to
   activate, the first must have matched zero clusters.

**Most common root cause:** the user thinks `clusterAffinity` filters by member cluster
*node* labels; it actually filters by labels on the `Cluster` object in the Karmada
control plane. These are not the same labels.

## T-Override-NotApplied — "the policy is matched but the override didn't take effect"

**Walk:**
1. Confirm the OverridePolicy is in the same namespace as the resource template, OR is
   a ClusterOverridePolicy.
2. Confirm `spec.resourceSelectors` matches the resource template (same rules as
   PropagationPolicy — apiVersion is case-sensitive).
3. For each `overrideRules` entry, confirm `targetCluster` matches the destination cluster.
4. Inspect the rendered manifest on the member cluster:
   `kubectl --context member1 get deploy <name> -o yaml`. If the field is unchanged, the
   override was probably scoped out by `targetCluster`.

**Most common root cause:** The `path` is a JSON Pointer, not JSONPath. Users write
`$.spec.template.spec.containers[0].image` instead of
`/spec/template/spec/containers/0/image`. The webhook does not catch this — the path
just silently misses.

## T-Deps-Missing — "Deployment propagated but its ConfigMap didn't"

**Walk:**
1. Is `spec.propagateDeps: true` set on the PropagationPolicy? If not, dependencies are
   not auto-propagated; they need their own policy or must be listed explicitly in
   `resourceSelectors`.
2. If `propagateDeps: true` is set, check whether a `ResourceInterpreter` exists for
   the resource type. Built-in Deployments/StatefulSets/DaemonSets are handled natively;
   custom CRDs require a Lua interpreter (`CustomizedResourceInterpreter`) or a
   webhook interpreter.
3. Inspect the RB: `karmadactl describe rb <name>` — the `spec.requiredBy` and
   `spec.referencedResources` fields show what the interpreter discovered.

**Most common root cause:** custom CRD with no `ResourceInterpreter` registered.

## T-Stuck-Terminating — "I deleted my PropagationPolicy and the workloads are gone too,
but the policy itself is stuck in Terminating"

**Walk:**
1. `karmadactl describe pp <name>` — look for finalizers. Karmada uses
   `karmada.io/policy-deleting` and similar.
2. Check controller logs:
   `kubectl logs -n karmada-system deploy/karmada-controller-manager | grep <name>`.
3. If a member cluster is unreachable, the execution controller cannot confirm cleanup
   and the finalizer hangs. Either restore the cluster or `karmadactl unjoin` it first.

**Most common root cause:** an unhealthy member cluster blocks finalizer removal.

## T-ApiVersionMismatch — "strict decoding error" on apply

**Walk:**
1. The error names the offending field. Usually a deprecated alias from a long-deprecated
   API version that has since been removed.
2. Cross-check the field against the CRD at `charts/karmada/_crds/bases/policy/`.
3. If the field is genuinely current and the error persists, the user is talking to a
   Karmada control plane older than the CRD they copied the example from.

**Most common root cause:** copy-pasted example from a blog post written against an older
Karmada version.

## Quick `karmadactl` commands an agent should reach for

```
karmadactl get pp,cpp,op,cop -A                   # what policies exist
karmadactl get rb,crb -A                          # what bindings exist
karmadactl get work -A                            # what is dispatched
karmadactl describe rb <name> -n <ns>             # full schedule + apply story
karmadactl get clusters --show-labels             # what the scheduler can see
karmadactl --kubeconfig=member1.config get all -A # confirm member-side state
```

When a user says "X did not propagate", the agent should never guess. It should run
through the pipeline in order: Object → Policy → ResourceBinding → Work → member. Each
hop has its own `status.conditions` block; read them.
