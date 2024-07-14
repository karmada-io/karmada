#!/usr/bin/env bash
# Copyright 2020 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

DIFFROOT="${SCRIPT_ROOT}/pkg"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/pkg"
DIFFEXAMPLES="${SCRIPT_ROOT}/examples/customresourceinterpreter/apis"
TMP_DIFFEXAMPLES="${SCRIPT_ROOT}/_tmp/examples/customresourceinterpreter/apis"
DIFFOPERATOR="${SCRIPT_ROOT}/operator/pkg/apis/operator"
TMP_DIFFOPERATOR="${SCRIPT_ROOT}/_tmp/operator/pkg/apis/operator"
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

bash "${SCRIPT_ROOT}/hack/update-codegen.sh"
echo "diffing ${DIFFROOT} against freshly generated codegen"
ret=0
diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFROOT} up to date."
else
  echo "${DIFFROOT} is out of date. Please run hack/update-codegen.sh"
  exit 1
fi

echo "diffing ${DIFFEXAMPLES} against freshly generated codegen"
ret=0
diff -Naupr "${DIFFEXAMPLES}" "${TMP_DIFFEXAMPLES}" || ret=$?
cp -a "${TMP_DIFFEXAMPLES}"/* "${DIFFEXAMPLES}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFEXAMPLES} up to date."
else
  echo "${DIFFEXAMPLES} is out of date. Please run hack/update-codegen.sh"
  exit 1
fi

echo "diffing ${DIFFOPERATOR} against freshly generated codegen"
ret=0
diff -Naupr "${DIFFOPERATOR}" "${TMP_DIFFOPERATOR}" || ret=$?
cp -a "${TMP_DIFFOPERATOR}"/* "${DIFFOPERATOR}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFOPERATOR} up to date."
else
  echo "${DIFFOPERATOR} is out of date. Please run hack/update-codegen.sh"
  exit 1
fi
