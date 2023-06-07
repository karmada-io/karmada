#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

DIFFROOT="${SCRIPT_ROOT}/charts/karmada/_crds/bases"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/charts/karmada/_crds/bases"
DIFFEXAMPLES="${SCRIPT_ROOT}/examples/customresourceinterpreter/apis"
TMP_DIFFEXAMPLES="${SCRIPT_ROOT}/_tmp/examples/customresourceinterpreter/apis"
DIFFOPERATOR="${SCRIPT_ROOT}/charts/karmada-operator/crds"
TMP_DIFFOPERATOR="${SCRIPT_ROOT}/_tmp/charts/karmada-operator/crds"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"

mkdir -p "${TMP_DIFFEXAMPLES}"
cp -a "${DIFFEXAMPLES}"/* "${TMP_DIFFEXAMPLES}"

mkdir -p "${TMP_DIFFOPERATOR}"
cp -a "${DIFFOPERATOR}"/* "${TMP_DIFFOPERATOR}"

bash "${SCRIPT_ROOT}/hack/update-crdgen.sh"
echo "diffing ${DIFFROOT} against freshly generated files"

ret=0

diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFROOT} up to date."
else
  echo "${DIFFROOT} is out of date. Please run hack/update-crdgen.sh"
  exit 1
fi

echo "diffing ${DIFFEXAMPLES} against freshly generated files"

ret=0

diff -Naupr "${DIFFEXAMPLES}" "${TMP_DIFFEXAMPLES}" || ret=$?
cp -a "${TMP_DIFFEXAMPLES}"/* "${DIFFEXAMPLES}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFEXAMPLES} up to date."
else
  echo "${DIFFEXAMPLES} is out of date. Please run hack/update-crdgen.sh"
  exit 1
fi

diff -Naupr "${DIFFOPERATOR}" "${TMP_DIFFOPERATOR}" || ret=$?
cp -a "${TMP_DIFFOPERATOR}"/* "${DIFFOPERATOR}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFOPERATOR} up to date."
else
  echo "${DIFFOPERATOR} is out of date. Please run hack/update-crdgen.sh"
  exit 1
fi

