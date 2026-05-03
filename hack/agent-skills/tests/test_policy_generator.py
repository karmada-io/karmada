"""Unit tests for the deterministic policy generator.

Run from repo root:
    python3 -m unittest hack/agent-skills/tests/test_policy_generator.py
"""
from __future__ import annotations

import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "scripts"))

import importlib  # noqa: E402

policy_generator = importlib.import_module("policy-generator")  # type: ignore[assignment]
build = policy_generator.build
parse_args = policy_generator.parse_args
PolicyError = policy_generator.PolicyError


def args(*tokens: str):
    return parse_args(list(tokens))


class PropagationPolicyTests(unittest.TestCase):
    def test_minimal_clusters(self):
        p = build(args(
            "--kind", "PropagationPolicy",
            "--name", "nginx",
            "--namespace", "default",
            "--target-kind", "Deployment",
            "--target-name", "nginx",
            "--clusters", "member-de,member-fr",
        ))
        self.assertEqual(p["apiVersion"], "policy.karmada.io/v1alpha1")
        self.assertEqual(p["kind"], "PropagationPolicy")
        self.assertEqual(p["metadata"]["namespace"], "default")
        self.assertEqual(
            p["spec"]["placement"]["clusterAffinity"]["clusterNames"],
            ["member-de", "member-fr"],
        )

    def test_replica_scheduling_validation(self):
        with self.assertRaises(PolicyError):
            build(args(
                "--kind", "PropagationPolicy",
                "--name", "x", "--namespace", "default",
                "--target-kind", "Deployment", "--target-name", "x",
                "--clusters", "a",
                "--replica-scheduling", "Sharded",  # invalid
            ))

    def test_spread_by_field_only(self):
        p = build(args(
            "--kind", "PropagationPolicy",
            "--name", "spread", "--namespace", "default",
            "--target-kind", "Deployment", "--target-name", "x",
            "--spread-by-field", "region",
            "--max-groups", "3",
        ))
        sc = p["spec"]["placement"]["spreadConstraints"][0]
        self.assertEqual(sc["spreadByField"], "region")
        self.assertEqual(sc["maxGroups"], 3)

    def test_requires_clusters_or_spread(self):
        with self.assertRaises(PolicyError):
            build(args(
                "--kind", "PropagationPolicy",
                "--name", "x", "--namespace", "default",
                "--target-kind", "Deployment", "--target-name", "x",
            ))

    def test_cluster_propagation_policy_no_namespace(self):
        p = build(args(
            "--kind", "ClusterPropagationPolicy",
            "--name", "global",
            "--target-kind", "ClusterRole", "--target-name", "edit",
            "--target-api-version", "rbac.authorization.k8s.io/v1",
            "--clusters", "member-1",
        ))
        self.assertNotIn("namespace", p["metadata"])


class OverridePolicyTests(unittest.TestCase):
    def test_image_registry_override(self):
        p = build(args(
            "--kind", "OverridePolicy",
            "--name", "cn-mirror", "--namespace", "default",
            "--target-kind", "Deployment", "--target-name", "nginx",
            "--clusters", "member-cn",
            "--override-image-registry", "registry.cn-hangzhou.aliyuncs.com/myorg",
        ))
        rule = p["spec"]["overrideRules"][0]
        self.assertEqual(rule["targetCluster"]["clusterNames"], ["member-cn"])
        img = rule["overriders"]["imageOverrider"][0]
        self.assertEqual(img["component"], "Registry")
        self.assertEqual(img["operator"], "replace")
        self.assertEqual(img["value"], "registry.cn-hangzhou.aliyuncs.com/myorg")

    def test_replicas_override_uses_jsonpointer(self):
        p = build(args(
            "--kind", "OverridePolicy",
            "--name", "scale", "--namespace", "default",
            "--target-kind", "Deployment", "--target-name", "nginx",
            "--clusters", "member-1",
            "--override-replicas", "5",
        ))
        plain = p["spec"]["overrideRules"][0]["overriders"]["plaintext"][0]
        self.assertEqual(plain["path"], "/spec/replicas")
        self.assertEqual(plain["value"], 5)

    def test_override_requires_some_change(self):
        with self.assertRaises(PolicyError):
            build(args(
                "--kind", "OverridePolicy",
                "--name", "x", "--namespace", "default",
                "--target-kind", "Deployment", "--target-name", "x",
                "--clusters", "member-1",
            ))


class KindValidationTests(unittest.TestCase):
    def test_unknown_kind_rejected(self):
        with self.assertRaises(PolicyError):
            build(args(
                "--kind", "WorkloadPolicy",
                "--name", "x", "--namespace", "default",
                "--target-kind", "Deployment", "--target-name", "x",
                "--clusters", "a",
            ))


if __name__ == "__main__":
    unittest.main()
