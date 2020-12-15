#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script starts a local karmada control plane based on current codebase and with a certain number of clusters joined.
# This script intended used in following scenarios:
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to setup test environment automatically.

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
KARMADA_APISERVER_KUBECONFIG=${KARMADA_APISERVER_KUBECONFIG:-"/var/run/karmada"}

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

export KUBECONFIG_PATH="${KUBECONFIG_PATH}"
"${REPO_ROOT}"/hack/local-up-karmada.sh

export KUBECONFIG="${KARMADA_APISERVER_KUBECONFIG}/karmada-apiserver.config"

# Install karmadactl
GO111MODULE=on go install "github.com/karmada-io/karmada/cmd/karmadactl"

# create clusters and join
MEMBER_CLUSTER_1_KUBECONFIG="${KUBECONFIG_PATH}/member1.config"
"${REPO_ROOT}"/hack/create-cluster.sh member1 "${MEMBER_CLUSTER_1_KUBECONFIG}"
karmadactl join member1 --member-cluster-kubeconfig="${MEMBER_CLUSTER_1_KUBECONFIG}"
MEMBER_CLUSTER_2_KUBECONFIG="${KUBECONFIG_PATH}/member2.config"
"${REPO_ROOT}"/hack/create-cluster.sh member2 "${MEMBER_CLUSTER_2_KUBECONFIG}"
karmadactl join member2 --member-cluster-kubeconfig="${MEMBER_CLUSTER_2_KUBECONFIG}"
