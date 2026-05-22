---
name: karmada-debug-propagation
description: Diagnose why a resource did not propagate to a member cluster. Walks the pipeline Object → PropagationPolicy → ResourceBinding → Work → Member and reports the first hop where the chain breaks. Use when the user says "X isn't showing up in member cluster Y".
metadata:
  type: workflow
  loads:
    - knowledge/02-troubleshooting.md
    - knowledge/03-components.md
    - knowledge/00-overview.md
  uses_scripts:
    - scripts/debug_propagation.py
---

# karmada-debug-propagation

A user reports a propagation failure. The agent's job:

1. Get a bundle of YAML snapshots (the resource template, policies, RBs, Works,
   Clusters). Ask for them explicitly if missing:
   ```
   karmadactl get pp,cpp,op,cop,rb,crb,work -A -o yaml > bundle.yaml
   karmadactl get clusters -o yaml >> bundle.yaml
   kubectl get <kind>/<name> -n <ns> -o yaml >> bundle.yaml
   ```
2. Run the diagnoser:
   ```
   python3 hack/agent-skills/scripts/debug_propagation.py \
       --bundle bundle.yaml \
       --target apps/v1/Deployment/default/global-frontend
   ```
3. Read the JSON verdict. It names the first hop that failed and lists evidence +
   suggested next steps.
4. Translate the verdict into a user-friendly answer that cites the matching
   troubleshooting case in `knowledge/02-troubleshooting.md`.

## Verdict → troubleshooting-case mapping

| verdict             | hop_reached       | knowledge/02-troubleshooting.md case |
|---------------------|-------------------|--------------------------------------|
| `policy-mismatch`   | Policy            | T-NoRB                               |
| `no-binding`        | ResourceBinding   | T-NoRB                               |
| `schedule-failed`   | ResourceBinding   | T-NoSchedule                         |
| `no-work`           | Work              | T-NoWork (binding→work step)         |
| `work-failed`       | Member            | T-NoWork (member-side reject)        |
| `indeterminate`     | varies            | re-ask user for fuller snapshot      |
| `ok`                | Member            | report no failure observed           |

## What the diagnoser does NOT do

- It does not run live `kubectl` commands. The user provides snapshots; this keeps
  the helper CI-friendly and lets the agent reason about offline bug reports.
- It does not infer OverridePolicy failures (those are silent in the pipeline; the
  resource still applies, just without the override). Route override symptoms to
  case T-Override-NotApplied in `knowledge/02-troubleshooting.md`.

## When the user has no snapshot

Tell them the exact one-liner above. Do not guess at the cause from symptoms alone —
the failure can be at any of five hops and guessing is what skills are supposed to
prevent.

## Failure modes

- **The target is missing from the bundle.** The diagnoser returns
  `verdict=indeterminate, hop_reached=Object`. Ask the user to include the resource
  template in the bundle.
- **The bundle is from a different control plane.** RB / Work objects exist but with
  unrelated UIDs. The diagnoser will still walk the pipeline; the agent should note
  if owner references don't match the target.
- **The user expected the override to apply but the propagation itself succeeded.**
  Override failures show as "deployed but field not changed". Switch to T-Override-NotApplied
  and read the rendered manifest from the member cluster.
