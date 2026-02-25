#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=""
CONTEXT=""
TRACE_ENABLED=false

usage() {
  cat <<USAGE
Usage: $0 --kubeconfig <path> [--context <context>] [--trace]

Options:
  --kubeconfig    Path to kubeconfig file (required)
  --context       Kubeconfig context to use (optional)
  --trace         Enable bash xtrace (set -x)
USAGE
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --kubeconfig) KUBECONFIG_PATH="$2"; shift ;;
    --context)    CONTEXT="$2"; shift ;;
    --trace)      TRACE_ENABLED=true ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "Unknown parameter passed: $1"; usage; exit 1 ;;
  esac
  shift
done

if [[ -z "${KUBECONFIG_PATH}" ]]; then
  echo "Error: --kubeconfig is required."
  usage
  exit 1
fi

if [[ ! -f "$KUBECONFIG_PATH" ]]; then
  echo "Error: kubeconfig file not found at $KUBECONFIG_PATH"
  exit 1
fi

if [ "$TRACE_ENABLED" = true ]; then
  set -o xtrace
fi

# Build kubectl command with optional context
KUBECTL_CMD="kubectl --kubeconfig=$KUBECONFIG_PATH"
if [[ -n "$CONTEXT" ]]; then
  KUBECTL_CMD="$KUBECTL_CMD --context=$CONTEXT"
fi

echo "Creating PropagationPolicy in karmada..."
$KUBECTL_CMD apply -f - <<'POLICYEOF'
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: deployment-policy
  namespace: default
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
  placement:
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        dynamicWeight: AvailableReplicas
POLICYEOF

echo "Starting deployment lifecycle loop. Press Ctrl+C to stop."
while true; do
  echo "=== Applying nginx Deployment with 1 replicas..."
  $KUBECTL_CMD apply -f - <<'INNEREOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment-1
  namespace: default
  labels:
    app: nginx-1
    team: a
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-1
  template:
    metadata:
      labels:
        app: nginx-1
        team: a
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          ports:
            - containerPort: 80
INNEREOF

  echo "Waiting for initial rollout to complete (1 replicas)..."
  $KUBECTL_CMD --namespace default rollout status deployment/nginx-deployment-1 --timeout=180s

  echo "Sleeping for 2 seconds..."
  sleep 2

  echo "Scaling to 2 replicas..."
  $KUBECTL_CMD --namespace default scale deployment/nginx-deployment-1 --replicas=2

  echo "Waiting for all 2 replicas to be ready..."
  $KUBECTL_CMD --namespace default rollout status deployment/nginx-deployment-1 --timeout=180s

  echo "Sleeping for 2 seconds..."
  sleep 2

  echo "Deleting deployment..."
  $KUBECTL_CMD --namespace default delete deployment/nginx-deployment-1

  echo "Sleeping for 2 seconds..."
  sleep 2
done