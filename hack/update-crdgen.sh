#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CONTROLLER_GEN_VERSION="v0.4.1"

# Install tools we need.
TEMP_PATH=$(mktemp -d)
pushd "${TEMP_PATH}" >/dev/null
  GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@"${CONTROLLER_GEN_VERSION}"
popd >/dev/null
rm -rf "${TEMP_PATH}"

controller-gen crd paths=./pkg/apis/... output:crd:dir=./artifacts/deploy
