#!/usr/bin/env bash

set -o errexit
set -o nounset

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"/var/run/karmada"}
KARMADA_APISERVER_CONFIG="${CERT_DIR}/karmada-apiserver.config"
# The host cluster name which used to install karmada control plane components.
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
HOST_CLUSTER_KUBECONFIG=${HOST_CLUSTER_KUBECONFIG:-"${HOME}/.kube/karmada-host.config"}

etcd_pod_label="etcd"
apiserver_pod_label="karmada-apiserver"
controller_pod_label="kube-controller-manager"

function usage() {
  echo "This script will deploy karmada control plane to a cluster."
  echo "Usage: hack/deploy-karmada.sh"
  echo "Example: hack/deploy-karmada.sh"
}

function waitPodReady() {
    local pod_label=$1
    local pod_namespace=$2

    echo "wait the $pod_label ready..."
    kubectl wait --for=condition=Ready --timeout=200s pods -l app=$pod_label -n $pod_namespace
}

function installCRDs() {
    if [ ! -f ${KARMADA_APISERVER_CONFIG} ]; then
        echo "Please provide kubeconfig to connect karmada apiserver"
        return 1
    fi

    # install APIs
    kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/namespace.yaml"
    kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/membercluster.karmada.io_memberclusters.yaml"
    kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationpolicies.yaml"
    kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationbindings.yaml"
    kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationworks.yaml"
    kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_overridepolicies.yaml"
}

export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"

# create namespace for control plane components
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/namespace.yaml"

# create service account, cluster role for controller-manager
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

#generate cert
"${SCRIPT_ROOT}"/hack/generate-cert.sh

# deploy karmada etcd
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/karmada-etcd.yaml"

# Wait for karmada-etcd to come up before launching the rest of the components.
waitPodReady $etcd_pod_label "karmada-system"

# deploy karmada apiserver
KARMADA_API_IP=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${HOST_CLUSTER_NAME}-control-plane")
cp -rf ${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver.yaml ${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver-tmp.yaml
sed -i "s/{{api_addr}}/${KARMADA_API_IP}/g" ${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver-tmp.yaml

kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver-tmp.yaml"

# Wait for karmada-apiserver to come up before launching the rest of the components.
waitPodReady $apiserver_pod_label "karmada-system"

# deploy kube controller manager
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/kube-controller-manager.yaml"

# Wait for karmada kube controller manager to come up before launching the rest of the components.
waitPodReady $controller_pod_label "karmada-system"

# install CRD APIs on karmada apiserver.
export KUBECONFIG=${KARMADA_APISERVER_CONFIG}
installCRDs

# deploy controller-manager on host cluster
export KUBECONFIG=${HOST_CLUSTER_KUBECONFIG}
kubectl apply -f "${SCRIPT_ROOT}/artifacts/deploy/controller-manager.yaml"
