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

# check if doctoc is installed.
if ! command -v doctoc &> /dev/null
then
    echo "doctoc is not installed, downloading now..."
    npm install -g doctoc

    if [ $? -ne 0 ]; then
        echo "doctoc installation failed, check your network connection or npm configuration."
        exit 1
    fi

    echo "doctoc has been successfully installed."
fi

# updating all relevant CHANGELOG files using doctoc.
doctoc ${DIFFDOCTOCDIR}
