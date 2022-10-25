#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
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

# This script generates mock files using mockgen.
# Usage: `hack/update-mocks.sh`.

set -o errexit
set -o nounset
set -o pipefail

KARMADA_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# Explicitly opt into go modules, even though we're inside a GOPATH directory
export GO111MODULE=on
export PATH=$PATH:$(go env GOPATH)/bin

trap EXIT

echo 'installing mockgen'
source "${KARMADA_ROOT}"/hack/util.sh
echo -n "Preparing: 'mockgen' existence check - "
if [ ! $(util::cmd_exist mockgen) ]; then
  GO111MODULE=on go install "github.com/golang/mock/mockgen@v1.6.0"
fi

find_files() {
  find . -not \( \
      \( \
        -wholename './output' \
        -o -wholename './.git' \
        -o -wholename './_output' \
        -o -wholename './_gopath' \
        -o -wholename './release' \
        -o -wholename './target' \
        -o -wholename '*/third_party/*' \
        -o -wholename '*/vendor/*' \
        -o -wholename './staging/src/k8s.io/client-go/*vendor/*' \
        -o -wholename '*/bindata.go' \
	-o -wholename '*/_tmp/*' \
      \) -prune \
    \) -name '*.go'
}

cd "${KARMADA_ROOT}"

echo 'executing go generate command on below files'
for IFILE in $(find_files | xargs grep --files-with-matches -e '//go:generate mockgen'); do
  go generate -v "$IFILE"
done
