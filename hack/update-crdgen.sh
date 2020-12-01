#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

controller-gen crd paths=./pkg/apis/... output:crd:dir=./artifacts/deploy
