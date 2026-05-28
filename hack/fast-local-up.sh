#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script starts a minimal local karmada control plane using prebuilt images.
# Optimized for low RAM/CPU and automatic dependency handling.
# Fixed for WSL2 + Docker Desktop connectivity.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# Variable definitions
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
export KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
export MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
export MEMBER_CLUSTER_NAME=${MEMBER_CLUSTER_NAME:-"member1"}
export KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}
export CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}

# Set KUBECONFIG for all subsequent commands
export KUBECONFIG="${MAIN_KUBECONFIG}"

# Optimization: Skip building from source
export BUILD_FROM_SOURCE=false

# Step 0: Ensure dependencies are installed
echo "Checking dependencies..."
mkdir -p "${KIND_LOG_FILE}"

# Install kind if missing
if ! util::cmd_exist kind; then
  echo "kind not found, installing..."
  util::install_tools "sigs.k8s.io/kind" "${KIND_VERSION}"
fi

# Install kubectl if missing
if ! util::cmd_exist kubectl; then
  echo "kubectl not found, installing..."
  util::install_kubectl "" "$(go env GOARCH)" "$(go env GOOS)"
fi

# Install cfssl if missing
if ! util::cmd_exist cfssl || ! util::cmd_exist cfssljson; then
  echo "cfssl/cfssljson not found, installing..."
  util::cmd_must_exist_cfssl "v1.6.5"
fi

# Install karmadactl if missing
if ! util::cmd_exist karmadactl; then
  echo "karmadactl not found, installing..."
  INSTALL_LOCATION="${REPO_ROOT}/_output/bin"
  mkdir -p "${INSTALL_LOCATION}"
  rm -f "${INSTALL_LOCATION}/karmadactl"
  export INSTALL_LOCATION
  "${REPO_ROOT}"/hack/install-cli.sh karmadactl
  export PATH="${INSTALL_LOCATION}:${PATH}"
fi

# Step 1: Create clusters
echo "Creating clusters (1 Host + 1 Member)..."
kind delete clusters "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_NAME}" || true
rm -f "${MAIN_KUBECONFIG}" "${MEMBER_CLUSTER_KUBECONFIG}"

# Prepare host cluster config with port mapping
TEMP_PATH=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH}; }' EXIT
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/karmada-host.yaml "${TEMP_PATH}"/karmada-host.yaml
sed -i "s/{{host_ipaddress}}/127.0.0.1/g" "${TEMP_PATH}"/karmada-host.yaml

util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/karmada-host.yaml
util::create_cluster "${MEMBER_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"

echo "Waiting for clusters to be ready..."
util::check_clusters_ready "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_NAME}"

# Step 2: Deploy Karmada control plane
echo "Deploying Karmada Control Plane..."
export KARMADA_APISERVER_IP=$(util::get_docker_native_ipaddress "${HOST_CLUSTER_NAME}-control-plane")
"${REPO_ROOT}"/hack/deploy-karmada.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# Step 3: Join member cluster
echo "Joining member cluster '${MEMBER_CLUSTER_NAME}' (In-Container Join)..."
KARMADACTL_BIN=$(which karmadactl)

CP_KUBECONFIG_INTERNAL="${TEMP_PATH}/karmada-internal.config"
MEM_KUBECONFIG_INTERNAL="${TEMP_PATH}/members-internal.config"
cp "${MAIN_KUBECONFIG}" "${CP_KUBECONFIG_INTERNAL}"
cp "${MEMBER_CLUSTER_KUBECONFIG}" "${MEM_KUBECONFIG_INTERNAL}"

kubectl config set-cluster "karmada-apiserver" --server="https://${KARMADA_APISERVER_IP}:5443" --insecure-skip-tls-verify=true --kubeconfig="${CP_KUBECONFIG_INTERNAL}"
kubectl config set-cluster "kind-${MEMBER_CLUSTER_NAME}" --server="https://$(util::get_docker_native_ipaddress "${MEMBER_CLUSTER_NAME}-control-plane"):6443" --kubeconfig="${MEM_KUBECONFIG_INTERNAL}"

docker cp "${KARMADACTL_BIN}" "${HOST_CLUSTER_NAME}-control-plane:/usr/local/bin/karmadactl"
docker exec "${HOST_CLUSTER_NAME}-control-plane" chmod +x /usr/local/bin/karmadactl
cat "${CP_KUBECONFIG_INTERNAL}" | docker exec -i "${HOST_CLUSTER_NAME}-control-plane" tee /tmp/karmada.config > /dev/null
cat "${MEM_KUBECONFIG_INTERNAL}" | docker exec -i "${HOST_CLUSTER_NAME}-control-plane" tee /tmp/members.config > /dev/null

docker exec "${HOST_CLUSTER_NAME}-control-plane" /usr/local/bin/karmadactl join "${MEMBER_CLUSTER_NAME}" \
  --kubeconfig="/tmp/karmada.config" \
  --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" \
  --cluster-kubeconfig="/tmp/members.config" \
  --cluster-context="${MEMBER_CLUSTER_NAME}"

# Fix local context for WSL access
kubectl config set-cluster "karmada-apiserver" --server="https://127.0.0.1:5443" --insecure-skip-tls-verify=true --kubeconfig="${MAIN_KUBECONFIG}"

# Step 4: Verify setup
echo "Verifying setup..."
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_NAME}"

# Step 5: Run sample Nginx workload
echo "Running sample Nginx workload..."
cat <<EOF | kubectl --context="${KARMADA_APISERVER_CLUSTER_NAME}" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-sample
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx-sample
  placement:
    clusterAffinity:
      clusterNames:
        - ${MEMBER_CLUSTER_NAME}
EOF

echo "Waiting for Nginx to be scheduled..."
sleep 15
kubectl --context="${KARMADA_APISERVER_CLUSTER_NAME}" get deployment nginx-sample

echo ""
echo "================================================================================"
echo "      FAST LOCAL KARMADA SETUP COMPLETE"
echo "================================================================================"
echo "Karmada Control Plane: karmada-apiserver (localhost:5443)"
echo "Member cluster: ${MEMBER_CLUSTER_NAME} (Ready)"
echo "Sample Nginx workload: Running"
echo ""
echo "Verification Commands (Run from WSL):"
echo "  export KUBECONFIG=${MAIN_KUBECONFIG}"
echo "  kubectl config use-context ${KARMADA_APISERVER_CLUSTER_NAME}"
echo "  kubectl get clusters"
echo "  kubectl get deployment nginx-sample"
echo "  karmadactl get pods -l app=nginx"
echo ""
echo "To Stop the environment:"
echo "  kind delete clusters ${HOST_CLUSTER_NAME} ${MEMBER_CLUSTER_NAME}"
echo "================================================================================"
