#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${SCRIPT_ROOT}"
ROOT_PATH=$(pwd)

temp_path=$(mktemp -d)
pushd "${temp_path}" >/dev/null
cp "${ROOT_PATH}"/go.mod .
GO111MODULE=on go get "k8s.io/kubernetes/cmd/preferredimports@v1.21.3"
GO111MODULE=on go get "golang.org/x/tools/cmd/goimports@v0.1.5"
popd >/dev/null

IMPORT_ALIASES_PATH="${ROOT_PATH}/hack/.import-aliases"
INCLUDE_PATH="(${ROOT_PATH}/cmd|${ROOT_PATH}/test/e2e|${ROOT_PATH}/test/helper|\
${ROOT_PATH}/pkg/clusterdiscovery|${ROOT_PATH}/pkg/controllers|\
${ROOT_PATH}/pkg/estimator/server|${ROOT_PATH}/pkg/karmadactl|${ROOT_PATH}/pkg/scheduler|\
${ROOT_PATH}/pkg/util|${ROOT_PATH}/pkg/version|${ROOT_PATH}/pkg/webhook)"

preferredimports -confirm -import-aliases "${IMPORT_ALIASES_PATH}" -include-path "${INCLUDE_PATH}"  "${ROOT_PATH}"

len=${#INCLUDE_PATH}
INCLUDE_PATH=${INCLUDE_PATH:1:len-2}

IFS="|" read -r -a array <<< "${INCLUDE_PATH}"
for var in "${array[@]}"
do
   echo "Sorting importing in file" "${var}"
   goimports -local "github.com/karmada-io/karmada" -w "${var}"
done

