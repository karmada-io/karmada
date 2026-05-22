#!/usr/bin/env bash
# demo.sh — replay the POC end-to-end. Mentors run this to see every helper in action.
#
# Usage:   bash hack/agent-skills/demo.sh
# Output:  step-by-step terminal walkthrough showing what each skill's helper produces.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Optional pacing for recordings — set DEMO_PAUSE=1.2 for asciinema, default 0 in CI.
PAUSE="${DEMO_PAUSE:-0}"

bold()  { printf '\033[1m%s\033[0m\n' "$*"; }
sec()   { printf '\n\033[1;36m=== %s ===\033[0m\n' "$*"; sleep "$PAUSE"; }
pause() { sleep "$PAUSE"; }

# Allow the user to point at a local checkout of karmada to test schema regeneration.
# Defaults to ../../research/karmada/charts/karmada/_crds (matches the POC layout).
CRD_DIR="${KARMADA_CRD_DIR:-../../../research/karmada/charts/karmada/_crds/bases/policy}"

sec "1. Regenerate auto schema docs from upstream CRDs"
if [[ -d "$CRD_DIR" ]]; then
  python3 scripts/extract_crd_schema.py \
      --crd "$CRD_DIR/policy.karmada.io_propagationpolicies.yaml" \
      --out knowledge/10-propagation-schema.md \
      --title "PropagationPolicy"
  python3 scripts/extract_crd_schema.py \
      --crd "$CRD_DIR/policy.karmada.io_overridepolicies.yaml" \
      --out knowledge/11-override-schema.md \
      --title "OverridePolicy"
  echo "  (re-generated 10-propagation-schema.md and 11-override-schema.md)"
else
  echo "  KARMADA_CRD_DIR not found at $CRD_DIR — using committed schema files."
  echo "  Set KARMADA_CRD_DIR to your local karmada checkout to test regeneration."
fi

sec "2. Generate the multi-country policy from intent JSON"
python3 scripts/generate_policy.py \
    --intent examples/01-multi-country-frontend/intent.json \
    --out /tmp/generated-policy.yaml
echo "  wrote /tmp/generated-policy.yaml"
head -25 /tmp/generated-policy.yaml | sed 's/^/    /'
echo "    ..."

sec "3. Audit the generated policy (should be clean)"
python3 scripts/audit_policy.py /tmp/generated-policy.yaml --format text

sec "4. Audit a deliberately broken policy (should report R5 = JSONPath misuse)"
python3 scripts/audit_policy.py examples/02-bad-policy-audit-targets/bad-2-jsonpath-path.yaml --format text || true

sec "5. Audit a misplaced-replicaScheduling policy (should report R1)"
python3 scripts/audit_policy.py examples/02-bad-policy-audit-targets/bad-1-misplaced-rs.yaml --format text || true

sec "6. Debug a propagation failure (should report verdict=schedule-failed)"
python3 scripts/debug_propagation.py \
    --bundle examples/03-propagation-failure-bundle/bundle.yaml \
    --target v1/Service/default/checkout || true

pause
sec "7. Run the test suite"
python3 -m unittest tests.test_helpers 2>&1 | tail -5

pause
bold "Done. All seven steps demonstrate one of the POC's deterministic helpers."
