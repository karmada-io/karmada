#!/usr/bin/env bash

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

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/
TESTING_RESULT=$?

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

exit $TESTING_RESULT
