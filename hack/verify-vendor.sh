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

DIFFROOT="${SCRIPT_ROOT}/vendor"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/vendor"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"
cp "${SCRIPT_ROOT}"/go.mod "$_tmp"/go.mod
cp "${SCRIPT_ROOT}"/go.sum "$_tmp"/go.sum

bash "${SCRIPT_ROOT}/hack/update-vendor.sh"
echo "diffing ${DIFFROOT} against freshly generated files"

govendor=0
diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || govendor=$?
gomod=0
diff -Naupr "${SCRIPT_ROOT}"/go.mod "${_tmp}"/go.mod || gomod=$?
gosum=0
diff -Naupr "${SCRIPT_ROOT}"/go.sum "${_tmp}"/go.sum || gosum=$?

cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
cp "${_tmp}"/go.mod "${SCRIPT_ROOT}"/go.mod
cp "${_tmp}"/go.sum "${SCRIPT_ROOT}"/go.sum
if [[ $govendor -eq 0 && $gomod -eq 0 && $gosum -eq 0 ]]
then
  echo "${DIFFROOT} up to date."
else
  echo "${DIFFROOT}, 'go.mod' or 'go.sum' is out of date. Please run hack/update-vendor.sh"
  exit 1
fi
