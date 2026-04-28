#!/bin/bash
# Copyright 2021 The Karmada Authors.
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

KARMADA_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
_tmp="${KARMADA_ROOT}/_tmp"
ESTIMATORPB="pkg/estimator/pb"

cleanup() {
  rm -rf "${_tmp}"
}

trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${_tmp}/${ESTIMATORPB}"

cp -a "${KARMADA_ROOT}/${ESTIMATORPB}"/* "${_tmp}/${ESTIMATORPB}/"

source "$KARMADA_ROOT/hack/update-estimator-protobuf.sh"

echo "diffing ${ESTIMATORPB} against freshly generated estimator protobuf"
ret=0
diff -Naupr "${KARMADA_ROOT}/${ESTIMATORPB}" "${_tmp}/${ESTIMATORPB}" || ret=$?
cp -a "${_tmp}/${ESTIMATORPB}"/* "${KARMADA_ROOT}/${ESTIMATORPB}/"
if [[ $ret -eq 0 ]]; then
  echo "${ESTIMATORPB} is up to date."
else
  echo "${ESTIMATORPB} is out of date. Please run hack/update-estimator-protobuf.sh to update."
  exit 1
fi
