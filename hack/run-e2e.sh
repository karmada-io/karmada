#!/usr/bin/env bash
# Copyright 2020 The Karmada Authors.
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

# This script runs e2e test against on karmada control plane.
# You should prepare your environment in advance and following environment may be you need to set or use default one.
# - CONTROL_PLANE_KUBECONFIG: absolute path of control plane KUBECONFIG file.
#
# Usage: hack/run-e2e.sh
# Example 1: hack/run-e2e.sh (run e2e with default config)
# Example 2: export KARMADA_APISERVER_KUBECONFIG=<KUBECONFIG PATH> hack/run-e2e.sh (run e2e with your KUBECONFIG)

# Cluster name definitions
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
KARMADA_APISERVER_KUBECONFIG=${KARMADA_APISERVER_KUBECONFIG:-"$KUBECONFIG_PATH/karmada.config"}
PULL_BASED_CLUSTERS=${PULL_BASED_CLUSTERS:-"${PULL_MODE_CLUSTER_NAME}:$KUBECONFIG_PATH/members.config"}

# KARMADA_RUNNING_ON_KIND indicates if current testing against on karmada that installed on a kind cluster.
# Defaults to true.
# For kind cluster, the kind related logs will be collected after the testing.
KARMADA_RUNNING_ON_KIND=${KARMADA_RUNNING_ON_KIND:-true}

KARMADA_HOST_CLUSTER_NAME=${KARMADA_HOST_CLUSTER_NAME:-"karmada-host"}

ARTIFACTS_PATH=${ARTIFACTS_PATH:-"${HOME}/karmada-e2e-logs"}
mkdir -p "$ARTIFACTS_PATH"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

# Pre run e2e for install extra components
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
"${REPO_ROOT}"/hack/pre-run-e2e.sh

# Run e2e
export KUBECONFIG=${KARMADA_APISERVER_KUBECONFIG}
export PULL_BASED_CLUSTERS=${PULL_BASED_CLUSTERS}

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/suites/base -- --karmada-context=karmada-apiserver
TESTING_RESULT=$?

# Collect logs
echo "Collect logs to $ARTIFACTS_PATH..."
cp "$KARMADA_APISERVER_KUBECONFIG" "$ARTIFACTS_PATH"

if [ "$KARMADA_RUNNING_ON_KIND" = true ]; then
  echo "Collecting $KARMADA_HOST_CLUSTER_NAME logs..."
  mkdir -p "$ARTIFACTS_PATH/$KARMADA_HOST_CLUSTER_NAME"
  kind export logs --name="$KARMADA_HOST_CLUSTER_NAME" "$ARTIFACTS_PATH/$KARMADA_HOST_CLUSTER_NAME"

  echo "Collecting $PULL_MODE_CLUSTER_NAME logs..."
  mkdir -p "$ARTIFACTS_PATH/$PULL_MODE_CLUSTER_NAME"
  kind export logs --name="$PULL_MODE_CLUSTER_NAME" "$ARTIFACTS_PATH/$PULL_MODE_CLUSTER_NAME"
fi

echo "Collected logs at $ARTIFACTS_PATH:"
ls -al "$ARTIFACTS_PATH"

# Post run e2e for delete extra components
"${REPO_ROOT}"/hack/post-run-e2e.sh

# If E2E test failed, exit directly with the test result
if [ $TESTING_RESULT -ne 0 ]; then
  echo "E2E test failed with exit code $TESTING_RESULT, skipping component restart check."
  exit $TESTING_RESULT
fi

# Check if Karmada components have restarted, if any has, it means that OOM or panic has occurred
# due to memory modification, and needs to be investigated.
echo "E2E run successfully."
echo "Checking if Karmada components have restarted..."

# Function to check pod restart count for a given component
check_component_restart() {
  local component_label=$1
  local component_name=$2
  
  echo "Checking ${component_name} pods..."
  
  # Get pod information in a single call, including both name and restart count
  # Use a template that handles missing containerStatuses gracefully
  local pod_info
  pod_info=$(kubectl --context="${KARMADA_HOST_CLUSTER_NAME}" get pod -n karmada-system -l "${component_label}" \
    -o go-template='{{range .items}}{{.metadata.name}}:{{if .status.containerStatuses}}{{(index .status.containerStatuses 0).restartCount}}{{else}}0{{end}}{{"\n"}}{{end}}' 2>/dev/null)
  
  if [ -z "$pod_info" ]; then
    echo "No pods found for ${component_name}, skipping..."
    return 0
  fi
  
  # Process each pod's information
  while IFS=: read -r pod_name restart_count; do
    # Skip empty lines
    [ -z "$pod_name" ] && continue
    
    # Ensure restart_count is a number (default to 0 if empty or invalid)
    if ! [[ "$restart_count" =~ ^[0-9]+$ ]]; then
      echo "Warning: Unable to get restart count for pod $pod_name, assuming 0"
      restart_count=0
    fi
    
    if [ "$restart_count" -gt 0 ]; then
      echo "ERROR: ${component_name} pod $pod_name has restarted $restart_count times."
      echo "This indicates OOM or panic occurred and needs to be investigated."
      
      # Collect logs from the previous container instance
      echo "Collecting logs from previous container instance for ${component_name} pod $pod_name..."
      local log_file="$ARTIFACTS_PATH/${component_name}-${pod_name}-previous-logs.log"
      if kubectl --context="${KARMADA_HOST_CLUSTER_NAME}" logs -p "$pod_name" -n karmada-system > "$log_file" 2>&1; then
        echo "Previous container logs saved to: $log_file"
      else
        echo "Warning: Failed to collect previous container logs for pod $pod_name. See details in $log_file"
      fi
      
      return 1  # Return failure to stop checking
    else
      echo "${component_name} pod $pod_name: no restarts"
    fi
  done <<< "$pod_info"
  
  return 0
}

# List of components to check (label=component_name)
components=(
  "app=karmada-controller-manager:karmada-controller-manager"
  "app=karmada-descheduler:karmada-descheduler"
  "app=karmada-metrics-adapter:karmada-metrics-adapter"
  "app=karmada-scheduler:karmada-scheduler"
  "app=karmada-search:karmada-search"
  "app=karmada-scheduler-estimator-${MEMBER_CLUSTER_1_NAME}:karmada-scheduler-estimator-${MEMBER_CLUSTER_1_NAME}"
  "app=karmada-scheduler-estimator-${MEMBER_CLUSTER_2_NAME}:karmada-scheduler-estimator-${MEMBER_CLUSTER_2_NAME}"
  "app=karmada-scheduler-estimator-${PULL_MODE_CLUSTER_NAME}:karmada-scheduler-estimator-${PULL_MODE_CLUSTER_NAME}"
)

# Check each component, stop at first failure
for component in "${components[@]}"; do
  IFS=':' read -r label name <<< "$component"
  if ! check_component_restart "$label" "$name"; then
    echo "COMPONENT RESTART CHECK FAILED: Component $name has restarted, stopping further checks."
    exit 1
  fi
done

echo "All component restart checks passed."
exit $TESTING_RESULT
