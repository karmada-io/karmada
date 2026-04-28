#!/usr/bin/env bash
# Copyright 2026 The Karmada Authors.
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

DIFFROOT="${SCRIPT_ROOT}/docs/command-line-flags"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/docs/command-line-flags"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT SIGTERM

cleanup

mkdir -p "${DIFFROOT}" "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}/." "${TMP_DIFFROOT}"

bash "${SCRIPT_ROOT}/hack/update-command-line-flags.sh"
echo "diffing ${DIFFROOT} against freshly generated docs"

ret=0

diff -Naupr "${TMP_DIFFROOT}" "${DIFFROOT}" || ret=$?
rm -rf "${DIFFROOT:?}"/* && cp -a "${TMP_DIFFROOT}/." "${DIFFROOT}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFROOT} up to date."
else
  echo "${DIFFROOT} is out of date. Please run hack/update-command-line-flags.sh"
  exit 1
fi
