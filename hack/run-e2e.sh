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

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
KARMADA_APISERVER_KUBECONFIG=${KARMADA_APISERVER_KUBECONFIG:-"$KUBECONFIG_PATH/karmada.config"}
PULL_BASED_CLUSTERS=${PULL_BASED_CLUSTERS:-"member3:$KUBECONFIG_PATH/members.config"}

# KARMADA_RUNNING_ON_KIND indicates if current testing against on karmada that installed on a kind cluster.
# Defaults to true.
# For kind cluster, the kind related logs will be collected after the testing.
KARMADA_RUNNING_ON_KIND=${KARMADA_RUNNING_ON_KIND:-true}

KARMADA_HOST_CLUSTER_NAME=${KARMADA_HOST_CLUSTER_NAME:-"karmada-host"}
KARMADA_PULL_CLUSTER_NAME=${KARMADA_PULL_CLUSTER_NAME:-"member3"}

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

TEST_RESULT="PASS"
E2E_FAILED=false

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/suites/base -- --karmada-context=karmada-apiserver
TESTING_RESULT=$?

# Check if karmada-controller-manager has restarted, if it has, it means that OOM or panic has occurred
# due to memory modification, and needs to be investigated.
if [ $TESTING_RESULT -eq 0 ]; then
  echo "E2E run successfully."
  echo "Checking if karmada-controller-manager has restarted..."
  POD_NAMES=($(kubectl --context="${KARMADA_HOST_CLUSTER_NAME}" get pod -n karmada-system -l app=karmada-controller-manager -o=jsonpath='{.items[*].metadata.name}'))
  RESTART_COUNTS=($(kubectl --context="${KARMADA_HOST_CLUSTER_NAME}" get pod -n karmada-system -l app=karmada-controller-manager -o=jsonpath='{.items[*].status.containerStatuses[0].restartCount}'))
  for i in "${!POD_NAMES[@]}"; do
    POD_NAME="${POD_NAMES[$i]}"
    RESTART_COUNT="${RESTART_COUNTS[$i]}"
    if [ "$RESTART_COUNT" -gt 0 ]; then
      echo "WARNING: karmada-controller-manager pod $POD_NAME has restarted $RESTART_COUNT times."
      TEST_RESULT="FAIL"
    fi
  done
else
  echo "E2E run failed."
  TEST_RESULT="FAIL"
  E2E_FAILED=true
fi

# Collect logs
echo "Collect logs to $ARTIFACTS_PATH..."
cp "$KARMADA_APISERVER_KUBECONFIG" "$ARTIFACTS_PATH"

if [ "$KARMADA_RUNNING_ON_KIND" = true ]; then
  echo "Collecting $KARMADA_HOST_CLUSTER_NAME logs..."
  mkdir -p "$ARTIFACTS_PATH/$KARMADA_HOST_CLUSTER_NAME"
  kind export logs --name="$KARMADA_HOST_CLUSTER_NAME" "$ARTIFACTS_PATH/$KARMADA_HOST_CLUSTER_NAME"

  echo "Collecting $KARMADA_PULL_CLUSTER_NAME logs..."
  mkdir -p "$ARTIFACTS_PATH/KARMADA_PULL_CLUSTER_NAME"
  kind export logs --name="$KARMADA_PULL_CLUSTER_NAME" "$ARTIFACTS_PATH/$KARMADA_PULL_CLUSTER_NAME"
fi

echo "Collected logs at $ARTIFACTS_PATH:"
ls -al "$ARTIFACTS_PATH"

# Post run e2e for delete extra components
"${REPO_ROOT}"/hack/post-run-e2e.sh

if [ "$TEST_RESULT" == "FAIL" ]; then
    if [ "${E2E_FAILED:-false}" = true ]; then
        exit $TESTING_RESULT
    else
        exit 1
    fi
else
    exit 0
fi
