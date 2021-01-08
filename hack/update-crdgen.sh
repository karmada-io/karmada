#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CONTROLLER_GEN_PKG="sigs.k8s.io/controller-tools/cmd/controller-gen"
CONTROLLER_GEN_VER="v0.4.1"

source hack/util.sh

util::install_tools ${CONTROLLER_GEN_PKG} ${CONTROLLER_GEN_VER}

controller-gen crd paths=./pkg/apis/... output:crd:dir=./artifacts/deploy
