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
