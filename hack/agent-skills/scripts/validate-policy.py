#!/usr/bin/env python3
"""Lint-style validator for Karmada policy YAML.

Powers the `karmada-audit-policy` skill. Loads one or more YAML files
and runs a set of *correctness* and *idiom* checks that catch the
mistakes AI agents most commonly make. Each finding has a stable rule
ID so audit reports are diffable.

Exit codes:
    0  no findings
    1  findings emitted (errors and/or warnings)
    2  invalid usage / unparseable input

Rule severities:
    error    — schema- or behavior-breaking; will not work in production
    warning  — works but contradicts a documented idiom
    info     — informational; an opportunity to tighten the policy

Usage:
    validate-policy.py path/to/policy.yaml [more.yaml ...]
    validate-policy.py --format json policy.yaml
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from collections.abc import Iterable
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    sys.stderr.write("error: PyYAML is required (pip install pyyaml)\n")
    raise

JSON_POINTER_RE = re.compile(r"^/[^\s]*$|^$")
DOTTED_PATH_RE = re.compile(r"^[a-zA-Z_][\w\.\[\]]*$")
KNOWN_KINDS = {
    "PropagationPolicy",
    "ClusterPropagationPolicy",
    "OverridePolicy",
    "ClusterOverridePolicy",
}
NAMESPACED_KINDS = {"PropagationPolicy", "OverridePolicy"}
PROPAGATION_KINDS = {"PropagationPolicy", "ClusterPropagationPolicy"}
OVERRIDE_KINDS = {"OverridePolicy", "ClusterOverridePolicy"}
PURGE_MODES = {"Immediately", "Graciously", "Never", "Directly", "Gracefully"}
REPLICA_SCHEDULING_TYPES = {"Duplicated", "Divided"}
SPREAD_FIELDS = {"cluster", "region", "zone", "provider"}
OVERRIDER_OPS = {"add", "remove", "replace"}
IMAGE_COMPONENTS = {"Registry", "Repository", "Tag"}


@dataclass
class Finding:
    rule: str
    severity: str  # error | warning | info
    path: str
    message: str
    file: str = ""

    def format(self) -> str:
        prefix = f"{self.file}: " if self.file else ""
        sev = self.severity.upper()
        return f"{prefix}[{sev}] {self.rule} at {self.path}: {self.message}"


def _walk(doc: Any, *, source: str) -> Iterable[Finding]:
    if not isinstance(doc, dict):
        yield Finding(
            rule="KMD000", severity="error", path="$",
            message="document is not a YAML mapping", file=source,
        )
        return

    api_version = doc.get("apiVersion")
    kind = doc.get("kind")

    if api_version != "policy.karmada.io/v1alpha1":
        yield Finding(
            rule="KMD001", severity="error", path="$.apiVersion",
            message=f"expected 'policy.karmada.io/v1alpha1', got {api_version!r}",
            file=source,
        )

    if kind not in KNOWN_KINDS:
        yield Finding(
            rule="KMD002", severity="error", path="$.kind",
            message=f"unknown kind {kind!r}; expected one of {sorted(KNOWN_KINDS)}",
            file=source,
        )
        return

    metadata = doc.get("metadata") or {}
    spec = doc.get("spec") or {}

    yield from _check_metadata(metadata, kind, source)
    yield from _check_resource_selectors(spec.get("resourceSelectors"), source)

    if kind in PROPAGATION_KINDS:
        yield from _check_propagation_spec(spec, source)
    else:
        yield from _check_override_spec(spec, source)


def _check_metadata(metadata: dict, kind: str, source: str) -> Iterable[Finding]:
    if not metadata.get("name"):
        yield Finding(
            rule="KMD010", severity="error", path="$.metadata.name",
            message="name is required", file=source,
        )
    has_ns = "namespace" in metadata
    if kind in NAMESPACED_KINDS and not has_ns:
        yield Finding(
            rule="KMD011", severity="error", path="$.metadata.namespace",
            message=f"{kind} is namespaced; .metadata.namespace is required",
            file=source,
        )
    if kind not in NAMESPACED_KINDS and has_ns:
        yield Finding(
            rule="KMD012", severity="error", path="$.metadata.namespace",
            message=f"{kind} is cluster-scoped; remove .metadata.namespace",
            file=source,
        )


def _check_resource_selectors(rs: Any, source: str) -> Iterable[Finding]:
    if not rs:
        yield Finding(
            rule="KMD020", severity="error", path="$.spec.resourceSelectors",
            message="at least one resourceSelector is required", file=source,
        )
        return
    if not isinstance(rs, list):
        yield Finding(
            rule="KMD021", severity="error", path="$.spec.resourceSelectors",
            message="must be a list", file=source,
        )
        return
    for i, sel in enumerate(rs):
        base = f"$.spec.resourceSelectors[{i}]"
        if not isinstance(sel, dict):
            yield Finding(rule="KMD022", severity="error", path=base,
                          message="must be a mapping", file=source)
            continue
        if not sel.get("apiVersion"):
            yield Finding(rule="KMD023", severity="error",
                          path=f"{base}.apiVersion",
                          message="apiVersion is required", file=source)
        if not sel.get("kind"):
            yield Finding(rule="KMD024", severity="error",
                          path=f"{base}.kind",
                          message="kind is required", file=source)
        if sel.get("kind") == "Deployment" and sel.get("apiVersion") == "v1":
            yield Finding(rule="KMD025", severity="error",
                          path=f"{base}.apiVersion",
                          message="Deployment requires apiVersion 'apps/v1', not 'v1'",
                          file=source)


def _check_propagation_spec(spec: dict, source: str) -> Iterable[Finding]:
    placement = spec.get("placement") or {}
    has_singular = bool(placement.get("clusterAffinity"))
    has_plural = bool(placement.get("clusterAffinities"))

    if has_singular and has_plural:
        yield Finding(
            rule="KMD100", severity="error",
            path="$.spec.placement",
            message="clusterAffinity (singular) and clusterAffinities (plural) "
                    "are mutually exclusive — pick one",
            file=source,
        )

    if not has_singular and not has_plural \
            and not placement.get("spreadConstraints"):
        yield Finding(
            rule="KMD101", severity="warning",
            path="$.spec.placement",
            message="placement has no clusterAffinity/clusterAffinities/"
                    "spreadConstraints — policy will match no clusters",
            file=source,
        )

    rs = placement.get("replicaScheduling")
    if rs:
        rst = rs.get("replicaSchedulingType")
        if rst not in REPLICA_SCHEDULING_TYPES:
            yield Finding(
                rule="KMD110", severity="error",
                path="$.spec.placement.replicaScheduling.replicaSchedulingType",
                message=(
                    f"must be one of {sorted(REPLICA_SCHEDULING_TYPES)}, "
                    f"got {rst!r}"
                ),
                file=source,
            )

    for i, sc in enumerate(placement.get("spreadConstraints") or []):
        base = f"$.spec.placement.spreadConstraints[{i}]"
        if not isinstance(sc, dict):
            continue
        if sc.get("spreadByField") and sc.get("spreadByLabel"):
            yield Finding(
                rule="KMD120", severity="error", path=base,
                message="spreadByField and spreadByLabel are mutually exclusive "
                        "in a single constraint",
                file=source,
            )
        if sc.get("spreadByField") and sc["spreadByField"] not in SPREAD_FIELDS:
            yield Finding(
                rule="KMD121", severity="error",
                path=f"{base}.spreadByField",
                message=f"must be one of {sorted(SPREAD_FIELDS)}",
                file=source,
            )
        mn = sc.get("minGroups")
        mx = sc.get("maxGroups")
        if isinstance(mn, int) and isinstance(mx, int) and mn > mx:
            yield Finding(
                rule="KMD122", severity="error", path=base,
                message=f"minGroups ({mn}) > maxGroups ({mx})",
                file=source,
            )

    failover = spec.get("failover")
    if failover:
        app = failover.get("application")
        if app:
            pm = app.get("purgeMode")
            if pm and pm not in PURGE_MODES:
                yield Finding(
                    rule="KMD130", severity="error",
                    path="$.spec.failover.application.purgeMode",
                    message=f"must be one of {sorted(PURGE_MODES)}, got {pm!r}",
                    file=source,
                )
            dc = app.get("decisionConditions")
            if not dc or "tolerationSeconds" not in (dc or {}):
                yield Finding(
                    rule="KMD131", severity="error",
                    path="$.spec.failover.application.decisionConditions",
                    message=(
                        "application failover requires "
                        "decisionConditions.tolerationSeconds"
                    ),
                    file=source,
                )

    cr = spec.get("conflictResolution")
    if cr and cr not in {"Abort", "Overwrite"}:
        yield Finding(
            rule="KMD140", severity="error",
            path="$.spec.conflictResolution",
            message=f"must be 'Abort' or 'Overwrite', got {cr!r}",
            file=source,
        )


def _check_override_spec(spec: dict, source: str) -> Iterable[Finding]:
    rules = spec.get("overrideRules")
    if not rules:
        yield Finding(
            rule="KMD200", severity="error", path="$.spec.overrideRules",
            message="at least one overrideRule is required", file=source,
        )
        return
    for i, rule in enumerate(rules):
        base = f"$.spec.overrideRules[{i}]"
        if not isinstance(rule, dict):
            continue
        overriders = rule.get("overriders") or {}
        if not overriders:
            yield Finding(
                rule="KMD201", severity="error",
                path=f"{base}.overriders",
                message="overriders block must contain at least one overrider",
                file=source,
            )
        # Common typo: imageOverride (no -r) instead of imageOverrider.
        if "imageOverride" in overriders and "imageOverrider" not in overriders:
            yield Finding(
                rule="KMD210", severity="error",
                path=f"{base}.overriders.imageOverride",
                message=(
                    "field is named 'imageOverrider' (with -r), "
                    "not 'imageOverride'"
                ),
                file=source,
            )
        for j, p in enumerate(overriders.get("plaintext") or []):
            ppath = f"{base}.overriders.plaintext[{j}]"
            path = p.get("path")
            if path is None:
                yield Finding(rule="KMD220", severity="error",
                              path=f"{ppath}.path",
                              message="path is required", file=source)
            elif isinstance(path, str) and not JSON_POINTER_RE.match(path):
                hint = ""
                if DOTTED_PATH_RE.match(path):
                    pointer = "/" + (
                        path.replace(".", "/").replace("[", "/").replace("]", "")
                    )
                    hint = (
                        " — looks like a JSONPath; rewrite as JSON Pointer "
                        f"(e.g., {pointer})"
                    )
                yield Finding(
                    rule="KMD221", severity="error", path=f"{ppath}.path",
                    message=f"path must be a JSON Pointer (RFC 6901){hint}",
                    file=source,
                )
            op = p.get("operator")
            if op not in OVERRIDER_OPS:
                yield Finding(
                    rule="KMD222", severity="error", path=f"{ppath}.operator",
                    message=f"must be one of {sorted(OVERRIDER_OPS)}, got {op!r}",
                    file=source,
                )
        for j, im in enumerate(overriders.get("imageOverrider") or []):
            ipath = f"{base}.overriders.imageOverrider[{j}]"
            comp = im.get("component")
            if comp not in IMAGE_COMPONENTS:
                yield Finding(
                    rule="KMD230", severity="error", path=f"{ipath}.component",
                    message=f"must be one of {sorted(IMAGE_COMPONENTS)}, got {comp!r}",
                    file=source,
                )
            op = im.get("operator")
            if op not in OVERRIDER_OPS:
                yield Finding(
                    rule="KMD231", severity="error", path=f"{ipath}.operator",
                    message=f"must be one of {sorted(OVERRIDER_OPS)}, got {op!r}",
                    file=source,
                )


def validate_doc(doc: Any, source: str = "") -> list[Finding]:
    return list(_walk(doc, source=source))


def validate_file(path: str) -> list[Finding]:
    text = Path(path).read_text(encoding="utf-8")
    findings: list[Finding] = []
    try:
        docs = list(yaml.safe_load_all(text))
    except yaml.YAMLError as e:
        return [Finding(rule="KMD000", severity="error", path="$",
                        message=f"YAML parse error: {e}", file=path)]
    for doc in docs:
        if doc is None:
            continue
        findings.extend(validate_doc(doc, source=path))
    return findings


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="Lint Karmada policy YAML.")
    p.add_argument("paths", nargs="+")
    p.add_argument("--format", choices=("text", "json"), default="text")
    p.add_argument("--warnings-as-errors", action="store_true")
    args = p.parse_args(argv)

    all_findings: list[Finding] = []
    for path in args.paths:
        all_findings.extend(validate_file(path))

    if args.format == "json":
        sys.stdout.write(json.dumps([asdict(f) for f in all_findings], indent=2))
        sys.stdout.write("\n")
    else:
        for f in all_findings:
            sys.stdout.write(f.format() + "\n")
        if not all_findings:
            sys.stdout.write("ok: no findings\n")

    sevs = {f.severity for f in all_findings}
    if "error" in sevs:
        return 1
    if args.warnings_as_errors and "warning" in sevs:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
