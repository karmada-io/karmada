#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

DIFFROOT="${SCRIPT_ROOT}/artifacts/crds/bases"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/artifacts/crds/bases"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

ret=0
# check if crds in chart up to date.
CHART_CRD_PATH="${SCRIPT_ROOT}/charts/_crds"
diff -Naupr "${DIFFROOT}" "${CHART_CRD_PATH}" || ret=$?
if [[ $ret -eq 0 ]]
then
  echo "${CHART_CRD_PATH} up to date."
else
  echo "${CHART_CRD_PATH} is out of date. Please run hack/update-crdgen.sh"
  exit 1
fi

mkdir -p "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"

bash "${SCRIPT_ROOT}/hack/update-crdgen.sh"
echo "diffing ${DIFFROOT} against freshly generated files"

diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFROOT} up to date."
else
  echo "${DIFFROOT} is out of date. Please run hack/update-crdgen.sh"
  exit 1
fi
