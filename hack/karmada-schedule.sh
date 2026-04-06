#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=""
TRACE_ENABLED=false

usage() {
  cat <<USAGE
Usage: $0 --kubeconfig <path>  [--trace]

Options:
  --trace         Enable bash xtrace (set -x)
USAGE
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --trace)      TRACE_ENABLED=true ;;
    -h|--help)    usage; exit 0 ;;
    *) echo "Unknown parameter passed: $1"; usage; exit 1 ;;
  esac
  shift
done

if [ "$TRACE_ENABLED" = true ]; then
  set -o xtrace
fi

echo "Applying nginx Deployment..."
kubectl --kubeconfig ~/.kube/karmada.config --context karmada-apiserver  apply -f - <<'EOF'
---
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
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
    team: a
spec:
  replicas: 4
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
        team: a
    spec:
      containers:
        - name: nginx
          image: docker.io/nginx:latest
          ports:
            - containerPort: 80
EOF

#echo "Waiting for initial rollout to complete..."
#kubectl --kubeconfig ~/.kube/karmada.config --context karmada-apiserver  rollout status deployment/nginx-deployment --timeout=180s

echo "Starting infinite scale loop: 2 -> (sleep 3s) -> 4 -> repeat. Press Ctrl+C to stop."
while true; do
  echo "Scaling to 2 replicas..."
  kubectl --kubeconfig ~/.kube/karmada.config --context karmada-apiserver  scale deployment/nginx-deployment --replicas=2
  sleep 1
  echo "Scaling to 4 replicas..."
  kubectl --kubeconfig ~/.kube/karmada.config --context karmada-apiserver scale deployment/nginx-deployment --replicas=4
  sleep 1
done
