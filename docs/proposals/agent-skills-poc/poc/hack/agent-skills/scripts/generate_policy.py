#!/usr/bin/env python3
"""
generate_policy.py — deterministic skeleton generator for Karmada policies.

The agent calls this script with a small JSON intent. The script emits one or two YAML
documents: a PropagationPolicy and (optionally) an OverridePolicy. Doing the structural
work here, not in the LLM, eliminates the well-known hallucinations: misplaced
`replicaScheduling`, JSONPath-instead-of-JSONPointer, missing `resourceSelectors`,
case-broken `apiVersion`. The LLM's remaining job is to fill in user-specific values
(cluster names, weights, paths) — not to remember the shape of the API.

Intent schema (passed via --intent stdin or a JSON file):

    {
      "policy_name": "global-frontend",
      "namespace": "default",
      "kind_scope": "namespaced" | "cluster",
      "resources": [
        {"apiVersion": "apps/v1", "kind": "Deployment", "name": "global-frontend"}
      ],
      "placement": {
        "mode": "name-list" | "label-selector" | "ordered-groups",
        "clusters": ["member1", "member2"],         // for name-list
        "labels": {"environment": "production"},     // for label-selector
        "groups": [                                  // for ordered-groups
          {"name": "primary", "clusters": ["sg-1", "sg-2"]},
          {"name": "backup",  "clusters": ["id-1"]}
        ]
      },
      "replica_strategy": "duplicated" | "divided-weighted" | "divided-dynamic" | null,
      "weights": [{"clusters": ["member1"], "weight": 3}, {"clusters": ["member2"], "weight": 2}],
      "propagate_deps": true,
      "overrides": [
        {
          "target_clusters": ["member2"],
          "image": {"component": "Registry", "operator": "replace", "value": "mirror.id.internal"}
        },
        {
          "target_clusters": ["member2"],
          "plaintext": [{"path": "/data/theme", "operator": "replace", "value": "light-mode"}]
        }
      ]
    }
"""
from __future__ import annotations

import argparse
import json
import sys
from typing import Any

import yaml

API_VERSION = "policy.karmada.io/v1alpha1"


def build_propagation(intent: dict[str, Any]) -> dict[str, Any]:
    kind = "ClusterPropagationPolicy" if intent.get("kind_scope") == "cluster" else "PropagationPolicy"
    meta: dict[str, Any] = {"name": intent["policy_name"]}
    if kind == "PropagationPolicy":
        meta["namespace"] = intent.get("namespace", "default")

    spec: dict[str, Any] = {
        "resourceSelectors": _resource_selectors(intent["resources"]),
        "placement": _placement(intent.get("placement", {}), intent.get("replica_strategy"), intent.get("weights")),
    }
    if intent.get("propagate_deps"):
        spec["propagateDeps"] = True
    return {"apiVersion": API_VERSION, "kind": kind, "metadata": meta, "spec": spec}


def _resource_selectors(resources: list[dict]) -> list[dict]:
    if not resources:
        # R2: resourceSelectors must have MinItems=1. Fail loudly rather than emit invalid YAML.
        raise ValueError("intent.resources must contain at least one selector (CRD MinItems=1)")
    out = []
    for r in resources:
        sel: dict[str, Any] = {"apiVersion": r["apiVersion"], "kind": r["kind"]}
        if "name" in r:
            sel["name"] = r["name"]
        if "namespace" in r:
            sel["namespace"] = r["namespace"]
        if "labelSelector" in r and "name" not in r:
            sel["labelSelector"] = r["labelSelector"]
        out.append(sel)
    return out


