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

# This script sets up member clusters for Karmada.
# It creates multiple Kind clusters with kwokctl.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
source "${REPO_ROOT}"/hack/util.sh

export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}

export CLUSTER_COUNT=${CLUSTER_COUNT:-10}
export NODES_PER_CLUSTER=${NODES_PER_CLUSTER:-10}
export CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}

function check_clusters_ready() {
  local kubeconfig_path=${1}
  local context_name=${2}

  echo "Waiting for kubeconfig file ${kubeconfig_path} and clusters ${context_name} to be ready..."
  util::wait_for_condition 'running' "docker inspect --format='{{.State.Status}}' ${context_name}-control-plane &> /dev/null" 300

  # delete the redundant context
  kubectl config delete-context kind-kwok-foo-${i}

  local os_name
  os_name=$(go env GOOS)
  local container_ip_port
  case $os_name in
    linux)
      if util::is_wsl2; then
        # WSL2 uses a different method to get the container IP address
        container_ip_port=$(util::get_docker_host_ip_port "${context_name}-control-plane")
      else # normal linux environment
        container_ip_port=$(util::get_docker_native_ipaddress "${context_name}-control-plane")":6443"
      fi
    ;;
    darwin) container_ip_port=$(util::get_docker_host_ip_port "${context_name}-control-plane")
    ;;
    *)
        echo "OS ${os_name} does NOT support for getting container ip in installation script"
        exit 1
  esac
  kubectl config set-cluster "kind-${context_name}" --server="https://${container_ip_port}" --kubeconfig="${kubeconfig_path}"

  util::wait_for_condition 'ok' "kubectl --kubeconfig ${kubeconfig_path} --context ${context_name} get --raw=/healthz &> /dev/null" 300
}

# step1. create the clusters
export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}

for i in $(seq 1 "${CLUSTER_COUNT}"); do 
  kwokctl delete cluster --name=foo-${i}
  kwokctl create cluster --name=foo-${i} --runtime=kind --kind-node-image=${CLUSTER_VERSION}

  check_clusters_ready "${KUBECONFIG}" "kwok-foo-${i}"
done

kwokctl get clusters

# step2. create the fake nodes in each cluster
for i in $(seq 1 "${CLUSTER_COUNT}"); do 
  echo "start to create ${NODES_PER_CLUSTER} nodes in cluster kwok-foo-${i}..."
  
  for j in $(seq 1 "${NODES_PER_CLUSTER}"); do
    # use kwok-node.yaml as template, dynamically generate nodes
    cat "${REPO_ROOT}/hack/performance-env/kwok-node.yaml" | \
      sed "s/{{name}}/kwok-node-${j}/" | \
      kubectl --context="kwok-foo-${i}" apply -f -
  done
  
  echo "all nodes in cluster kwok-foo-${i} are created"
done

echo "all nodes in all clusters are created, total $((CLUSTER_COUNT * NODES_PER_CLUSTER)) nodes"

# step3. join the clusters to the karmada control plane

export KUBECONFIG=${MAIN_KUBECONFIG}
kubectl config use-context karmada-apiserver

for i in $(seq 1 "${CLUSTER_COUNT}"); do 
  karmadactl join "kwok-foo-${i}" --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" --cluster-context="kwok-foo-${i}"
done
