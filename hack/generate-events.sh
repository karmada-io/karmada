#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=""
CONTEXT=""
TRACE_ENABLED=false
NAMESPACE="default"

usage() {
  cat <<USAGE
Usage: $0 --kubeconfig <path> [--context <context>] [--namespace <namespace>] [--trace]

Options:
  --kubeconfig    Path to kubeconfig file (required)
  --context       Kubeconfig context to use (optional)
  --namespace     Namespace to create resources in (default: default)
  --trace         Enable bash xtrace (set -x)
USAGE
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --kubeconfig) KUBECONFIG_PATH="$2"; shift ;;
    --context)    CONTEXT="$2"; shift ;;
    --namespace)  NAMESPACE="$2"; shift ;;
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

# Ensure namespace exists
echo "Ensuring namespace $NAMESPACE exists..."
$KUBECTL_CMD create namespace "$NAMESPACE" --dry-run=client -o yaml | $KUBECTL_CMD apply -f -

# Create PropagationPolicy for the namespace
echo "Creating PropagationPolicy for event generation..."
$KUBECTL_CMD apply -f - <<POLICYEOF
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: event-gen-policy
  namespace: $NAMESPACE
spec:
  resourceSelectors:
    - apiVersion: v1
      kind: Pod
    - apiVersion: apps/v1
      kind: Deployment
    - apiVersion: v1
      kind: ConfigMap
    - apiVersion: v1
      kind: Service
  placement:
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        dynamicWeight: AvailableReplicas
POLICYEOF

echo ""
echo "Starting event generation loop. Press Ctrl+C to stop."
echo "Generating events in namespace: $NAMESPACE"
echo ""

ITERATION=0

while true; do
  ITERATION=$((ITERATION + 1))
  echo "=== Iteration $ITERATION ==="

  # Scenario 1: Create a simple deployment (generates Normal events)
  echo "[1/8] Creating nginx deployment..."
  $KUBECTL_CMD apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: event-gen-nginx
  namespace: default
  labels:
    app: event-gen-nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: event-gen-nginx
  template:
    metadata:
      labels:
        app: event-gen-nginx
    spec:
      containers:
        - name: nginx
          image: nginx:alpine
          ports:
            - containerPort: 80
          resources:
            requests:
              memory: "32Mi"
              cpu: "50m"
            limits:
              memory: "64Mi"
              cpu: "100m"
EOF

  sleep 3

  # Scenario 2: Create a ConfigMap (generates Normal events)
  echo "[2/8] Creating ConfigMap..."
  $KUBECTL_CMD create configmap event-gen-config \
    --namespace="$NAMESPACE" \
    --from-literal=key1=value1 \
    --from-literal=key2=value2 \
    --dry-run=client -o yaml | $KUBECTL_CMD apply -f -

  sleep 2

  # Scenario 3: Create a Service (generates Normal events)
  echo "[3/8] Creating Service..."
  $KUBECTL_CMD apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: event-gen-service
  namespace: $NAMESPACE
spec:
  selector:
    app: event-gen-nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  type: ClusterIP
EOF

  sleep 2

  # Scenario 4: Create a pod with init container (generates multiple events)
  echo "[4/8] Creating pod with init container..."
  $KUBECTL_CMD apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: event-gen-init-pod
  namespace: $NAMESPACE
  labels:
    app: event-gen-init
spec:
  initContainers:
    - name: init-container
      image: busybox:latest
      command: ['sh', '-c', 'echo Initializing... && sleep 2']
  containers:
    - name: main-container
      image: nginx:alpine
      ports:
        - containerPort: 80
EOF

  sleep 5

  # Scenario 5: Scale the deployment (generates ScalingReplicaSet events)
  echo "[5/8] Scaling deployment up..."
  $KUBECTL_CMD --namespace "$NAMESPACE" scale deployment/event-gen-nginx --replicas=3

  sleep 3

  echo "[5/8] Scaling deployment down..."
  $KUBECTL_CMD --namespace "$NAMESPACE" scale deployment/event-gen-nginx --replicas=1

  sleep 3

  # Scenario 6: Create a pod that will fail (generates Warning events)
  echo "[6/8] Creating pod with invalid image (will generate warning events)..."
  $KUBECTL_CMD apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: event-gen-failing-pod
  namespace: $NAMESPACE
  labels:
    app: event-gen-failing
spec:
  containers:
    - name: failing-container
      image: this-image-does-not-exist:invalid-tag
      imagePullPolicy: Always
EOF

  sleep 5

  # Scenario 7: Create a pod with resource constraints that might cause scheduling issues
  echo "[7/8] Creating pod with high resource requests..."
  $KUBECTL_CMD apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: event-gen-resource-pod
  namespace: $NAMESPACE
  labels:
    app: event-gen-resources
spec:
  containers:
    - name: resource-container
      image: nginx:alpine
      resources:
        requests:
          memory: "16Mi"
          cpu: "10m"
        limits:
          memory: "32Mi"
          cpu: "50m"
EOF

  sleep 4

  # Scenario 8: Update the ConfigMap (generates events)
  echo "[8/8] Updating ConfigMap..."
  $KUBECTL_CMD create configmap event-gen-config \
    --namespace="$NAMESPACE" \
    --from-literal=key1=updated-value-$ITERATION \
    --from-literal=key2=updated-value-$ITERATION \
    --from-literal=iteration=$ITERATION \
    --dry-run=client -o yaml | $KUBECTL_CMD apply -f -

  sleep 3

  # Cleanup phase - delete resources to generate deletion events
  echo "Cleaning up resources..."

  echo "Deleting failing pod..."
  $KUBECTL_CMD --namespace "$NAMESPACE" delete pod event-gen-failing-pod --ignore-not-found=true

  sleep 2

  echo "Deleting init pod..."
  $KUBECTL_CMD --namespace "$NAMESPACE" delete pod event-gen-init-pod --ignore-not-found=true

  sleep 2

  echo "Deleting resource pod..."
  $KUBECTL_CMD --namespace "$NAMESPACE" delete pod event-gen-resource-pod --ignore-not-found=true

  sleep 2

  echo "Deleting Service..."
  $KUBECTL_CMD --namespace "$NAMESPACE" delete service event-gen-service --ignore-not-found=true

  sleep 2

  echo "Deleting Deployment..."
  $KUBECTL_CMD --namespace "$NAMESPACE" delete deployment event-gen-nginx --ignore-not-found=true

  sleep 2

  echo "Deleting ConfigMap..."
  $KUBECTL_CMD --namespace "$NAMESPACE" delete configmap event-gen-config --ignore-not-found=true

  sleep 3

  echo ""
  echo "=== Iteration $ITERATION complete. Waiting before next iteration... ==="
  echo ""
  sleep 5
done
