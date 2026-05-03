#!/usr/bin/env python3
# Copyright 2026 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
Karmada Policy Validator — Deterministic helper for validating
PropagationPolicy and OverridePolicy YAML against known schema rules.

Usage:
  python3 validate-policy.py <policy.yaml>
  python3 validate-policy.py --kind PropagationPolicy <policy.yaml>
"""

import argparse
import sys

try:
    import yaml
except ImportError:
    print("ERROR: Missing required Python package 'PyYAML' (module 'yaml'). "
          "Install it with: pip install PyYAML", file=sys.stderr)
    sys.exit(1)


def validate_propagation_policy(policy):
    """Validate a PropagationPolicy against schema rules. Returns list of (severity, message) tuples."""
    findings = []
    spec = policy.get("spec", {})
    kind = policy.get("kind", "PropagationPolicy")

    # Required: resourceSelectors with min 1 item for PropagationPolicy
    # OverridePolicy allows empty resourceSelectors to match all resources in scope
    selectors = spec.get("resourceSelectors", [])
    if not selectors:
        if kind in ("PropagationPolicy", "ClusterPropagationPolicy"):
            findings.append(("ERROR", "spec.resourceSelectors is required and must have at least 1 entry"))
        else:
            findings.append(("INFO", "spec.resourceSelectors is empty — OverridePolicy will match all resources in scope"))

    for i, sel in enumerate(selectors):
        if not sel.get("apiVersion"):
            findings.append(("ERROR", f"resourceSelectors[{i}].apiVersion is required"))
        if not sel.get("kind"):
            findings.append(("ERROR", f"resourceSelectors[{i}].kind is required"))
        if sel.get("name") and sel.get("labelSelector"):
            findings.append(("WARNING", f"resourceSelectors[{i}]: name is set, so labelSelector is ignored"))

    # Placement constraints
    placement = spec.get("placement", {})
    if placement.get("clusterAffinity") and placement.get("clusterAffinities"):
        findings.append(("ERROR", "placement.clusterAffinity and clusterAffinities are mutually exclusive"))

    # Spread constraints
    for i, sc in enumerate(placement.get("spreadConstraints", [])):
        if sc.get("spreadByField") and sc.get("spreadByLabel"):
            findings.append(("ERROR", f"spreadConstraints[{i}]: spreadByField and spreadByLabel are mutually exclusive"))

    # Suspension
    suspension = spec.get("suspension", {})
    if suspension:
        if suspension.get("dispatching") and suspension.get("dispatchingOnClusters"):
            findings.append(("ERROR", "suspension.dispatching and dispatchingOnClusters are mutually exclusive"))

    # Failover
    failover = spec.get("failover", {})
    if failover:
        if failover.get("application"):
            if spec.get("propagateDeps") is not True:
                findings.append(("WARNING", "Application failover configured but propagateDeps is not true. Dependencies won't be migrated during failover"))

    # OverridePolicy specific
    if kind in ("OverridePolicy", "ClusterOverridePolicy"):
        rules = spec.get("overrideRules", [])
        deprecated_target = spec.get("targetCluster")
        deprecated_overriders = spec.get("overriders")
        if (deprecated_target or deprecated_overriders) and not rules:
            findings.append(("WARNING", "Using deprecated targetCluster/overriders fields. Use overrideRules instead"))

        for i, rule in enumerate(rules):
            overriders = rule.get("overriders", {})
            for img in overriders.get("imageOverrider", []):
                if not img.get("predicate"):
                    findings.append(("INFO", f"overrideRules[{i}].imageOverrider: No predicate set — applies to ALL container images"))
            for pt in overriders.get("plaintext", []):
                if pt.get("path", "").startswith("spec."):
                    findings.append(("WARNING", f"overrideRules[{i}].plaintext.path should use JSON pointer syntax (e.g. /spec/replicas), got: {pt['path']}"))
            for field in overriders.get("fieldOverrider", []):
                fp = field.get("fieldPath", "")
                if fp and not fp.startswith("/"):
                    findings.append(("WARNING", f"overrideRules[{i}].fieldOverrider.fieldPath should use RFC 6901 syntax (e.g. /data/key), got: {fp}"))

    return findings


def main():
    parser = argparse.ArgumentParser(description="Validate Karmada policy YAML")
    parser.add_argument("file", help="Policy YAML file to validate")
    parser.add_argument("--kind", help="Expected policy kind (optional)")

    args = parser.parse_args()

    try:
        with open(args.file, "r") as f:
            policy = yaml.safe_load(f)
    except FileNotFoundError:
        print(f"ERROR: File not found: {args.file}")
        sys.exit(1)
    except yaml.YAMLError as e:
        print(f"ERROR: Invalid YAML: {e}")
        sys.exit(1)

    if not isinstance(policy, dict):
        print("ERROR: Policy must be a YAML object")
        sys.exit(1)

    yaml_kind = policy.get("kind", "")
    kind = args.kind or yaml_kind
    if args.kind and yaml_kind and args.kind != yaml_kind:
        print(f"ERROR: --kind flag ({args.kind}) does not match YAML kind ({yamlKind})")
        sys.exit(1)
    api_version = policy.get("apiVersion", "")

    if api_version != "policy.karmada.io/v1alpha1":
        print(f"WARNING: Expected apiVersion policy.karmada.io/v1alpha1, got {api_version}")

    if kind not in ("PropagationPolicy", "ClusterPropagationPolicy", "OverridePolicy", "ClusterOverridePolicy"):
        print(f"ERROR: Unknown kind '{kind}'. Expected one of: PropagationPolicy, ClusterPropagationPolicy, OverridePolicy, ClusterOverridePolicy")
        sys.exit(1)

    findings = validate_propagation_policy(policy)

    # Categorize
    errors = [m for s, m in findings if s == "ERROR"]
    warnings = [m for s, m in findings if s == "WARNING"]
    infos = [m for s, m in findings if s == "INFO"]

    print(f"\n=== Audit Report: {policy.get('metadata', {}).get('name', 'unknown')} ({kind}) ===")

    if errors:
        print(f"\n  Errors ({len(errors)}):")
        for e in errors:
            print(f"    [ERROR] {e}")
    if warnings:
        print(f"\n  Warnings ({len(warnings)}):")
        for w in warnings:
            print(f"    [WARN]  {w}")
    if infos:
        print(f"\n  Info ({len(infos)}):")
        for i in infos:
            print(f"    [INFO]  {i}")

    if not findings:
        print("\n  No issues found. Policy looks valid.")

    print()

    if errors:
        sys.exit(1)
    else:
        sys.exit(0)


if __name__ == "__main__":
    main()
