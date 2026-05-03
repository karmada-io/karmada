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
Karmada Policy Generator — Deterministic helper for generating
PropagationPolicy and OverridePolicy YAML from structured input.

Usage:
  python3 policy-generator.py propagation --name my-app --kind Deployment --api apps/v1 --clusters member1,member2
  python3 policy-generator.py override --name my-app --kind Deployment --api apps/v1 --cluster member2 --override-type image --overrides '[{"component":"Registry","operator":"replace","value":"registry.example.com"}]'
"""

import argparse
import json
import sys

try:
    import yaml
except ImportError:
    print("ERROR: Missing required Python package 'PyYAML' (module 'yaml'). "
          "Install it with: pip install PyYAML", file=sys.stderr)
    sys.exit(1)


API_VERSION = "policy.karmada.io/v1alpha1"


def build_propagation_policy(name, namespace, api_version, kind, resource_name,
                              cluster_names, exclude_clusters=None,
                              label_selector=None, field_selector=None,
                              replica_type="Divided", replica_pref="Weighted",
                              weights=None, propagate_deps=False,
                              conflict_resolution="Abort"):
    """Generate a PropagationPolicy from structured parameters."""
    policy = {
        "apiVersion": API_VERSION,
        "kind": "PropagationPolicy",
        "metadata": {
            "name": f"{name}-propagation",
        },
        "spec": {
            "resourceSelectors": [{
                "apiVersion": api_version,
                "kind": kind,
            }],
            "placement": {
                "clusterAffinity": {
                    "clusterNames": cluster_names,
                },
            },
        },
    }

    if namespace:
        policy["metadata"]["namespace"] = namespace

    if resource_name:
        policy["spec"]["resourceSelectors"][0]["name"] = resource_name

    if exclude_clusters:
        policy["spec"]["placement"]["clusterAffinity"]["exclude"] = exclude_clusters

    if label_selector:
        policy["spec"]["placement"]["clusterAffinity"]["labelSelector"] = label_selector

    if field_selector:
        policy["spec"]["placement"]["clusterAffinity"]["fieldSelector"] = field_selector

    if kind in ("Deployment", "StatefulSet", "ReplicaSet", "DaemonSet", "Job"):
        policy["spec"]["placement"]["replicaScheduling"] = {
            "replicaSchedulingType": replica_type,
            "replicaDivisionPreference": replica_pref,
        }
        if weights and replica_pref == "Weighted":
            policy["spec"]["placement"]["replicaScheduling"]["weightPreference"] = {
                "staticWeightList": [
                    {"targetCluster": {"clusterNames": [c]}, "weight": w}
                    for c, w in zip(cluster_names, weights)
                ]
            }

    if propagate_deps:
        policy["spec"]["propagateDeps"] = True

    if conflict_resolution != "Abort":
        policy["spec"]["conflictResolution"] = conflict_resolution

    return policy


def build_override_policy(name, namespace, api_version, kind, resource_name,
                           cluster_name, overrides, override_type="labels"):
    """Generate an OverridePolicy from structured parameters."""
    policy = {
        "apiVersion": API_VERSION,
        "kind": "OverridePolicy",
        "metadata": {
            "name": f"{name}-override",
        },
        "spec": {
            "resourceSelectors": [{
                "apiVersion": api_version,
                "kind": kind,
            }],
            "overrideRules": [{
                "targetCluster": {
                    "clusterNames": [cluster_name],
                },
                "overriders": {},
            }],
        },
    }

    if namespace:
        policy["metadata"]["namespace"] = namespace

    if resource_name:
        policy["spec"]["resourceSelectors"][0]["name"] = resource_name

    if override_type == "labels":
        policy["spec"]["overrideRules"][0]["overriders"]["labelsOverrider"] = overrides
    elif override_type == "annotations":
        policy["spec"]["overrideRules"][0]["overriders"]["annotationsOverrider"] = overrides
    elif override_type == "image":
        policy["spec"]["overrideRules"][0]["overriders"]["imageOverrider"] = overrides
    elif override_type == "plaintext":
        policy["spec"]["overrideRules"][0]["overriders"]["plaintext"] = overrides
    else:
        policy["spec"]["overrideRules"][0]["overriders"][f"{override_type}Overrider"] = overrides

    return policy


def main():
    parser = argparse.ArgumentParser(description="Generate Karmada policy YAML")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # Propagation subcommand
    prop = subparsers.add_parser("propagation", help="Generate PropagationPolicy")
    prop.add_argument("--name", required=True, help="Policy name prefix")
    prop.add_argument("--namespace", help="Policy namespace")
    prop.add_argument("--api", required=True, dest="api_version", help="Resource apiVersion (e.g. apps/v1)")
    prop.add_argument("--kind", required=True, help="Resource Kind (e.g. Deployment)")
    prop.add_argument("--resource-name", help="Specific resource name")
    prop.add_argument("--clusters", required=True, help="Comma-separated cluster names")
    prop.add_argument("--exclude", help="Comma-separated cluster names to exclude")
    prop.add_argument("--weights", help="Comma-separated integer weights")
    prop.add_argument("--replica-type", default="Divided", choices=["Duplicated", "Divided"])
    prop.add_argument("--replica-pref", default="Weighted", choices=["Aggregated", "Weighted"])
    prop.add_argument("--propagate-deps", action="store_true")
    prop.add_argument("--conflict", default="Abort", choices=["Abort", "Overwrite"])

    # Override subcommand
    over = subparsers.add_parser("override", help="Generate OverridePolicy")
    over.add_argument("--name", required=True, help="Policy name prefix")
    over.add_argument("--namespace", help="Policy namespace")
    over.add_argument("--api", required=True, dest="api_version", help="Resource apiVersion")
    over.add_argument("--kind", required=True, help="Resource Kind")
    over.add_argument("--resource-name", help="Specific resource name")
    over.add_argument("--cluster", required=True, help="Target cluster name")
    over.add_argument("--override-type", default="labels", choices=["labels", "annotations", "image", "command", "args", "plaintext"])
    over.add_argument("--overrides", help="JSON string of overrides (key-value for labels, list for command/args)")

    args = parser.parse_args()

    if args.command == "propagation":
        cluster_names = [c.strip() for c in args.clusters.split(",")]
        exclude = [c.strip() for c in args.exclude.split(",")] if args.exclude else None
        weights = [int(w.strip()) for w in args.weights.split(",")] if args.weights else None

        if weights and len(weights) != len(cluster_names):
            parser.error(f"--weights count ({len(weights)}) must match --clusters count ({len(cluster_names)})")
        if weights and any(w < 1 for w in weights):
            parser.error("all weights must be >= 1")

        policy = build_propagation_policy(
            name=args.name,
            namespace=args.namespace,
            api_version=args.api_version,
            kind=args.kind,
            resource_name=args.resource_name,
            cluster_names=cluster_names,
            exclude_clusters=exclude,
            replica_type=args.replica_type,
            replica_pref=args.replica_pref,
            weights=weights,
            propagate_deps=args.propagate_deps,
            conflict_resolution=args.conflict,
        )

    elif args.command == "override":
        overrides = json.loads(args.overrides) if args.overrides else []

        policy = build_override_policy(
            name=args.name,
            namespace=args.namespace,
            api_version=args.api_version,
            kind=args.kind,
            resource_name=args.resource_name,
            cluster_name=args.cluster,
            overrides=overrides,
            override_type=args.override_type,
        )

    print(yaml.dump(policy, default_flow_style=False, sort_keys=False))


if __name__ == "__main__":
    main()
