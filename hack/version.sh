#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KARMADA_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "${KARMADA_ROOT}/hack/util.sh"

util::get_version
