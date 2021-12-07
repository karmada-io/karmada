#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${SCRIPT_ROOT}"
ROOT_PATH=$(pwd)

GO111MODULE=on go install "golang.org/x/tools/cmd/goimports@v0.1.5"

IMPORT_ALIASES_PATH="${ROOT_PATH}/hack/.import-aliases"
INCLUDE_PATH="(${ROOT_PATH}/cmd|${ROOT_PATH}/test/e2e|${ROOT_PATH}/test/helper|\
${ROOT_PATH}/pkg/clusterdiscovery|${ROOT_PATH}/pkg/controllers|\
${ROOT_PATH}/pkg/estimator/server|${ROOT_PATH}/pkg/karmadactl|${ROOT_PATH}/pkg/scheduler|\
${ROOT_PATH}/pkg/util|${ROOT_PATH}/pkg/version|${ROOT_PATH}/pkg/webhook)"

# We can't directly install preferredimports by `go install` due to the go.mod issue:
# go install k8s.io/kubernetes/cmd/preferredimports@v1.21.3: k8s.io/kubernetes@v1.21.3
#   The go.mod file for the module providing named packages contains one or
#   more replace directives. It must not contain directives that would cause
#   it to be interpreted differently than if it were the main module.
go run "${ROOT_PATH}/hack/tools/preferredimports/preferredimports.go" -confirm -import-aliases "${IMPORT_ALIASES_PATH}" -include-path "${INCLUDE_PATH}"  "${ROOT_PATH}"

len=${#INCLUDE_PATH}
INCLUDE_PATH=${INCLUDE_PATH:1:len-2}

IFS="|" read -r -a array <<< "${INCLUDE_PATH}"
for var in "${array[@]}"
do
   echo "Sorting importing in file" "${var}"
   goimports -local "github.com/karmada-io/karmada" -w "${var}"
done
