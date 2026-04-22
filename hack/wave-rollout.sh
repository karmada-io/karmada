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
#
# wave-rollout.sh — Percentage-based traffic shifting using two Deployments in tandem.
#
# Usage:
#   wave-rollout.sh start  <cluster|all> <steps>  [--kubeconfig <path>] [--context <ctx>] [--wait <seconds>]
#   wave-rollout.sh finalize                       [--kubeconfig <path>] [--context <ctx>]
#   wave-rollout.sh abort  <cluster|all>           [--kubeconfig <path>] [--context <ctx>]
#
# Commands:
#   start    — create the wave Deployment and step through traffic percentages
#   finalize — update base manifest to v2, restore base replicas, remove wave resources
#   abort    — restore base replicas and remove wave resources without promoting
#
# Arguments:
#   <cluster>  — target member cluster name (e.g. member1)
#   all        — target all three member clusters simultaneously
#   <steps>    — comma-separated traffic percentages (e.g. 25,50,100)
#
# Options:
#   --kubeconfig  path to kubeconfig        (default: ~/.kube/karmada.config)
#   --context     karmada API context       (default: karmada-apiserver)
#   --wait        seconds to pause per step (default: 30)
#
# Replica table (6 replicas per cluster via Duplicated policy):
#   Step   Base  Wave  Approx. traffic share
#   25%      4     2        ~33% wave
#   50%      3     3        ~50% wave
#   100%     0     6       100% wave
#
# Prerequisites:
#   - Karmada control plane running (hack/local-up-karmada.sh)
#   - http-probe-app deployed via samples/progressive-rollout/http-probe-app-wave-base.yaml
#   - KUBECONFIG pointing to karmada.config or --kubeconfig flag provided

set -euo pipefail

if ! command -v kubectl &> /dev/null; then
  echo "Error: kubectl is not installed or not in PATH." >&2
  exit 1
fi

# ── Constants ─────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# These can be overridden via environment variables to support different environments.
REPLICAS_PER_CLUSTER=${REPLICAS_PER_CLUSTER:-6}
NAMESPACE=${NAMESPACE:-default}
read -ra ALL_CLUSTERS <<< "${CLUSTERS:-member1 member2 member3}"

BASE_DEPLOY="http-probe-app"
WAVE_DEPLOY="http-probe-app-wave"
BASE_DOWN_POLICY="http-probe-wave-base-down"
WAVE_UP_POLICY="http-probe-wave-up"
WAVE_PROPAGATION="http-probe-app-wave-propagation"

# ── Defaults ──────────────────────────────────────────────────────────────────
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/.kube/karmada.config}"
KARMADA_CONTEXT="karmada-apiserver"
WAIT_SECONDS=30

# ── Argument parsing ──────────────────────────────────────────────────────────
COMMAND="${1:-}"
shift || true

TARGET=""
STEPS="25,50,100"

case "$COMMAND" in
  start)
    TARGET="${1:-}"; shift || true
    STEPS="${1:-25,50,100}"; shift || true
    if [[ -z "$TARGET" ]]; then
      echo "Usage: $0 start <cluster|all> [steps]" >&2
      exit 1
    fi
    ;;
  abort)
    TARGET="${1:-all}"; shift || true
    ;;
  finalize) ;;
  *)
    echo "Usage: $0 <start|finalize|abort> [args]" >&2
    exit 1
    ;;
esac

while [[ $# -gt 0 ]]; do
  case "$1" in
    --kubeconfig)
      [[ -z "${2:-}" ]] && { echo "Error: --kubeconfig requires a value." >&2; exit 1; }
      KUBECONFIG_PATH="$2"; shift 2 ;;
    --context)
      [[ -z "${2:-}" ]] && { echo "Error: --context requires a value." >&2; exit 1; }
      KARMADA_CONTEXT="$2"; shift 2 ;;
    --wait)
      [[ -z "${2:-}" ]] && { echo "Error: --wait requires a value." >&2; exit 1; }
      WAIT_SECONDS="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

export KUBECONFIG="$KUBECONFIG_PATH"
KC="kubectl --context $KARMADA_CONTEXT"

# ── Helpers ───────────────────────────────────────────────────────────────────

resolve_clusters() {
  if [[ "$1" == "all" ]]; then
    echo "${ALL_CLUSTERS[@]}"
  else
    echo "$1"
  fi
}

