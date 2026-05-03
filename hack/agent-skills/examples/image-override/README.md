# Example — Multi-Container Image Registry Overrides

Shows how to retarget the image registry **only for the `app`
container** while leaving `sidecar` and `init` containers pulling from
the upstream registry. This is the use case the `predicate` field on
`imageOverrider` exists for.

## Scenario

- Workload has three containers: an `init` container, a main `app`
  container, and a `sidecar` container.
- On `member-cn` clusters, only the main `app` image must come from the
  internal mirror (`registry.cn-hangzhou.aliyuncs.com/myorg`). The other
  two are vendor-published and not mirrored.

## Manifests

- [`deployment.yaml`](deployment.yaml) — three containers.
- [`override-policy.yaml`](override-policy.yaml) — uses
  `imageOverrider[].predicate.path` to scope the rewrite.

## How `predicate` works

Without a predicate, an `imageOverrider` entry rewrites the registry
of **every** container image in the workload. With
`predicate: { path: "/spec/template/spec/containers/1/image" }` the
rewrite is restricted to that one JSON Pointer.

```yaml
overrideRules:
  - targetCluster:
      labelSelector:
        matchLabels:
          location: cn
    overriders:
      imageOverrider:
        - predicate:
            # Only the second container (index 1) — the "app" container.
            path: /spec/template/spec/containers/1/image
          component: Registry
          operator: replace
          value: registry.cn-hangzhou.aliyuncs.com/myorg
```

## Reproduce with the generator

The generator's `--override-image-predicate` flag wires up the
predicate path:

```bash
python3 hack/agent-skills/scripts/policy-generator.py \
  --kind OverridePolicy \
  --name app-cn-mirror --namespace default \
  --target-kind Deployment --target-name webapp \
  --clusters member-cn \
  --override-image-registry registry.cn-hangzhou.aliyuncs.com/myorg \
  --override-image-predicate /spec/template/spec/containers/1/image \
  --output hack/agent-skills/examples/image-override/override-policy.yaml
```

## Verify

```bash
python3 hack/agent-skills/scripts/validate-policy.py \
  hack/agent-skills/examples/image-override/override-policy.yaml
# expected: ok: no findings
```

## Common mistake the validator catches

Writing `imageOverride` (without the trailing `-r`) — the JSON tag is
`imageOverrider`. The webhook silently drops unknown fields under
`overriders`, so the policy applies cleanly but does **nothing**. Rule
`KMD210` flags this in audit.
