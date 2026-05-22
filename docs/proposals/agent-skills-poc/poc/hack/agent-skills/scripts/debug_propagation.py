#!/usr/bin/env python3
"""
debug_propagation.py — walk the Karmada propagation pipeline and report the first hop
where things stop.

Inputs are a bundle of YAML snapshots (one or more docs in a single file or a directory):
  - the resource template
  - any PropagationPolicy / ClusterPropagationPolicy that could match
  - any ResourceBinding (rb) or ClusterResourceBinding (crb) for it
  - any Work objects for it
  - the Cluster objects in the fleet

The script does no live kubectl calls — it operates on YAML the user has captured
(via `karmadactl get -A -o yaml`). This keeps it CI-friendly and lets the agent reason
about offline snapshots posted in bug reports.

Output is a single JSON report:
  {
    "verdict": "ok" | "policy-mismatch" | "no-binding" | "schedule-failed" |
               "no-work" | "work-failed" | "indeterminate",
    "hop_reached": "Object" | "Policy" | "ResourceBinding" | "Work" | "Member",
    "evidence": [ ... ],
    "next_steps": [ ... ]
  }

Usage:
    debug_propagation.py --bundle snapshot.yaml --target apps/v1/Deployment/default/global-frontend
"""
from __future__ import annotations

import argparse
import glob
import json
import os
import sys
from typing import Any

import yaml


def load_bundle(path: str) -> list[dict]:
    paths = [path]
    if os.path.isdir(path):
        paths = sorted(glob.glob(os.path.join(path, "**/*.yaml"), recursive=True))
    docs: list[dict] = []
    for p in paths:
        with open(p) as fh:
            for d in yaml.safe_load_all(fh):
                if isinstance(d, dict):
                    docs.append(d)
                elif isinstance(d, dict) and d.get("kind") == "List":
                    docs.extend(d.get("items", []))
    return docs


def parse_target(s: str) -> dict[str, str]:
    parts = s.split("/")
    if len(parts) == 5:
        api_group, version, kind, namespace, name = parts
        return {"apiVersion": f"{api_group}/{version}", "kind": kind, "namespace": namespace, "name": name}
    if len(parts) == 4:
        version, kind, namespace, name = parts
        return {"apiVersion": version, "kind": kind, "namespace": namespace, "name": name}
    raise SystemExit(f"--target must be apiGroup/version/Kind/namespace/name or version/Kind/namespace/name; got {s!r}")


def matches_selector(target: dict, sel: dict) -> bool:
    if sel.get("apiVersion") != target["apiVersion"]:
        return False
    if sel.get("kind") != target["kind"]:
        return False
    if sel.get("namespace") and sel["namespace"] != target["namespace"]:
        return False
    if sel.get("name"):
        return sel["name"] == target["name"]
    # labelSelector is checked elsewhere only if the target's labels are in the bundle
    return True


def find_target_obj(docs: list[dict], target: dict) -> dict | None:
    for d in docs:
        if (d.get("apiVersion") == target["apiVersion"]
                and d.get("kind") == target["kind"]
                and (d.get("metadata") or {}).get("name") == target["name"]
                and (d.get("metadata") or {}).get("namespace", "") == target["namespace"]):
            return d
    return None