# Converts "member1 member2 member3" → "[member1, member2, member3]"
format_cluster_list() {
  local clusters=("$@")
  local result="["
  for i in "${!clusters[@]}"; do
    [[ $i -gt 0 ]] && result+=", "
    result+="${clusters[$i]}"
  done
  echo "${result}]"
}

# Print and apply a manifest passed via stdin or as a string
apply_manifest() {
  local description="$1"
  local manifest="$2"
  echo "==> ${description}"
  echo "--- manifest"
  echo "${manifest}"
  echo "---"
  printf "%s\n" "${manifest}" | $KC apply -f -
}

# Apply a replica-scaling OverridePolicy for a Deployment
apply_override() {
  local name="$1"
  local cluster_list="$2"
  local deploy="$3"
  local replicas="$4"
  local role="$5"

  apply_manifest "OverridePolicy: ${name} (${role}, replicas=${replicas})" "$(cat <<EOF
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: ${name}
  namespace: ${NAMESPACE}
  labels:
    wave-role: ${role}
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: ${deploy}
  overrideRules:
    - targetCluster:
        clusterNames: ${cluster_list}
      overriders:
        plaintext:
          - path: /spec/replicas
            operator: replace
            value: ${replicas}
EOF
)"
}

# Wait for a Deployment's ResourceBinding to reach the expected ready replica count
wait_for_ready() {
  local deploy="$1"
  local cluster="$2"
  local expected="$3"
  local label="${4:-$deploy}"

  echo "  Waiting for ${label} to reach ${expected} ready replicas on ${cluster}..."
  for i in $(seq 1 40); do
    ready=$($KC get resourcebinding "${deploy}-deployment" -n "$NAMESPACE" \
      -o jsonpath="{.status.aggregatedStatus[?(@.clusterName==\"${cluster}\")].status.readyReplicas}" \
      2>/dev/null)
    desired=$($KC get resourcebinding "${deploy}-deployment" -n "$NAMESPACE" \
      -o jsonpath="{.status.aggregatedStatus[?(@.clusterName==\"${cluster}\")].status.replicas}" \
      2>/dev/null)
    if [[ -n "$ready" ]] && { [[ "$ready" == "$expected" ]] || [[ "$expected" == "desired" && -n "$desired" && "$ready" == "$desired" ]]; }; then
      echo "  Ready (${ready}/${desired:-$expected})."
      return 0
    fi
    echo "  ($i/40) ${label} ready=${ready:-0}/${expected} — retrying in 5s..."
    sleep 5
  done
  echo "  Timed out waiting for ${label} on ${cluster}." >&2
  exit 1
}

# ── Commands ──────────────────────────────────────────────────────────────────

cmd_start() {
  local manifests="${SCRIPT_DIR}/../samples/progressive-rollout"

  local clusters
  read -ra clusters <<< "$(resolve_clusters "$TARGET")"
  local cluster_list
  cluster_list=$(format_cluster_list "${clusters[@]}")

  echo "==> Applying wave Deployment: ${WAVE_DEPLOY} (v2)"
  echo "--- manifest: samples/progressive-rollout/http-probe-app-wave-deployment.yaml"
  cat "${manifests}/http-probe-app-wave-deployment.yaml"
  echo "---"
  $KC apply -f "${manifests}/http-probe-app-wave-deployment.yaml" -n "$NAMESPACE"

  # PropagationPolicy is dynamic (clusterNames depends on --target argument)
  apply_manifest "PropagationPolicy: ${WAVE_PROPAGATION} → ${cluster_list}" "$(cat <<EOF
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: ${WAVE_PROPAGATION}
  namespace: ${NAMESPACE}
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: ${WAVE_DEPLOY}
  placement:
    clusterAffinity:
      clusterNames: ${cluster_list}
EOF
)"

  IFS=',' read -ra steps <<< "$STEPS"
  for pct in "${steps[@]}"; do
    pct="${pct// /}"
    pct="${pct//%/}"
    if ! [[ "$pct" =~ ^[0-9]+$ ]] || (( pct < 0 || pct > 100 )); then
      echo "Error: invalid step '${pct}' — must be an integer in [0,100]." >&2
      exit 1
    fi
    wave_replicas=$(( (REPLICAS_PER_CLUSTER * pct + 99) / 100 ))
    [[ $wave_replicas -gt $REPLICAS_PER_CLUSTER ]] && wave_replicas=$REPLICAS_PER_CLUSTER
    base_replicas=$(( REPLICAS_PER_CLUSTER - wave_replicas ))

    echo ""
    echo "==> Step ${pct}%: base=${base_replicas}, wave=${wave_replicas} per cluster"
    apply_override "$WAVE_UP_POLICY"   "$cluster_list" "$WAVE_DEPLOY" "$wave_replicas" "wave-up"
    apply_override "$BASE_DOWN_POLICY" "$cluster_list" "$BASE_DEPLOY" "$base_replicas" "base-down"

    for cluster in "${clusters[@]}"; do
      wait_for_ready "$WAVE_DEPLOY" "$cluster" "$wave_replicas" "wave"
    done

    echo "  Holding for ${WAIT_SECONDS}s — observe the dashboard before advancing..."
    sleep "$WAIT_SECONDS"
  done

  echo ""
  echo "==> All steps complete. Run './hack/wave-rollout.sh finalize' to promote or './hack/wave-rollout.sh abort' to restore."
}

