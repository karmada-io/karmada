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

function usage() {
  echo "Usage: $0 <owner> <repository> <base-ref> <head-ref>" >&2
}

if [[ $# -ne 4 ]]; then
  usage
  exit 2
fi

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "Error: GITHUB_TOKEN is not set" >&2
  exit 2
fi

for command in curl jq; do
  if ! command -v "${command}" > /dev/null 2>&1; then
    echo "Error: required command '${command}' was not found" >&2
    exit 2
  fi
done

repo_owner=$1
repo_name=$2
base_ref=$3
head_ref=$4
encoded_base_ref=$(printf '%s' "${base_ref}" | jq -sRr @uri)
encoded_head_ref=$(printf '%s' "${head_ref}" | jq -sRr @uri)

temp_dir=$(mktemp -d)
trap 'rm -rf "${temp_dir}"' EXIT
authors_file="${temp_dir}/authors"
touch "${authors_file}"
unresolved_commits_file="${temp_dir}/unresolved-commits"
touch "${unresolved_commits_file}"

page=1
fetched_commits=0
total_commits=1

while (( fetched_commits < total_commits )); do
  url="https://api.github.com/repos/${repo_owner}/${repo_name}/compare/${encoded_base_ref}...${encoded_head_ref}?per_page=100&page=${page}"
  response_file="${temp_dir}/response-${page}.json"

  curl --fail --silent --show-error --location \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "${url}" > "${response_file}"

  total_commits=$(jq -er '.total_commits | numbers' "${response_file}")
  page_commits=$(jq -er '.commits | length' "${response_file}")
  jq -r '.commits[].author.login // empty' "${response_file}" >> "${authors_file}"
  jq -r '
    .commits[]
    | select(.author.login == null)
    | (.commit.message | split("\n")[0]) as $title
    | [
        .sha,
        if ($title | test("^Merge pull request #[0-9]+\\b")) then
          ($title | capture("^Merge pull request #(?<number>[0-9]+)\\b").number)
        elif ($title | test("\\(#[0-9]+\\)$")) then
          ($title | capture("\\(#(?<number>[0-9]+)\\)$").number)
        else
          ""
        end
      ]
    | @tsv
  ' \
    "${response_file}" >> "${unresolved_commits_file}"

  if (( page_commits == 0 )); then
    break
  fi

  fetched_commits=$((fetched_commits + page_commits))
  page=$((page + 1))
done

failed_commits_file="${temp_dir}/failed-commits"
touch "${failed_commits_file}"
while IFS=$'\t' read -r commit_sha pr_number; do
  [[ -n "${commit_sha}" ]] || continue
  if [[ -z "${pr_number}" ]]; then
    printf '%s\n' "${commit_sha}" >> "${failed_commits_file}"
    continue
  fi
  url="https://api.github.com/repos/${repo_owner}/${repo_name}/pulls/${pr_number}"
  response_file="${temp_dir}/pull-${pr_number}.json"

  if ! curl --fail --silent --show-error --location \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "${url}" > "${response_file}"; then
    echo "Error: failed to fetch PR #${pr_number} for commit ${commit_sha}" >&2
    exit 1
  fi

  if login=$(jq -er --arg sha "${commit_sha}" '
    if .merged_at != null
      and ((.head.sha // "") == $sha or (.merge_commit_sha // "") == $sha)
      and (.user.login // "") != ""
      then .user.login
      else empty
    end
  ' "${response_file}"); then
    printf '%s\n' "${login}" >> "${authors_file}"
  else
    printf '%s\n' "${commit_sha}" >> "${failed_commits_file}"
  fi
done < "${unresolved_commits_file}"

if [[ -s "${failed_commits_file}" ]]; then
  echo "Error: unable to resolve contributors for these commits:" >&2
  while IFS= read -r commit_sha; do
    echo "  ${commit_sha}" >&2
  done < "${failed_commits_file}"
  exit 1
fi

LC_ALL=C sort -fu "${authors_file}"
