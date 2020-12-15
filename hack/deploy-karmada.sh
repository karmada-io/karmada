#!/usr/bin/env bash

set -o errexit
set -o nounset

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"/var/run/karmada"}
KARMADA_APISERVER_CONFIG="${CERT_DIR}/karmada-apiserver.config"
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
KARMADA_KUBECONFIG="${KUBECONFIG_PATH}/karmada.config"
etcd_replicas=1
etcd_pod_label="etcd"
apiserver_replicas=1
apiserver_pod_label="karmada-apiserver"
controller_replicas=1
controller_pod_label="kube-controller-manager"

function usage() {
  echo "This script will deploy karmada control plane to a cluster."
  echo "Usage: hack/deploy-karmada.sh"
  echo "Example: hack/deploy-karmada.sh"
}

function waitPodReady() {
    local pod_label=$1
    local pod_namespaces=$2
    local pod_replicas=$3

    timeout=200
    while [[ $timeout -gt 0 ]]; do
        echo "Waiting for $pod_label pods to become Ready"
        statuses=$(kubectl get pods -n $pod_namespaces -l app=$pod_label \
         -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' \
          | grep "True" | wc -w)
        if [[ $statuses -eq $pod_replicas ]]; then
            break
        else
          sleep 1
          (( timeout=timeout-1 ))
        fi
     done


    if [[ $timeout -gt 0 ]]; then
        echo "All $pod_label pods became Ready"
    else
        echo "ERROR: Not all $pod_label pods became Ready"
        echo "kubectl get pods -l app=$pod_label"
        kubectl get pods -l app=$pod_label
        exit 1
    fi
}

function installCRDs() {
    if [ ! -f ${KARMADA_APISERVER_CONFIG} ]; then
        echo "Please provide kubeconfig to connect karmada apiserver"
        return 1
    fi

    # install APIs
    kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/namespace.yaml"
    kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/membercluster.karmada.io_memberclusters.yaml"
    kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationpolicies.yaml"
    kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationbindings.yaml"
    kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/propagationstrategy.karmada.io_propagationworks.yaml"
}

# create namespace for control plane components
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/namespace.yaml"

# create service account, cluster role for controller-manager
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

#generate cert
"${SCRIPT_ROOT}"/hack/generate-cert.sh

# deploy karmada etcd
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/karmada-etcd.yaml"

# Wait for karmada-etcd to come up before launching the rest of the components.
waitPodReady $etcd_pod_label "karmada-system" $etcd_replicas

# deploy karmada apiserver
KARMADA_API_IP=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "karmada-control-plane")
cp -rf ${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver.yaml ${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver-tmp.yaml
sed -i "s/{{api_addr}}/${KARMADA_API_IP}/g" ${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver-tmp.yaml

kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/karmada-apiserver-tmp.yaml"

# Wait for karmada-apiserver to come up before launching the rest of the components.
waitPodReady $apiserver_pod_label "karmada-system" $apiserver_replicas

# deploy kube controller manager
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/kube-controller-manager.yaml"

# Wait for karmada kube controller manager to come up before launching the rest of the components.
waitPodReady $controller_pod_label "karmada-system" $controller_replicas

export KUBECONFIG=${KARMADA_APISERVER_CONFIG}

# install CRD APIs
installCRDs

export KUBECONFIG=${KARMADA_KUBECONFIG}

# deploy controller-manager
kubectl create -f "${SCRIPT_ROOT}/artifacts/deploy/controller-manager.yaml"