cmd_finalize() {
  # Use http-probe-app-wave-base-v2.yaml — identical to http-probe-app-wave-base.yaml
  # but with CLUSTER_LABEL: v2. Cannot use http-probe-app-v2.yaml here because it
  # uses Divided scheduling which would drop replicas from 6 to 2 per cluster.
  echo "==> Updating base manifest to v2 (preserving Duplicated scheduling)..."
  echo "--- manifest: samples/progressive-rollout/http-probe-app-wave-base-v2.yaml"
  cat "${SCRIPT_DIR}/../samples/progressive-rollout/http-probe-app-wave-base-v2.yaml"
  echo "---"
  $KC apply -f "${SCRIPT_DIR}/../samples/progressive-rollout/http-probe-app-wave-base-v2.yaml" -n "$NAMESPACE"

  echo "==> Deleting base-down OverridePolicy — base scales back up to ${REPLICAS_PER_CLUSTER}×v2..."
  $KC delete overridepolicy "$BASE_DOWN_POLICY" -n "$NAMESPACE" --ignore-not-found

  # Determine which clusters the wave was targeting by inspecting the PropagationPolicy
  local wave_clusters
  wave_clusters=$($KC get propagationpolicy "$WAVE_PROPAGATION" -n "$NAMESPACE" \
    -o jsonpath='{.spec.placement.clusterAffinity.clusterNames[*]}' 2>/dev/null || true)
  if [[ -z "$wave_clusters" ]]; then
    echo "Warning: could not read wave cluster list from ${WAVE_PROPAGATION}; falling back to ALL_CLUSTERS." >&2
    wave_clusters="${ALL_CLUSTERS[*]}"
  fi
  read -ra finalize_clusters <<< "$wave_clusters"

  echo "==> Waiting for base Deployment to be fully ready on wave clusters: ${finalize_clusters[*]}..."
  for cluster in "${finalize_clusters[@]}"; do
    wait_for_ready "$BASE_DEPLOY" "$cluster" "desired" "base"
  done

  echo "==> Removing wave resources..."
  $KC delete overridepolicy    "$WAVE_UP_POLICY"   -n "$NAMESPACE" --ignore-not-found
  $KC delete propagationpolicy "$WAVE_PROPAGATION" -n "$NAMESPACE" --ignore-not-found
  $KC delete deployment        "$WAVE_DEPLOY"      -n "$NAMESPACE" --ignore-not-found

  echo "==> Done. All clusters are now on v2 with ${REPLICAS_PER_CLUSTER} replicas per cluster."
}

cmd_abort() {
  local clusters
  read -ra clusters <<< "$(resolve_clusters "$TARGET")"

  echo "==> Deleting base-down OverridePolicy — base replicas will restore..."
  $KC delete overridepolicy "$BASE_DOWN_POLICY" -n "$NAMESPACE" --ignore-not-found

  echo "==> Waiting for base Deployment to be fully ready before removing wave pods..."
  for cluster in "${clusters[@]}"; do
    wait_for_ready "$BASE_DEPLOY" "$cluster" "desired" "base"
  done

  echo "==> Base fully restored — removing wave resources..."
  $KC delete overridepolicy    "$WAVE_UP_POLICY"   -n "$NAMESPACE" --ignore-not-found
  $KC delete propagationpolicy "$WAVE_PROPAGATION" -n "$NAMESPACE" --ignore-not-found
  $KC delete deployment        "$WAVE_DEPLOY"      -n "$NAMESPACE" --ignore-not-found

  echo "==> Done. Base Deployment restored to v1."
}

# ── Dispatch ──────────────────────────────────────────────────────────────────
case "$COMMAND" in
  start)    cmd_start ;;
  finalize) cmd_finalize ;;
  abort)    cmd_abort ;;
esac
