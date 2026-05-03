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
Tests for Karmada policy-generator script.
Run: python3 tests/test-policy-generator.py
"""

import json
import subprocess
import sys
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
GENERATOR = os.path.join(SCRIPT_DIR, "..", "scripts", "policy-generator.py")
VALIDATOR = os.path.join(SCRIPT_DIR, "..", "scripts", "validate-policy.py")


def run(cmd):
    return subprocess.run(cmd, capture_output=True, text=True)


def test_generate_propagation():
    """Generate a basic PropagationPolicy."""
    result = run([
        "python3", GENERATOR, "propagation",
        "--name", "test-app",
        "--namespace", "default",
        "--api", "apps/v1",
        "--kind", "Deployment",
        "--clusters", "member1,member2",
        "--weights", "1,1",
    ])
    assert result.returncode == 0, f"Generator failed: {result.stderr}"
    assert "PropagationPolicy" in result.stdout
    assert "test-app-propagation" in result.stdout
    assert "member1" in result.stdout
    assert "member2" in result.stdout
    print("PASS: test_generate_propagation")


def test_generate_override():
    """Generate a basic OverridePolicy with labels."""
    overrides = json.dumps([
        {"operator": "add", "value": {"env": "production"}}
    ])
    result = run([
        "python3", GENERATOR, "override",
        "--name", "test-app",
        "--namespace", "default",
        "--api", "apps/v1",
        "--kind", "Deployment",
        "--cluster", "member1",
        "--override-type", "labels",
        "--overrides", overrides,
    ])
    assert result.returncode == 0, f"Generator failed: {result.stderr}"
    assert "OverridePolicy" in result.stdout
    assert "member1" in result.stdout
    assert "env" in result.stdout
    print("PASS: test_generate_override")


def test_validate_valid_policy():
    """Validate a known-valid policy."""
    import tempfile
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
        f.write("""apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: test-policy
  namespace: default
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: test-app
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
""")
        f.flush()
        result = run(["python3", VALIDATOR, f.name])
        os.unlink(f.name)

    assert result.returncode == 0, f"Validation should pass: {result.stdout}"
    print("PASS: test_validate_valid_policy")


def test_validate_invalid_policy():
    """Validate a policy with errors."""
    import tempfile
    with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
        f.write("""apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: bad-policy
spec:
  resourceSelectors: []
  placement:
    clusterAffinity:
      clusterNames:
        - member1
    clusterAffinities:
      - affinityName: group1
        clusterNames:
          - member2
""")
        f.flush()
        result = run(["python3", VALIDATOR, f.name])
        os.unlink(f.name)

    assert result.returncode == 1, "Validation should fail for invalid policy"
    assert "ERROR" in result.stdout or "error" in result.stdout.lower()
    print("PASS: test_validate_invalid_policy")


def main():
    print("Running Karmada policy generator tests...\n")
    tests = [
        test_generate_propagation,
        test_generate_override,
        test_validate_valid_policy,
        test_validate_invalid_policy,
    ]
    for test in tests:
        try:
            test()
        except Exception as e:
            print(f"FAIL: {test.__name__}: {e}")
            sys.exit(1)
    print("\nAll tests passed!")


if __name__ == "__main__":
    main()
