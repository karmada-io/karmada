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

set -o errexit
set -o nounset
set -o pipefail

function usage() {
    echo "Usage:"
    echo "    hack/local-up-performance.sh [HOST_IPADDRESS] [-h]"
    echo "Args:"
    echo "    HOST_IPADDRESS: (optional) if you want to export clusters' API server port to specific IP address"
    echo "    h: print help information"
}

while getopts 'h' OPT; do
    case $OPT in
        h)
          usage
          exit 0
          ;;
        ?)
          usage
          exit 1
          ;;
    esac
done
shift $((OPTIND-1))

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
export KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
export MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
export HOST_IPADDRESS=${1:-}

# step0. install dependencies
function install_dependencies() {
    echo "Installing performance test dependencies..."

    # Install kwokctl
    echo "Installing kwokctl..."
    local kwokctl_version="v0.7.0"
    local kwokctl_path="/usr/local/bin/kwokctl"

    if command -v kwokctl &> /dev/null; then
        echo "✅ kwokctl is already installed"
    else
        local os_type
        os_type=$(uname -s | tr '[:upper:]' '[:lower:]')
        local arch
        arch=$(uname -m)
        case "$arch" in
            x86_64) arch="amd64" ;;
            aarch64) arch="arm64" ;;
        esac

        local binary_name="kwokctl-${os_type}-${arch}"
        local kwokctl_url="https://github.com/kubernetes-sigs/kwok/releases/download/${kwokctl_version}/${binary_name}"

        local tmp_dir
        tmp_dir=$(mktemp -d)
        local tmp_kwokctl="${tmp_dir}/${binary_name}"

        # Ensure tmp dir is cleaned up on exit
        trap 'rm -rf "${tmp_dir}"' RETURN

        wget -q "${kwokctl_url}" -O "${tmp_kwokctl}" || {
            echo "❌ Failed to download kwokctl from ${kwokctl_url}"
            return 1
        }

        chmod +x "${tmp_kwokctl}"
        sudo mv "${tmp_kwokctl}" "${kwokctl_path}" || {
            echo "❌ Failed to move kwokctl to ${kwokctl_path}"
            return 1
        }
        echo "✅ kwokctl installed successfully at ${kwokctl_path}"
    fi

    # Install system dependencies
    echo "Installing system dependencies (jq, bc)..."
    if command -v apt-get &> /dev/null; then
        sudo apt-get update
        sudo apt-get install -y jq bc
    elif command -v yum &> /dev/null; then
        sudo yum install -y jq bc
    elif command -v brew &> /dev/null; then
        brew install jq bc
    else
        echo "❌ Unable to find a supported package manager (apt-get, yum, or brew)"
        return 1
    fi
    echo "✅ System dependencies installed successfully"
}

install_dependencies

# step1. set up a base development environment
"${REPO_ROOT}"/hack/performance/setup-control-plane.sh
export KUBECONFIG="${MAIN_KUBECONFIG}"

# step2. install karmada control plane components
"${REPO_ROOT}"/hack/deploy-karmada.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# step3. set up member clusters
"${REPO_ROOT}"/hack/performance/setup-member-clusters.sh

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  echo -e "  export KUBECONFIG=${MAIN_KUBECONFIG}"
  echo "Please use 'kubectl config use-context ${HOST_CLUSTER_NAME}/${KARMADA_APISERVER_CLUSTER_NAME}' to switch the host and control plane cluster."
  echo -e "\nTo manage your member clusters, run:"
  echo -e "  export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context kwok-foo-i' to switch to the different member cluster, where i is the cluster number."
}

print_success

# step4. deploy prometheus
function deploy_prometheus() {
    kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/hack/performance/prometheus-rbac.yaml"
    kubectl --context="${KARMADA_APISERVER_CLUSTER_NAME}" apply -f "${REPO_ROOT}/hack/performance/prometheus-rbac.yaml"

    # Wait for prometheus service account token secret to be populated in the Karmada apiserver cluster
    local max_retries=30
    local retry_interval=2
    local karmada_token_base64=""
    for ((i=1; i<=max_retries; i++)); do
        karmada_token_base64=$(kubectl --context="${KARMADA_APISERVER_CLUSTER_NAME}" get secret prometheus -n monitor -o jsonpath='{.data.token}' 2>/dev/null || true)
        if [[ -n "${karmada_token_base64}" ]]; then
            break
        fi
        echo "Waiting for prometheus service account token to be populated in ${KARMADA_APISERVER_CLUSTER_NAME} cluster (${i}/${max_retries})..."
        sleep "${retry_interval}"
    done
    if [[ -z "${karmada_token_base64}" ]]; then
        echo "Failed to retrieve prometheus service account token from ${KARMADA_APISERVER_CLUSTER_NAME} cluster after $((max_retries * retry_interval)) seconds."
        return 1
    fi
    KARMADA_TOKEN=$(echo -n "${karmada_token_base64}" | base64 -d)

    # Create secret with token in host cluster
    kubectl --context="${HOST_CLUSTER_NAME}" create secret generic karmada-token --from-literal=token="${KARMADA_TOKEN}" -n monitor --dry-run=client -o yaml | kubectl --context="${HOST_CLUSTER_NAME}" apply -f -

    kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/hack/performance/prometheus-application.yaml"

    kubectl --context="${HOST_CLUSTER_NAME}" wait --for=condition=available --timeout=180s deployment/prometheus -n monitor
}

deploy_prometheus

echo "print pod status"
kubectl --context="${HOST_CLUSTER_NAME}" get pod -n monitor
kubectl --context="${HOST_CLUSTER_NAME}" get pod -n karmada-system
