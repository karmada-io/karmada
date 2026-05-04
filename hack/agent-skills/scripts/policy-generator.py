#!/usr/bin/env python3
"""Deterministic Karmada policy YAML generator.

Used by the `karmada-create-policy` skill so AI agents never free-hand
PropagationPolicy / OverridePolicy YAML. Input is validated against a
narrow allow-list; unknown values are rejected, not silently coerced.

Examples:
    policy-generator.py --kind PropagationPolicy --name nginx-eu \\
        --namespace default --target-kind Deployment --target-name nginx \\
        --clusters member-de,member-fr --replica-scheduling Divided

    policy-generator.py --kind PropagationPolicy --name nginx-failover \\
        --namespace default --target-kind Deployment --target-name nginx \\
        --affinity-group primary=member-prod-east \\
        --affinity-group backup=member-prod-west \\
        --failover-toleration-seconds 120 --failover-purge-mode Graciously

    policy-generator.py --kind OverridePolicy --name cn-mirror \\
        --namespace default --target-kind Deployment --target-name nginx \\
        --override-image-registry registry.cn-hangzhou.aliyuncs.com/myorg \\
        --clusters member-cn

The script intentionally does NOT talk to a Kubernetes API; it is a pure
schema-aware text generator suitable for offline / agent use.
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any

try:
    import yaml  # PyYAML
except ImportError:  # pragma: no cover - import-time guard
    sys.stderr.write("error: PyYAML is required (pip install pyyaml)\n")
    raise

API_VERSION = "policy.karmada.io/v1alpha1"

PROPAGATION_KINDS = {"PropagationPolicy", "ClusterPropagationPolicy"}
OVERRIDE_KINDS = {"OverridePolicy", "ClusterOverridePolicy"}
ALL_KINDS = PROPAGATION_KINDS | OVERRIDE_KINDS

REPLICA_SCHEDULING_TYPES = {"Duplicated", "Divided"}
SPREAD_FIELDS = {"cluster", "region", "zone", "provider"}
# Directly/Gracefully are the current preferred values.
# Immediately/Graciously are deprecated aliases (still accepted by the API).
# Source: +kubebuilder:validation:Enum=Directly;Gracefully;Never;Immediately;Graciously
PURGE_MODES = {"Directly", "Gracefully", "Never", "Immediately", "Graciously"}
CONFLICT_RESOLUTIONS = {"Abort", "Overwrite"}


class PolicyError(ValueError):
    """Surface validation failures with a clear, agent-readable message."""


def _split_csv(value: str | None) -> list[str]:
    if not value:
        return []
    return [v.strip() for v in value.split(",") if v.strip()]


def _parse_affinity_group(spec: str) -> dict[str, Any]:
    """Parse 'name=cluster1,cluster2' into a clusterAffinities entry."""
    if "=" not in spec:
        raise PolicyError(
            f"--affinity-group must be 'name=cluster1[,cluster2]', got {spec!r}"
        )
    name, _, csv = spec.partition("=")
    name = name.strip()
    clusters = _split_csv(csv)
    if not name or not clusters:
        raise PolicyError(f"--affinity-group has empty name or clusters: {spec!r}")
    return {"affinityName": name, "clusterNames": clusters}


def _build_resource_selector(args: argparse.Namespace) -> dict[str, Any]:
    if not args.target_kind or not args.target_name:
        raise PolicyError("--target-kind and --target-name are required")
    return {
        "apiVersion": args.target_api_version,
        "kind": args.target_kind,
        "name": args.target_name,
    }


def _build_failover(args: argparse.Namespace) -> dict[str, Any] | None:
    if args.failover_toleration_seconds is None and not args.failover_purge_mode:
        return None
    app: dict[str, Any] = {}
    if args.failover_toleration_seconds is not None:
        app["decisionConditions"] = {
            "tolerationSeconds": args.failover_toleration_seconds
        }
    if args.failover_purge_mode:
        if args.failover_purge_mode not in PURGE_MODES:
            raise PolicyError(
                f"--failover-purge-mode must be one of {sorted(PURGE_MODES)}"
            )
        app["purgeMode"] = args.failover_purge_mode
    if args.failover_grace_period_seconds is not None:
        app["gracePeriodSeconds"] = args.failover_grace_period_seconds
    if "decisionConditions" not in app:
        # decisionConditions is required by the API when application is set.
        raise PolicyError(
            "failover.application requires --failover-toleration-seconds"
        )
    return {"application": app}


def build_propagation_policy(args: argparse.Namespace) -> dict[str, Any]:
    clusters = _split_csv(args.clusters)
    affinity_groups = [_parse_affinity_group(s) for s in args.affinity_group or []]

    if not clusters and not args.spread_by_field and not affinity_groups:
        raise PolicyError(
            "PropagationPolicy needs --clusters, --spread-by-field, or "
            "--affinity-group"
        )
    if clusters and affinity_groups:
        raise PolicyError(
            "Pick one: --clusters (single affinity) or --affinity-group "
            "(ordered failover list). They are mutually exclusive."
        )

    placement: dict[str, Any] = {}
    if affinity_groups:
        placement["clusterAffinities"] = affinity_groups
    elif clusters:
        placement["clusterAffinity"] = {"clusterNames": clusters}

    if args.spread_by_field:
        if args.spread_by_field not in SPREAD_FIELDS:
            raise PolicyError(
                f"--spread-by-field must be one of {sorted(SPREAD_FIELDS)}"
            )
        placement["spreadConstraints"] = [
            {
                "spreadByField": args.spread_by_field,
                "maxGroups": args.max_groups,
                "minGroups": args.min_groups,
            }
        ]

    if args.replica_scheduling:
        if args.replica_scheduling not in REPLICA_SCHEDULING_TYPES:
            raise PolicyError(
                f"--replica-scheduling must be one of "
                f"{sorted(REPLICA_SCHEDULING_TYPES)}"
            )
        placement["replicaScheduling"] = {
            "replicaSchedulingType": args.replica_scheduling
        }

    spec: dict[str, Any] = {
        "resourceSelectors": [_build_resource_selector(args)],
        "placement": placement,
    }
    if args.propagate_deps:
        spec["propagateDeps"] = True
    if args.conflict_resolution:
        if args.conflict_resolution not in CONFLICT_RESOLUTIONS:
            raise PolicyError(
                f"--conflict-resolution must be one of "
                f"{sorted(CONFLICT_RESOLUTIONS)}"
            )
        spec["conflictResolution"] = args.conflict_resolution
    failover = _build_failover(args)
    if failover is not None:
        spec["failover"] = failover

    metadata: dict[str, Any] = {"name": args.name}
    if args.kind == "PropagationPolicy":
        if not args.namespace:
            raise PolicyError("--namespace required for PropagationPolicy")
        metadata["namespace"] = args.namespace
    elif args.namespace:
        raise PolicyError(
            "ClusterPropagationPolicy is cluster-scoped; do not pass --namespace"
        )

    return {
        "apiVersion": API_VERSION,
        "kind": args.kind,
        "metadata": metadata,
        "spec": spec,
    }


def build_override_policy(args: argparse.Namespace) -> dict[str, Any]:
    clusters = _split_csv(args.clusters)
    if not clusters:
        raise PolicyError("OverridePolicy needs --clusters (target cluster names)")

    overriders: dict[str, Any] = {}
    if args.override_image_registry:
        entry: dict[str, Any] = {
            "component": "Registry",
            "operator": "replace",
            "value": args.override_image_registry,
        }
        if args.override_image_predicate:
            entry["predicate"] = {"path": args.override_image_predicate}
        overriders["imageOverrider"] = [entry]

    if args.override_replicas is not None:
        overriders.setdefault("plaintext", []).append(
            {
                "path": "/spec/replicas",
                "operator": "replace",
                "value": args.override_replicas,
            }
        )

    if args.override_command:
        container, _, csv = args.override_command.partition("=")
        if not container or not csv:
            raise PolicyError(
                "--override-command must be 'container=arg1,arg2'"
            )
        overriders["commandOverrider"] = [
            {
                "containerName": container.strip(),
                "operator": "replace",
                "value": _split_csv(csv),
            }
        ]

    if not overriders:
        raise PolicyError(
            "Provide at least one override: --override-image-registry, "
            "--override-replicas, or --override-command"
        )

    spec = {
        "resourceSelectors": [_build_resource_selector(args)],
        "overrideRules": [
            {
                "targetCluster": {"clusterNames": clusters},
                "overriders": overriders,
            }
        ],
    }

    metadata: dict[str, Any] = {"name": args.name}
    if args.kind == "OverridePolicy":
        if not args.namespace:
            raise PolicyError("--namespace required for OverridePolicy")
        metadata["namespace"] = args.namespace
    elif args.namespace:
        raise PolicyError(
            "ClusterOverridePolicy is cluster-scoped; do not pass --namespace"
        )

    return {
        "apiVersion": API_VERSION,
        "kind": args.kind,
        "metadata": metadata,
        "spec": spec,
    }


def build(args: argparse.Namespace) -> dict[str, Any]:
    if args.kind not in ALL_KINDS:
        raise PolicyError(
            f"--kind must be one of {sorted(ALL_KINDS)}, got {args.kind!r}"
        )
    if args.kind in PROPAGATION_KINDS:
        return build_propagation_policy(args)
    return build_override_policy(args)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="policy-generator.py",
        description="Generate Karmada PropagationPolicy / OverridePolicy YAML.",
    )
    p.add_argument("--kind", required=True, help=f"one of {sorted(ALL_KINDS)}")
    p.add_argument("--name", required=True)
    p.add_argument("--namespace")

    p.add_argument("--target-api-version", default="apps/v1")
    p.add_argument("--target-kind")
    p.add_argument("--target-name")

    # Placement options (mutually exclusive forms enforced in build_*).
    p.add_argument("--clusters", help="comma-separated cluster names (single affinity)")
    p.add_argument("--affinity-group", action="append",
                   help="ordered failover group: 'name=cluster1,cluster2' "
                        "(repeat for multiple groups)")
    p.add_argument("--spread-by-field",
                   help=f"one of {sorted(SPREAD_FIELDS)}")
    p.add_argument("--max-groups", type=int, default=3)
    p.add_argument("--min-groups", type=int, default=1)
    p.add_argument("--replica-scheduling",
                   help=f"one of {sorted(REPLICA_SCHEDULING_TYPES)}")
    p.add_argument("--propagate-deps", action="store_true")
    p.add_argument("--conflict-resolution",
                   help=f"one of {sorted(CONFLICT_RESOLUTIONS)}")

    # Failover.
    p.add_argument("--failover-toleration-seconds", type=int)
    p.add_argument("--failover-purge-mode",
                   help=f"one of {sorted(PURGE_MODES)}")
    p.add_argument("--failover-grace-period-seconds", type=int)

    # Override options.
    p.add_argument("--override-image-registry",
                   help="replace container image registry on target clusters")
    p.add_argument("--override-image-predicate",
                   help="JSON Pointer to image string for image override")
    p.add_argument("--override-replicas", type=int,
                   help="replace /spec/replicas on target clusters")
    p.add_argument("--override-command",
                   help="replace container command: 'containerName=arg1,arg2'")

    p.add_argument("--output", help="write to this file instead of stdout")
    return p.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        policy = build(args)
    except PolicyError as e:
        sys.stderr.write(f"error: {e}\n")
        return 2

    rendered = yaml.safe_dump(policy, sort_keys=False)
    if args.output:
        Path(args.output).write_text(rendered, encoding="utf-8")
    else:
        sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
