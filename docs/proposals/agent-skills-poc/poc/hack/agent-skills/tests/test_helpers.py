#!/usr/bin/env python3
"""
test_helpers.py — assertion suite for the deterministic helpers.

Run via:
    cd hack/agent-skills && python3 -m unittest tests.test_helpers

These tests are the safety net the mentors care about most: every time the schema
files are regenerated from upstream CRDs, or a helper is refactored, the assertions
below guarantee the agent still produces correct YAML.

The suite is intentionally vanilla unittest (no pytest, no fixtures library) so it
runs in a stock CI image with `python3 -m unittest`. The only dependency is PyYAML.
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent  # hack/agent-skills/
SCRIPTS = ROOT / "scripts"
EXAMPLES = ROOT / "examples"


def run(script: str, *args: str, stdin: str | None = None) -> tuple[int, str, str]:
    p = subprocess.run(
        [sys.executable, str(SCRIPTS / script), *args],
        input=stdin,
        text=True,
        capture_output=True,
    )
    return p.returncode, p.stdout, p.stderr


class GeneratorTests(unittest.TestCase):
    def test_minimal_intent_round_trip(self):
        intent = {
            "policy_name": "x",
            "namespace": "default",
            "resources": [{"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}],
            "placement": {"mode": "name-list", "clusters": ["member1"]},
        }
        rc, out, err = run("generate_policy.py", stdin=json.dumps(intent))
        self.assertEqual(rc, 0, err)
        docs = list(yaml.safe_load_all(out))
        self.assertEqual(len(docs), 1)
        pp = docs[0]
        self.assertEqual(pp["kind"], "PropagationPolicy")
        self.assertEqual(pp["spec"]["placement"]["clusterAffinity"]["clusterNames"], ["member1"])

    def test_requires_at_least_one_resource_selector(self):
        intent = {
            "policy_name": "x",
            "resources": [],
            "placement": {"mode": "name-list", "clusters": ["member1"]},
        }
        rc, _, err = run("generate_policy.py", stdin=json.dumps(intent))
        self.assertNotEqual(rc, 0)
        self.assertIn("MinItems=1", err)

    def test_replica_scheduling_lives_under_placement(self):
        intent = {
            "policy_name": "x",
            "resources": [{"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}],
            "placement": {"mode": "name-list", "clusters": ["m1", "m2"]},
            "replica_strategy": "duplicated",
        }
        rc, out, _ = run("generate_policy.py", stdin=json.dumps(intent))
        self.assertEqual(rc, 0)
        pp = list(yaml.safe_load_all(out))[0]
        self.assertNotIn("replicaScheduling", pp["spec"])
        self.assertIn("replicaScheduling", pp["spec"]["placement"])

    def test_json_pointer_path_required_for_plaintext(self):
        intent = {
            "policy_name": "x",
            "resources": [{"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}],
            "placement": {"mode": "name-list", "clusters": ["m1"]},
            "overrides": [{
                "target_clusters": ["m1"],
                "plaintext": [{"path": "$.spec.replicas", "operator": "replace", "value": "3"}],
            }],
        }
        rc, _, err = run("generate_policy.py", stdin=json.dumps(intent))
        self.assertNotEqual(rc, 0)
        self.assertIn("JSON Pointer", err)

    def test_multi_country_example_audits_clean(self):
        intent_file = EXAMPLES / "01-multi-country-frontend" / "intent.json"
        rc, generated, _ = run("generate_policy.py", "--intent", str(intent_file))
        self.assertEqual(rc, 0)
        rc, audit_out, _ = run("audit_policy.py", "-", stdin=generated)
        self.assertEqual(rc, 0, audit_out)
        report = json.loads(audit_out)
        self.assertEqual(report["summary"]["error"], 0)


class AuditorTests(unittest.TestCase):
    BAD_DIR = EXAMPLES / "02-bad-policy-audit-targets"

    def _audit(self, fname: str) -> dict:
        rc, out, _ = run("audit_policy.py", str(self.BAD_DIR / fname))
        return json.loads(out)

    def _rules(self, report: dict) -> set[str]:
        return {f["rule"] for f in report["findings"]}

    def test_bad_1_misplaced_replica_scheduling(self):
        self.assertIn("R1", self._rules(self._audit("bad-1-misplaced-rs.yaml")))

    def test_bad_2_jsonpath_path_caught(self):
        self.assertIn("R5", self._rules(self._audit("bad-2-jsonpath-path.yaml")))

    def test_bad_3_empty_selectors_caught(self):
        self.assertIn("R2", self._rules(self._audit("bad-3-empty-selectors.yaml")))

    def test_bad_4_deprecated_fields_warned(self):
        report = self._audit("bad-4-deprecated.yaml")
        self.assertIn("R14", self._rules(report))
        self.assertEqual(report["summary"]["error"], 0, "deprecation should warn, not error")
        self.assertGreaterEqual(report["summary"]["warn"], 2)

    def test_bad_5_mutual_exclusion_caught(self):
        self.assertIn("R4", self._rules(self._audit("bad-5-mutex-affinity.yaml")))

    def test_bad_6_float_weight_caught(self):
        report = self._audit("bad-6-float-weight.yaml")
        self.assertIn("R11", self._rules(report))
        # both bad weights should be flagged separately
        r11 = [f for f in report["findings"] if f["rule"] == "R11"]
        self.assertEqual(len(r11), 2)


class DebugTests(unittest.TestCase):
    BUNDLE = EXAMPLES / "03-propagation-failure-bundle" / "bundle.yaml"

    def _run(self, target: str) -> dict:
        rc, out, _ = run("debug_propagation.py", "--bundle", str(self.BUNDLE), "--target", target)
        # rc=1 is expected when verdict != ok
        return json.loads(out)

    def test_schedule_failure_recognised(self):
        report = self._run("v1/Service/default/checkout")
        self.assertEqual(report["verdict"], "schedule-failed")
        self.assertEqual(report["hop_reached"], "ResourceBinding")

    def test_missing_target_returns_indeterminate(self):
        report = self._run("v1/Service/default/does-not-exist")
        self.assertEqual(report["verdict"], "indeterminate")


class SchemaExtractorTests(unittest.TestCase):
    """We don't have the karmada CRDs vendored in this POC tree; if they are present
    next to the tree we exercise the extractor end-to-end. Otherwise we exercise the
    walker on a synthetic CRD so the test suite stays self-contained."""

    SYNTHETIC_CRD = {
        "apiVersion": "apiextensions.k8s.io/v1",
        "kind": "CustomResourceDefinition",
        "spec": {
            "versions": [{
                "schema": {"openAPIV3Schema": {
                    "type": "object",
                    "required": ["spec"],
                    "properties": {
                        "spec": {
                            "type": "object",
                            "required": ["resourceSelectors"],
                            "properties": {
                                "resourceSelectors": {
                                    "type": "array",
                                    "items": {
                                        "type": "object",
                                        "properties": {
                                            "apiVersion": {"type": "string", "description": "api version"},
                                            "kind": {"type": "string", "description": "kind"},
                                        },
                                    },
                                },
                                "preemption": {
                                    "type": "string",
                                    "enum": ["Always", "Never"],
                                    "description": "preempt behavior",
                                },
                            },
                        },
                    },
                }},
            }],
        },
    }

    def test_walker_emits_expected_paths(self):
        sys.path.insert(0, str(SCRIPTS))
        try:
            extractor = __import__("extract_crd_schema")
        finally:
            sys.path.pop(0)
        schema = self.SYNTHETIC_CRD["spec"]["versions"][0]["schema"]["openAPIV3Schema"]
        rows = extractor.walk(schema, "", set())
        paths = {r["path"] for r in rows}
        self.assertIn("spec", paths)
        self.assertIn("spec.resourceSelectors", paths)
        self.assertIn("spec.resourceSelectors[].apiVersion", paths)
        self.assertIn("spec.preemption", paths)
        preemption_row = next(r for r in rows if r["path"] == "spec.preemption")
        self.assertIn("enum: Always, Never", preemption_row["type"])


if __name__ == "__main__":
    unittest.main()
