#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

DIFFFILE="${SCRIPT_ROOT}/pkg/util/lifted/doc.go"
TMP_DIFFFILE="${SCRIPT_ROOT}/_tmp/pkg/util/lifted/doc.go"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFFILE%/*}"
cp "${DIFFFILE}" "${TMP_DIFFFILE}"

bash "${SCRIPT_ROOT}/hack/update-lifted.sh"
echo "diffing ${DIFFFILE} against freshly generated files"

ret=0

diff -Naupr "${DIFFFILE}" "${TMP_DIFFFILE}" || ret=$?
cp "${TMP_DIFFFILE}" "${DIFFFILE}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFFILE} up to date."
else
  echo "${DIFFFILE} is out of date. Please run hack/update-lifted.sh"
  exit 1
fi
