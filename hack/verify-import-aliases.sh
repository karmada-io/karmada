#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${SCRIPT_ROOT}"
ROOT_PATH=$(pwd)

temp_path=$(mktemp -d)
pushd "${temp_path}" >/dev/null
cat > go.mod <<EOF
module github.com/karmada-io/karmada

go 1.16

replace (
	k8s.io/api => k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.2
	k8s.io/apiserver => k8s.io/apiserver v0.22.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.2
	k8s.io/client-go => k8s.io/client-go v0.22.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.2
	k8s.io/code-generator => k8s.io/code-generator v0.22.2
	k8s.io/component-base => k8s.io/component-base v0.22.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.2
	k8s.io/cri-api => k8s.io/cri-api v0.22.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.2
	k8s.io/kubectl => k8s.io/kubectl v0.22.2
	k8s.io/kubelet => k8s.io/kubelet v0.22.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.2
	k8s.io/metrics => k8s.io/metrics v0.22.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.2
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.2
)
EOF
GO111MODULE=on go get "k8s.io/kubernetes/cmd/preferredimports@v1.21.3"
popd >/dev/null

IMPORT_ALIASES_PATH="${ROOT_PATH}/hack/.import-aliases"
INCLUDE_PATH="(${ROOT_PATH}/cmd|${ROOT_PATH}/test/e2e|${ROOT_PATH}/test/helper|\
${ROOT_PATH}/pkg/clusterdiscovery|${ROOT_PATH}/pkg/controllers|\
${ROOT_PATH}/pkg/estimator/server|${ROOT_PATH}/pkg/karmadactl|${ROOT_PATH}/pkg/scheduler|\
${ROOT_PATH}/pkg/util|${ROOT_PATH}/pkg/version|${ROOT_PATH}/pkg/webhook)"

ret=0
preferredimports -import-aliases "${IMPORT_ALIASES_PATH}" -include-path "${INCLUDE_PATH}"  "${ROOT_PATH}" || ret=$?
if [[ $ret -ne 0 ]]; then
  echo "!!! Please see hack/.import-aliases for the preferred aliases for imports." >&2
  exit 1
fi
