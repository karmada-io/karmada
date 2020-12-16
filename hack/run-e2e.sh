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
# Example 2: export CONTROL_PLANE_KUBECONFIG=<KUBECONFIG PATH> hack/run-e2e.sh (run e2e with your KUBECONFIG)

CONTROL_PLANE_KUBECONFIG=${CONTROL_PLANE_KUBECONFIG:-"${HOME}/.kube/karmada.config"}

export KUBECONFIG=${CONTROL_PLANE_KUBECONFIG}

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/ginkgo

# Run e2e
export KUBECONFIG=${CONTROL_PLANE_KUBECONFIG}
ginkgo -v -race -failFast ./test/e2e/
