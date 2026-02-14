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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
OUTPUT_DIR="${OUTPUT_DIR:-${REPO_ROOT}/performance-results}"

# variable define
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
export PROMETHEUS_PORT=${PROMETHEUS_PORT:-31801}
export PROMETHEUS_ENDPOINT="http://localhost:${PROMETHEUS_PORT}"

start_time=""
end_time=""

mkdir -p "${OUTPUT_DIR}"

function run_performance_test() {
    echo "start to run ClusterLoader2 performance test"
    
    rm -rf perf-tests
    git clone https://github.com/kubernetes/perf-tests.git

    cp "${REPO_ROOT}/hack/performance-env/clusterloader2/config.yaml" "${REPO_ROOT}/perf-tests/clusterloader2/config.yaml"
    cp "${REPO_ROOT}/hack/performance-env/clusterloader2/deployment.yaml" "${REPO_ROOT}/perf-tests/clusterloader2/deployment.yaml"
    cp "${REPO_ROOT}/hack/performance-env/clusterloader2/policy.yaml" "${REPO_ROOT}/perf-tests/clusterloader2/policy.yaml"
    cp "${REPO_ROOT}/hack/performance-env/clusterloader2/configmap.yaml" "${REPO_ROOT}/perf-tests/clusterloader2/configmap.yaml"

    cd "${REPO_ROOT}/perf-tests/clusterloader2"
    go run cmd/clusterloader.go --testconfig=config.yaml --provider=local --kubeconfig=$MAIN_KUBECONFIG --k8s-clients-number=1 --skip-cluster-verification=true --enable-exec-service=false
    cd ../..
    rm -rf perf-tests
}

function start_prometheus_forward() {
    echo "start to forward Prometheus port"
    
    # check if port forwarding is already running
    if pgrep -f "port-forward.*prometheus" > /dev/null; then
        echo "Prometheus port forwarding is already running"
    else
        kubectl --context="${HOST_CLUSTER_NAME}" port-forward -n monitor svc/prometheus --address 0.0.0.0 ${PROMETHEUS_PORT}:9090 > /dev/null 2>&1 &
        local forward_pid=$!
        echo ${forward_pid} > "${OUTPUT_DIR}/prometheus-forward-${PROMETHEUS_PORT}.pid"
        sleep 5
        echo "✅ Prometheus port forwarding is started (PID: ${forward_pid})"
    fi
    
    # verify connection
    if curl -s ${PROMETHEUS_ENDPOINT}/-/ready > /dev/null; then
        echo "✅ Prometheus is accessible: ${PROMETHEUS_ENDPOINT}"
    else
        echo "Warning: Prometheus is temporarily inaccessible, some metrics may not be collected"
    fi
}

trap cleanup EXIT

function cleanup() {
    echo "stop to forward Prometheus port"
    if [ -f "${OUTPUT_DIR}/prometheus-forward-${PROMETHEUS_PORT}.pid" ]; then
        local pid=$(cat "${OUTPUT_DIR}/prometheus-forward-${PROMETHEUS_PORT}.pid")
        if kill -0 ${pid} 2>/dev/null; then
            kill ${pid} 2>/dev/null || true
            echo "✅ Prometheus port forwarding is stopped"
        fi
        rm "${OUTPUT_DIR}/prometheus-forward-${PROMETHEUS_PORT}.pid"
    fi

    rm -rf "${OUTPUT_DIR}"
}

export KUBECONFIG=${MAIN_KUBECONFIG}

start_prometheus_forward

start_time=$(date +%s)
run_performance_test
sleep 5m
end_time=$(date +%s)

export START_TIME=${start_time}
export END_TIME=${end_time}
export STEP=${STEP:-15s}
"${REPO_ROOT}"/hack/performance-env/collect-metrics.sh

echo "✅ performance test is completed"
