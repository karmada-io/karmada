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

# This script used to update helm index to specific release version and automatically submit a pr to remote repo.
# Usage:
#   export UPSTREAM_REMOTE=upstream GITHUB_USER=${YOUR_GITHUB_NAME} tag=v1.12.0
#   hack/update-chart-index.sh

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd ${REPO_ROOT}

UPSTREAM_REMOTE=${UPSTREAM_REMOTE:-origin}
UPSTREAM_REPO_ORG=${UPSTREAM_REPO_ORG:-$(git remote get-url "$UPSTREAM_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $3}')}
UPSTREAM_REPO_NAME=${UPSTREAM_REPO_NAME:-$(git remote get-url "$UPSTREAM_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $4}')}
GITHUB_USER=${GITHUB_USER:-${UPSTREAM_REPO_ORG}}

get_latest_release_tag() {
  curl --silent "https://api.github.com/repos/$1/releases/latest" |
    grep '"tag_name":' |
    sed -E 's/.*"([^"]+)".*/\1/'
}

# step1: get tag, defaults to latest release tag
tag=${tag:-"$(get_latest_release_tag "${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}")"}
if [ $(grep -c "version: ${tag}" charts/index.yaml) -ge '2' ]; then
  echo "the tag already in chart index!"
  exit 0
fi

# step2: checkout a new branch
NEWBRANCH="auto-chart-index-${tag}"
NEWBRANCHUNIQ="${NEWBRANCH}-$(date +%s)"
git checkout -b ${NEWBRANCHUNIQ}

# step3: update index for karmada-chart
wget https://github.com/${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}/releases/download/${tag}/karmada-chart-${tag}.tgz -P charts/karmada/
helm repo index charts/karmada --url https://github.com/${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}/releases/download/${tag} --merge charts/index.yaml
mv charts/karmada/index.yaml charts/index.yaml

# step4: update index for karmada-operator-chart
wget https://github.com/${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}/releases/download/${tag}/karmada-operator-chart-${tag}.tgz -P charts/karmada-operator/
helm repo index charts/karmada-operator --url https://github.com/${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}/releases/download/${tag} --merge charts/index.yaml
mv charts/karmada-operator/index.yaml charts/index.yaml

# step5: the `helm repo index` command also generates index for dependencies(common-2.x.x) by default,
# which is undesirable; therefore, the contents of the `common` field should be manaually removed.
# this `sed` command deletes lines between the line `entries:` and the line `karmada:`.
sed -i'' '/entries:/,/karmada:/{//!d}' charts/index.yaml
echo "Successfully generated chart index."

# step6: commit the modification
git add charts/index.yaml
git commit -s -m "Bump upgrade helm chart index to ${tag}"
git push origin -f "${NEWBRANCHUNIQ}:${NEWBRANCH}"
echo "Successfully pushed the commit."

# step6: create pull request
if [ $(gh search prs --repo="${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}" --head "${NEWBRANCH}" --base master | wc -l) -gt 0 ]; then
  echo "Skip create pr, because the pr already exists."
  exit 0
fi
prtext=$(
  cat <<EOF
**What type of PR is this?**

/kind cleanup

**What this PR does / why we need it**:

Bump upgrade helm chart index to ${tag}

**Which issue(s) this PR fixes**:

Fixes

**Does this PR introduce a user-facing change?**:
\`\`\`release-note
upgrade helm chart index to ${tag}.
\`\`\`
EOF
)
gh pr create --title "Bump upgrade helm chart index to ${tag}" --body "${prtext}" --repo="${UPSTREAM_REPO_ORG}/${UPSTREAM_REPO_NAME}" --head "${GITHUB_USER}":"${NEWBRANCH}" --base master
echo "Successfully created the pr."