def _placement(p: dict, replica_strategy: str | None, weights: list[dict] | None) -> dict[str, Any]:
    mode = p.get("mode", "name-list")
    out: dict[str, Any] = {}
    if mode == "name-list":
        out["clusterAffinity"] = {"clusterNames": list(p.get("clusters", []))}
    elif mode == "label-selector":
        out["clusterAffinity"] = {"labelSelector": {"matchLabels": dict(p.get("labels", {}))}}
    elif mode == "ordered-groups":
        out["clusterAffinities"] = [
            {"affinityName": g["name"], "clusterNames": list(g["clusters"])}
            for g in p.get("groups", [])
        ]
    else:
        raise ValueError(f"unknown placement mode: {mode}")

    if replica_strategy == "duplicated":
        out["replicaScheduling"] = {"replicaSchedulingType": "Duplicated"}
    elif replica_strategy == "divided-weighted":
        if not weights:
            raise ValueError("replica_strategy=divided-weighted requires `weights`")
        out["replicaScheduling"] = {
            "replicaSchedulingType": "Divided",
            "replicaDivisionPreference": "Weighted",
            "weightPreference": {
                "staticWeightList": [
                    {"targetCluster": {"clusterNames": list(w["clusters"])}, "weight": int(w["weight"])}
                    for w in weights
                ]
            },
        }
    elif replica_strategy == "divided-dynamic":
        out["replicaScheduling"] = {
            "replicaSchedulingType": "Divided",
            "replicaDivisionPreference": "Weighted",
            "weightPreference": {"dynamicWeight": "AvailableReplicas"},
        }
    elif replica_strategy is None:
        pass
    else:
        raise ValueError(f"unknown replica_strategy: {replica_strategy}")
    return out


def build_override(intent: dict[str, Any]) -> dict[str, Any] | None:
    overrides = intent.get("overrides")
    if not overrides:
        return None
    kind = "ClusterOverridePolicy" if intent.get("kind_scope") == "cluster" else "OverridePolicy"
    meta: dict[str, Any] = {"name": intent["policy_name"] + "-override"}
    if kind == "OverridePolicy":
        meta["namespace"] = intent.get("namespace", "default")
    rules: list[dict[str, Any]] = []
    for ov in overrides:
        rule: dict[str, Any] = {"targetCluster": {"clusterNames": list(ov["target_clusters"])}}
        overriders: dict[str, Any] = {}
        if "image" in ov:
            overriders["imageOverrider"] = [{
                "component": ov["image"]["component"],
                "operator": ov["image"]["operator"],
                "value": ov["image"]["value"],
            }]
        if "plaintext" in ov:
            for p in ov["plaintext"]:
                _check_json_pointer(p["path"])
            overriders["plaintext"] = [
                {"path": p["path"], "operator": p["operator"], "value": p["value"]}
                for p in ov["plaintext"]
            ]
        # R15: rules with mixed overrider types are kept in separate rules already because
        # the agent is asked to pass them as separate `overrides[]` entries.
        rule["overriders"] = overriders
        rules.append(rule)
    return {
        "apiVersion": API_VERSION,
        "kind": kind,
        "metadata": meta,
        "spec": {
            "resourceSelectors": _resource_selectors(intent["resources"]),
            "overrideRules": rules,
        },
    }


def _check_json_pointer(path: str) -> None:
    # R5: paths must be JSON Pointer (RFC 6901), not JSONPath.
    if path.startswith("$") or "." in path.split("/")[-1] and not path.startswith("/"):
        raise ValueError(
            f"path {path!r} looks like JSONPath; OverridePolicy requires JSON Pointer "
            "(e.g. /data/theme, /spec/template/spec/containers/0/image). See knowledge/04-hard-rules.md R5."
        )
    if not path.startswith("/"):
        raise ValueError(f"JSON Pointer must start with '/' (got {path!r})")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--intent", type=argparse.FileType("r"), default=sys.stdin,
                    help="JSON intent file; defaults to stdin")
    ap.add_argument("--out", type=argparse.FileType("w"), default=sys.stdout,
                    help="output YAML stream; defaults to stdout")
    args = ap.parse_args()

    try:
        intent = json.load(args.intent)
    except json.JSONDecodeError as e:
        print(f"invalid intent JSON: {e}", file=sys.stderr)
        return 2

    try:
        pp = build_propagation(intent)
        op = build_override(intent)
    except (KeyError, ValueError) as e:
        print(f"intent error: {e}", file=sys.stderr)
        return 2

    docs = [pp] + ([op] if op else [])
    args.out.write(yaml.safe_dump_all(docs, sort_keys=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
