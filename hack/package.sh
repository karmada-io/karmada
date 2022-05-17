#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KARMADA_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "${KARMADA_ROOT}/hack/util.sh"

function get_package_names() {
  local -a names=()
  for s in "${@}"; do
    names+=(${s%%=*})
  done
  echo "${names[@]}"
}

case "${1:-all}" in
server)
  get_package_names ${KARMADA_SERVER_PACKAGES[*]}
  ;;
ctl)
  get_package_names ${KARMADA_CTL_PACKAGES[*]}
  ;;
all)
  get_package_names ${KARMADA_SERVER_PACKAGES[*]} ${KARMADA_CTL_PACKAGES[*]}
  ;;
esac
