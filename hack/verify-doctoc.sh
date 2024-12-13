#!/usr/bin/env bash
# Copyright 2024 The Karmada Authors.
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

DIFFDOCTOCDIR="${SCRIPT_ROOT}/docs/CHANGELOG"
TMP_DIFFDOCTOCDIR="${SCRIPT_ROOT}/_tmp/docs/CHANGELOG"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFDOCTOCDIR}"
cp -a "${DIFFDOCTOCDIR}"/* "${TMP_DIFFDOCTOCDIR}"

bash "${SCRIPT_ROOT}/hack/update-doctoc.sh"
echo "diffing ${DIFFDOCTOCDIR} against freshly generated files"

ret=0

diff -Naupr "${DIFFDOCTOCDIR}" "${TMP_DIFFDOCTOCDIR}" || ret=$?
cp -a "${TMP_DIFFDOCTOCDIR}"/* "${DIFFDOCTOCDIR}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFDOCTOCDIR} up to date."
else
  echo "${DIFFDOCTOCDIR} is out of date. Please run hack/update-doctoc.sh"
  exit 1
fi