def diagnose(docs: list[dict], target: dict) -> dict[str, Any]:
    evidence: list[str] = []
    target_obj = find_target_obj(docs, target)
    if not target_obj:
        return _verdict("indeterminate", "Object",
                        [f"target {target['kind']}/{target['name']} not present in bundle"],
                        ["include the resource template YAML in the bundle"])
    evidence.append(f"found target object {target['kind']}/{target['name']}")

    matching_policies = []
    for d in docs:
        if d.get("kind") not in ("PropagationPolicy", "ClusterPropagationPolicy"):
            continue
        for sel in (d.get("spec") or {}).get("resourceSelectors") or []:
            if matches_selector(target, sel):
                matching_policies.append(d)
                break
    if not matching_policies:
        return _verdict("policy-mismatch", "Policy",
                        evidence + ["no PropagationPolicy resourceSelectors matched the target"],
                        ["verify spec.resourceSelectors apiVersion (case-sensitive) and kind",
                         "if using name:, confirm spelling matches metadata.name exactly",
                         "if using labelSelector:, confirm the target carries those labels"])
    evidence.append(f"{len(matching_policies)} policy(ies) match: " +
                    ", ".join(((p.get('metadata') or {}).get('name') or '?') for p in matching_policies))

    bindings = [d for d in docs if d.get("kind") in ("ResourceBinding", "ClusterResourceBinding")
                and target["name"] in ((d.get("metadata") or {}).get("name") or "")]
    if not bindings:
        return _verdict("no-binding", "ResourceBinding",
                        evidence + ["no ResourceBinding observed for the target"],
                        ["check karmada-controller-manager logs (binding controller)",
                         "confirm the policy was actually admitted (kubectl get pp -n <ns>)",
                         "rule out a higher-priority policy already claiming the target"])
    evidence.append(f"{len(bindings)} binding(s) present")

    # Read RB status
    rb = bindings[0]
    conditions = ((rb.get("status") or {}).get("conditions") or [])
    scheduled = _cond(conditions, "Scheduled")
    fully_applied = _cond(conditions, "FullyApplied")
    if scheduled and scheduled.get("status") == "False":
        return _verdict("schedule-failed", "ResourceBinding",
                        evidence + [f"RB.status[Scheduled]=False reason={scheduled.get('reason')!r} message={scheduled.get('message')!r}"],
                        ["confirm Cluster objects have the labels the placement expects",
                         "if clusterAffinities (plural), the first group with any match short-circuits — list preferred groups first",
                         "see knowledge/02-troubleshooting.md case T-NoSchedule"])
    evidence.append("RB.Scheduled=True" if scheduled else "RB.Scheduled condition missing")

    works = [d for d in docs if d.get("kind") == "Work"
             and target["name"] in ((d.get("metadata") or {}).get("name") or "")]
    if not works:
        return _verdict("no-work", "Work",
                        evidence + ["RB is scheduled but no Work objects observed"],
                        ["check karmada-controller-manager logs (binding→work step)",
                         "verify execution-space namespaces karmada-es-<cluster> exist"])
    evidence.append(f"{len(works)} Work object(s) present")

    failed_works = []
    for w in works:
        wstatus = (w.get("status") or {}).get("conditions") or []
        applied = _cond(wstatus, "Applied")
        if applied and applied.get("status") != "True":
            failed_works.append((w["metadata"]["name"], applied.get("message")))
    if failed_works:
        return _verdict("work-failed", "Member",
                        evidence + [f"Work failed on {len(failed_works)} cluster(s): {failed_works}"],
                        ["read Work.status.manifestStatuses for member-side error verbatim",
                         "check for member-cluster admission webhooks / OPA / RBAC denying the manifest",
                         "see knowledge/02-troubleshooting.md case T-NoWork"])
    evidence.append("all observed Work objects report Applied=True")

    if fully_applied and fully_applied.get("status") != "True":
        return _verdict("indeterminate", "Member",
                        evidence + [f"RB.FullyApplied={fully_applied.get('status')!r}; partial dispatch"],
                        ["enumerate RB.spec.clusters vs Work objects to find missing cluster(s)"])

    return _verdict("ok", "Member", evidence, ["pipeline reached Member with no observed errors"])


def _cond(conds: list[dict], typ: str) -> dict | None:
    for c in conds:
        if c.get("type") == typ:
            return c
    return None


def _verdict(verdict: str, hop: str, evidence: list[str], next_steps: list[str]) -> dict:
    return {"verdict": verdict, "hop_reached": hop, "evidence": evidence, "next_steps": next_steps}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--bundle", required=True, help="YAML file or directory of snapshots")
    ap.add_argument("--target", required=True, help="apiVersion/Kind/namespace/name")
    args = ap.parse_args()

    docs = load_bundle(args.bundle)
    target = parse_target(args.target)
    report = diagnose(docs, target)
    json.dump(report, sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0 if report["verdict"] == "ok" else 1


if __name__ == "__main__":
    sys.exit(main())
