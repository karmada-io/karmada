"""Unit tests for the policy validator (linter).

Each test pins a specific rule ID. When a finding's rule code changes,
the test fails — which is the point: rule IDs are part of the public
contract of the audit skill.

Run from repo root:
    python3 -m unittest hack/agent-skills/tests/test_validate_policy.py
"""
from __future__ import annotations

import importlib
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "scripts"))

validate_policy = importlib.import_module("validate-policy")  # type: ignore
validate_doc = validate_policy.validate_doc


def rules(findings) -> set[str]:
    return {f.rule for f in findings}


class WellFormedPolicies(unittest.TestCase):
    def test_minimal_propagation_policy_is_clean(self):
        doc = {
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind": "PropagationPolicy",
            "metadata": {"name": "x", "namespace": "default"},
            "spec": {
                "resourceSelectors": [
                    {"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}
                ],
                "placement": {"clusterAffinity": {"clusterNames": ["m1"]}},
            },
        }
        self.assertEqual(validate_doc(doc), [])

    def test_minimal_override_policy_is_clean(self):
        doc = {
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind": "OverridePolicy",
            "metadata": {"name": "x", "namespace": "default"},
            "spec": {
                "resourceSelectors": [
                    {"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}
                ],
                "overrideRules": [
                    {
                        "targetCluster": {"clusterNames": ["m1"]},
                        "overriders": {
                            "imageOverrider": [
                                {"component": "Registry", "operator": "replace",
                                 "value": "example.com"}
                            ]
                        },
                    }
                ],
            },
        }
        self.assertEqual(validate_doc(doc), [])


class BadInputsHitRules(unittest.TestCase):
    def _base_propagation(self):
        return {
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind": "PropagationPolicy",
            "metadata": {"name": "x", "namespace": "default"},
            "spec": {
                "resourceSelectors": [
                    {"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}
                ],
                "placement": {"clusterAffinity": {"clusterNames": ["m1"]}},
            },
        }

    def test_kmd001_wrong_api_version(self):
        doc = self._base_propagation()
        doc["apiVersion"] = "v1"
        self.assertIn("KMD001", rules(validate_doc(doc)))

    def test_kmd002_unknown_kind(self):
        doc = self._base_propagation()
        doc["kind"] = "WorkloadPolicy"
        self.assertIn("KMD002", rules(validate_doc(doc)))

    def test_kmd011_namespaced_missing_namespace(self):
        doc = self._base_propagation()
        doc["metadata"].pop("namespace")
        self.assertIn("KMD011", rules(validate_doc(doc)))

    def test_kmd012_cluster_scoped_with_namespace(self):
        doc = self._base_propagation()
        doc["kind"] = "ClusterPropagationPolicy"
        # namespace still present -> error
        self.assertIn("KMD012", rules(validate_doc(doc)))

    def test_kmd025_deployment_with_v1_apiversion(self):
        doc = self._base_propagation()
        doc["spec"]["resourceSelectors"][0]["apiVersion"] = "v1"
        self.assertIn("KMD025", rules(validate_doc(doc)))

    def test_kmd100_affinity_singular_and_plural(self):
        doc = self._base_propagation()
        doc["spec"]["placement"]["clusterAffinities"] = [
            {"affinityName": "a", "clusterNames": ["m2"]}
        ]
        self.assertIn("KMD100", rules(validate_doc(doc)))

    def test_kmd110_bad_replica_scheduling_type(self):
        doc = self._base_propagation()
        doc["spec"]["placement"]["replicaScheduling"] = {
            "replicaSchedulingType": "Sharded"
        }
        self.assertIn("KMD110", rules(validate_doc(doc)))

    def test_kmd120_spread_field_and_label_together(self):
        doc = self._base_propagation()
        doc["spec"]["placement"]["spreadConstraints"] = [
            {"spreadByField": "region", "spreadByLabel": "topology"}
        ]
        self.assertIn("KMD120", rules(validate_doc(doc)))

    def test_kmd122_min_greater_than_max(self):
        doc = self._base_propagation()
        doc["spec"]["placement"]["spreadConstraints"] = [
            {"spreadByField": "region", "minGroups": 5, "maxGroups": 2}
        ]
        self.assertIn("KMD122", rules(validate_doc(doc)))

    def test_kmd131_failover_missing_toleration(self):
        doc = self._base_propagation()
        doc["spec"]["failover"] = {
            "application": {"purgeMode": "Graciously"}
        }
        self.assertIn("KMD131", rules(validate_doc(doc)))

    def test_kmd210_imageoverride_typo(self):
        doc = {
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind": "OverridePolicy",
            "metadata": {"name": "x", "namespace": "default"},
            "spec": {
                "resourceSelectors": [
                    {"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}
                ],
                "overrideRules": [
                    {
                        "targetCluster": {"clusterNames": ["m1"]},
                        "overriders": {
                            "imageOverride": [  # typo!
                                {"component": "Registry", "operator": "replace",
                                 "value": "example.com"}
                            ]
                        },
                    }
                ],
            },
        }
        self.assertIn("KMD210", rules(validate_doc(doc)))

    def test_kmd221_jsonpath_instead_of_pointer(self):
        doc = {
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind": "OverridePolicy",
            "metadata": {"name": "x", "namespace": "default"},
            "spec": {
                "resourceSelectors": [
                    {"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}
                ],
                "overrideRules": [
                    {
                        "targetCluster": {"clusterNames": ["m1"]},
                        "overriders": {
                            "plaintext": [
                                {"path": "spec.replicas",  # JSONPath, not Pointer
                                 "operator": "replace", "value": 3}
                            ]
                        },
                    }
                ],
            },
        }
        self.assertIn("KMD221", rules(validate_doc(doc)))

    def test_kmd222_unknown_operator(self):
        doc = {
            "apiVersion": "policy.karmada.io/v1alpha1",
            "kind": "OverridePolicy",
            "metadata": {"name": "x", "namespace": "default"},
            "spec": {
                "resourceSelectors": [
                    {"apiVersion": "apps/v1", "kind": "Deployment", "name": "x"}
                ],
                "overrideRules": [
                    {
                        "targetCluster": {"clusterNames": ["m1"]},
                        "overriders": {
                            "plaintext": [
                                {"path": "/spec/replicas",
                                 "operator": "merge",  # invalid
                                 "value": 3}
                            ]
                        },
                    }
                ],
            },
        }
        self.assertIn("KMD222", rules(validate_doc(doc)))


if __name__ == "__main__":
    unittest.main()
