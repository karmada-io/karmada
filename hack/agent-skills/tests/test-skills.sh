#!/bin/bash
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

# Skill validation tests
# Validates that all skills reference valid knowledge files and templates

set -euo pipefail

SKILLS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../skills" && pwd)"
KNOWLEDGE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../knowledge" && pwd)"
TEMPLATES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../templates" && pwd)"
EXAMPLES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../examples" && pwd)"

echo "=== Karmada Agent Skills Validation ==="
echo ""

# 1. All expected skills exist
echo "--- Checking skills ---"
EXPECTED_SKILLS=(
  "karmada-knowledge"
  "karmada-create-policy"
  "karmada-audit-policy"
  "karmada-explain-placement"
  "karmada-debug-propagation"
  "karmada-search"
  "karmada-controller-manager"
)

for skill in "${EXPECTED_SKILLS[@]}"; do
  if [[ -f "${SKILLS_DIR}/${skill}/SKILL.md" ]]; then
    echo "  [OK] ${skill}"
  else
    echo "  [FAIL] ${skill}: SKILL.md not found"
    exit 1
  fi
done
echo ""

# 2. All knowledge files exist
echo "--- Checking knowledge files ---"
EXPECTED_KNOWLEDGE=(
  "policy-apis.yaml"
  "policy-patterns.md"
  "troubleshooting.md"
  "components.md"
)

for kf in "${EXPECTED_KNOWLEDGE[@]}"; do
  if [[ -f "${KNOWLEDGE_DIR}/${kf}" ]]; then
    echo "  [OK] ${kf}"
  else
    echo "  [FAIL] ${kf}: not found"
    exit 1
  fi
done
echo ""

# 3. All templates exist and are valid YAML
echo "--- Checking templates ---"
EXPECTED_TEMPLATES=(
  "propagation-policy.yaml"
  "cluster-propagation-policy.yaml"
  "override-policy.yaml"
  "cluster-override-policy.yaml"
)

for tmpl in "${EXPECTED_TEMPLATES[@]}"; do
  if [[ -f "${TEMPLATES_DIR}/${tmpl}" ]]; then
    if python3 -c "import yaml; yaml.safe_load(open('${TEMPLATES_DIR}/${tmpl}'))" 2>/dev/null; then
      echo "  [OK] ${tmpl} (valid YAML)"
    else
      echo "  [FAIL] ${tmpl}: invalid YAML"
      exit 1
    fi
  else
    echo "  [FAIL] ${tmpl}: not found"
    exit 1
  fi
done
echo ""

# 4. All example directories have README and at least one YAML
echo "--- Checking examples ---"
EXPECTED_EXAMPLES=(
  "multi-country-setup"
  "failover"
  "image-override"
)

for ex in "${EXPECTED_EXAMPLES[@]}"; do
  if [[ -d "${EXAMPLES_DIR}/${ex}" ]]; then
    if [[ -f "${EXAMPLES_DIR}/${ex}/README.md" ]]; then
      yaml_count=$(find "${EXAMPLES_DIR}/${ex}" -maxdepth 1 -name "*.yaml" | wc -l | tr -d ' ')
      if [[ "$yaml_count" -gt 0 ]]; then
        echo "  [OK] ${ex} (README.md + ${yaml_count} YAML files)"
      else
        echo "  [FAIL] ${ex}: no YAML files found"
        exit 1
      fi
    else
      echo "  [FAIL] ${ex}: README.md not found"
      exit 1
    fi
  else
    echo "  [FAIL] ${ex}: directory not found"
    exit 1
  fi
done
echo ""

echo "=== All validations passed! ==="
