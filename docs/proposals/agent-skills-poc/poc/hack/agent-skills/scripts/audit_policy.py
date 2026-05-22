#!/usr/bin/env python3
"""
audit_policy.py — deterministic linter for Karmada PropagationPolicy / OverridePolicy YAML.

The `karmada-audit-policy` skill runs this first. The agent only adds the semantic
findings the linter cannot catch (intent mismatches, security concerns).

Each finding is one rule from knowledge/04-hard-rules.md plus a couple of structural
checks. Output is JSON for easy LLM consumption, with severity ∈ {error, warn, info}.

Usage:
    audit_policy.py path/to/policy.yaml
    cat policy.yaml | audit_policy.py -
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from typing import Any

import yaml

VALID_KINDS = {"PropagationPolicy", "ClusterPropagationPolicy", "OverridePolicy", "ClusterOverridePolicy"}
DEPRECATED_PURGE_MODES = {"Immediately", "Graciously"}


def lint_doc(doc: dict[str, Any], idx: int) -> list[dict]:
    findings: list[dict] = []
    where = f"docs[{idx}]"
    kind = doc.get("kind")
    if kind not in VALID_KINDS:
        findings.append(_f("error", "STRUCT", where, f"unknown kind {kind!r}; expected one of {sorted(VALID_KINDS)}"))
        return findings

    api_version = doc.get("apiVersion")
    if api_version != "policy.karmada.io/v1alpha1":
        findings.append(_f("error", "R3", where + ".apiVersion",
                           f"apiVersion {api_version!r} is wrong; expected 'policy.karmada.io/v1alpha1'"))

    spec = doc.get("spec") or {}
    if kind.endswith("PropagationPolicy"):
        findings.extend(_lint_propagation(spec, where))
    else:
        findings.extend(_lint_override(spec, where))
    return findings


def _lint_propagation(spec: dict, where: str) -> list[dict]:
    out: list[dict] = []

    # R2: resourceSelectors required, MinItems=1
    selectors = spec.get("resourceSelectors")
    if not selectors:
        out.append(_f("error", "R2", where + ".spec.resourceSelectors",
                      "resourceSelectors is required and must have at least one item (CRD MinItems=1)"))
    else:
        for i, sel in enumerate(selectors):
            out.extend(_lint_resource_selector(sel, f"{where}.spec.resourceSelectors[{i}]"))

    # R1: replicaScheduling must NOT be at spec root
    if "replicaScheduling" in spec:
        out.append(_f("error", "R1", where + ".spec.replicaScheduling",
                      "replicaScheduling is misplaced; move it under spec.placement.replicaScheduling"))

    placement = spec.get("placement") or {}
    # R4: mutually exclusive
    if "clusterAffinity" in placement and "clusterAffinities" in placement:
        out.append(_f("error", "R4", where + ".spec.placement",
                      "clusterAffinity and clusterAffinities are mutually exclusive"))

    # R11: integer weights
    rs = placement.get("replicaScheduling", {}) or {}
    wp = rs.get("weightPreference") or {}
    for i, w in enumerate(wp.get("staticWeightList") or []):
        if not isinstance(w.get("weight"), int):
            out.append(_f("error", "R11",
                          f"{where}.spec.placement.replicaScheduling.weightPreference.staticWeightList[{i}].weight",
                          f"weight must be an integer (got {w.get('weight')!r})"))

    # R8: warn about Overwrite
    if spec.get("conflictResolution") == "Overwrite":
        out.append(_f("warn", "R8", where + ".spec.conflictResolution",
                      "conflictResolution=Overwrite is dangerous; only use during migration"))

    # R14: deprecated fields
    if spec.get("association"):
        out.append(_f("warn", "R14", where + ".spec.association",
                      "field 'association' is deprecated in favor of 'propagateDeps'"))
    failover_pm = (((spec.get("failover") or {}).get("application") or {}).get("purgeMode"))
    if failover_pm in DEPRECATED_PURGE_MODES:
        out.append(_f("warn", "R14", where + ".spec.failover.application.purgeMode",
                      f"purgeMode={failover_pm!r} is deprecated; use 'Directly' or 'Gracefully'"))

    # R9: priority without preemption
    pri = spec.get("priority", 0) or 0
    if pri > 0 and spec.get("preemption", "Never") == "Never":
        out.append(_f("info", "R9", where + ".spec.preemption",
                      "priority is set but preemption=Never; existing claims will not be displaced"))

    return out


def _lint_resource_selector(sel: dict, where: str) -> list[dict]:
    out: list[dict] = []
    av = sel.get("apiVersion")
    if not av:
        out.append(_f("error", "R3", where + ".apiVersion", "apiVersion is required"))
    elif av != av.lower() and "/" in av and not re.match(r"^[a-z0-9.]+/v[a-z0-9]+$", av):
        # case-sensitivity heuristic: groups must be lowercase
        out.append(_f("error", "R3", where + ".apiVersion",
                      f"apiVersion {av!r} looks malformed (group must be lowercase, version like v1, v1alpha1, etc.)"))
    if not sel.get("kind"):
        out.append(_f("error", "STRUCT", where + ".kind", "kind is required"))
    # R12: labelSelector ignored when name is set
    if sel.get("name") and sel.get("labelSelector"):
        out.append(_f("warn", "R12", where,
                      "both name and labelSelector are set; labelSelector will be silently ignored"))
    return out


def _lint_override(spec: dict, where: str) -> list[dict]:
    out: list[dict] = []
    if spec.get("targetCluster") or spec.get("overriders"):
        out.append(_f("warn", "R14", where + ".spec.targetCluster",
                      "spec.targetCluster + spec.overriders is deprecated since v1.0; use spec.overrideRules"))
    rules = spec.get("overrideRules") or []
    for i, rule in enumerate(rules):
        rwhere = f"{where}.spec.overrideRules[{i}]"
        for j, p in enumerate((rule.get("overriders") or {}).get("plaintext") or []):
            path = p.get("path", "")
            err = _check_json_pointer(path)
            if err:
                out.append(_f("error", "R5", f"{rwhere}.overriders.plaintext[{j}].path", err))
    return out


def _check_json_pointer(path: str) -> str | None:
    if not isinstance(path, str) or not path:
        return "path is required"
    if path.startswith("$"):
        return f"path {path!r} is JSONPath; OverridePolicy requires JSON Pointer (RFC 6901, starts with '/')"
    if not path.startswith("/"):
        return f"path {path!r} must start with '/' (JSON Pointer per RFC 6901)"
    return None


def _f(severity: str, rule: str, path: str, message: str) -> dict:
    return {"severity": severity, "rule": rule, "path": path, "message": message}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("file", help="YAML file to audit (use '-' for stdin)")
    ap.add_argument("--format", choices=["json", "text"], default="json")
    args = ap.parse_args()

    src = sys.stdin.read() if args.file == "-" else open(args.file).read()
    try:
        docs = [d for d in yaml.safe_load_all(src) if d]
    except yaml.YAMLError as e:
        print(json.dumps({"error": f"yaml parse: {e}"}), file=sys.stderr)
        return 2

    findings: list[dict] = []
    for i, doc in enumerate(docs):
        if not isinstance(doc, dict):
            findings.append(_f("error", "STRUCT", f"docs[{i}]", "expected a mapping at top level"))
            continue
        findings.extend(lint_doc(doc, i))

    if args.format == "json":
        json.dump({"findings": findings, "summary": _summary(findings)}, sys.stdout, indent=2)
        sys.stdout.write("\n")
    else:
        if not findings:
            print("ok — no findings")
        for f in findings:
            print(f"[{f['severity'].upper():5}] {f['rule']:6} {f['path']}: {f['message']}")
    return 1 if any(f["severity"] == "error" for f in findings) else 0


def _summary(findings: list[dict]) -> dict:
    counts = {"error": 0, "warn": 0, "info": 0}
    for f in findings:
        counts[f["severity"]] = counts.get(f["severity"], 0) + 1
    return counts


if __name__ == "__main__":
    sys.exit(main())
