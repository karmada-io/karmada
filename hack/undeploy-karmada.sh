#!/usr/bin/env bash

set +e
set -o nounset
set -o pipefail

function usage() {
  echo "This script will remove karmada control plane from a cluster."
  echo "Usage: hack/undeploy-karmada.sh"
  echo "Example: hack/undeploy-karmada.sh"
}

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# delete controller-manager
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/controller-manager.yaml"

# delete APIs
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/membercluster.karmada.io_memberclusters.yaml"
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationpolicies.yaml"
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationbindings.yaml"
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationworks.yaml"

# delete service account, cluster role
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

# delete namespace for control plane components
kubectl delete -f "${SCRIPT_ROOT}/artifacts/deploy/namespace.yaml"
