#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

helm push _output/charts/karmada-chart-$1.tgz oci://docker.io/karmada
helm push _output/charts/karmada-operator-chart-$1.tgz oci://docker.io/karmada

util::signImage docker.io/karmada/karmada:$1
util::signImage docker.io/karmada/karmada-operator:$1
