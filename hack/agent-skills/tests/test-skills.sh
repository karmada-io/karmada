#!/usr/bin/env bash
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

# Integration smoke test for the agent-skills toolchain.
#
# Verifies:
#   1. policy-generator.py produces lint-clean YAML for every documented
#      example.
#   2. validate-policy.py catches a curated set of bad inputs (each one
#      should hit a specific rule ID).
#   3. Every SKILL.md has the required front-matter.
#
# Usage: bash hack/agent-skills/tests/test-skills.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GEN=("python3" "${ROOT}/scripts/policy-generator.py")
LINT=("python3" "${ROOT}/scripts/validate-policy.py")
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
ok()  { pass=$((pass+1)); printf '  \033[32mPASS\033[0m %s\n' "$*"; }
bad() { fail=$((fail+1)); printf '  \033[31mFAIL\033[0m %s\n' "$*"; }

# Asserts that running LINT on $1 exits 0 (clean) and labels with $2.
assert_clean() {
  local file="$1" label="$2"
  if "${LINT[@]}" "$file" >/dev/null; then
    ok "$label"
  else
    bad "$label"
  fi
}

# Asserts that LINT on $1 emits rule code $2 in its output (label $3).
assert_rule() {
  local file="$1" rule="$2" label="$3" out
  out="$("${LINT[@]}" "$file" 2>&1 || true)"
  if printf '%s\n' "$out" | grep -q "$rule"; then
    ok "$label"
  else
    bad "$label"
    printf '%s\n' "$out"
  fi
}

echo "== 1. Generator round-trips =="

"${GEN[@]}" --kind PropagationPolicy --name nginx-eu --namespace default \
  --target-kind Deployment --target-name nginx \
  --clusters member-de,member-fr --replica-scheduling Divided \
  --output "$TMP/pp1.yaml"
assert_clean "$TMP/pp1.yaml" "PropagationPolicy (clusters) lint-clean"

"${GEN[@]}" --kind PropagationPolicy --name api-failover --namespace default \
  --target-kind Deployment --target-name api \
  --affinity-group primary=member-prod-east \
  --affinity-group backup=member-prod-west \
  --replica-scheduling Duplicated \
  --failover-toleration-seconds 120 \
  --failover-purge-mode Gracefully \
  --failover-grace-period-seconds 60 \
  --output "$TMP/pp2.yaml"
assert_clean "$TMP/pp2.yaml" "PropagationPolicy (failover) lint-clean"

"${GEN[@]}" --kind ClusterPropagationPolicy --name global-crd \
  --target-api-version apiextensions.k8s.io/v1 \
  --target-kind CustomResourceDefinition --target-name foos.example.com \
  --clusters member-1,member-2 \
  --output "$TMP/cpp.yaml"
assert_clean "$TMP/cpp.yaml" "ClusterPropagationPolicy lint-clean"

"${GEN[@]}" --kind OverridePolicy --name cn-mirror --namespace default \
  --target-kind Deployment --target-name nginx \
  --clusters member-cn \
  --override-image-registry registry.cn-hangzhou.aliyuncs.com/myorg \
  --output "$TMP/op.yaml"
assert_clean "$TMP/op.yaml" "OverridePolicy (image) lint-clean"

echo
echo "== 2. Linter catches known-bad inputs =="

cat > "$TMP/bad-jsonpath.yaml" <<'YAML'
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata: { name: bad, namespace: default }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: x
  overrideRules:
    - targetCluster: { clusterNames: [member-1] }
      overriders:
        plaintext:
          - path: spec.replicas
            operator: replace
            value: 5
YAML
assert_rule "$TMP/bad-jsonpath.yaml" "KMD221" \
  "KMD221 (JSONPath instead of JSON Pointer)"

cat > "$TMP/bad-imagetypo.yaml" <<'YAML'
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata: { name: bad, namespace: default }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: x
  overrideRules:
    - targetCluster: { clusterNames: [member-1] }
      overriders:
        imageOverride:
          - component: Registry
            operator: replace
            value: example.com
YAML
assert_rule "$TMP/bad-imagetypo.yaml" "KMD210" "KMD210 (imageOverride typo)"

cat > "$TMP/bad-affinity-both.yaml" <<'YAML'
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata: { name: bad, namespace: default }
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: x
  placement:
    clusterAffinity:
      clusterNames: [member-1]
    clusterAffinities:
      - affinityName: a
        clusterNames: [member-2]
YAML
assert_rule "$TMP/bad-affinity-both.yaml" "KMD100" \
  "KMD100 (affinity singular+plural)"

cat > "$TMP/bad-deployment-apiversion.yaml" <<'YAML'
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata: { name: bad, namespace: default }
spec:
  resourceSelectors:
    - apiVersion: v1
      kind: Deployment
      name: x
  placement:
    clusterAffinity:
      clusterNames: [member-1]
YAML
assert_rule "$TMP/bad-deployment-apiversion.yaml" "KMD025" \
  "KMD025 (Deployment with apiVersion v1)"

echo
echo "== 3. SKILL.md front matter present =="
shopt -s nullglob
for skill in "$ROOT"/skills/*/SKILL.md; do
  name="$(basename "$(dirname "$skill")")/SKILL.md"
  if head -1 "$skill" | grep -q '^---$' \
      && head -10 "$skill" | grep -q '^name:' \
      && head -10 "$skill" | grep -q '^description:'; then
    ok "$name has front matter"
  else
    bad "$name missing front matter"
  fi
done

echo
echo "== Summary =="
echo "  passed: $pass"
echo "  failed: $fail"
[[ "$fail" -eq 0 ]]
