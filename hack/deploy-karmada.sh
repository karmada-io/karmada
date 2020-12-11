#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script will deploy karmada control plane to a cluster."
  echo "Usage: hack/deploy-karmada.sh"
  echo "Example: hack/deploy-karmada.sh"
}

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# create namespace for control plane components
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/namespace.yaml"

# create service account, cluster role for controller-manager
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

# install APIs
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/membercluster.karmada.io_memberclusters.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationpolicies.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationbindings.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationworks.yaml"

# create secret for controller-manager
kubectl create secret generic kubeconfig --from-file=kubeconfig="${KUBECONFIG}" -n karmada-system

# deploy controller-manager
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/controller-manager.yaml"
