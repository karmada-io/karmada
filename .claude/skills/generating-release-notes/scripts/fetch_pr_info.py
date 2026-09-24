#!/usr/bin/env python3
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

"""Fetch merged PR metadata and user-facing changes for release notes."""

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request


GITHUB_API_BASE = "https://api.github.com"
NO_RELEASE_NOTE_VALUES = {
    "",
    "n/a",
    "na",
    "no",
    "none",
    "noop",
    "nope",
    "nothing",
}


def github_request(url, token, payload=None):
    """Send an authenticated GitHub API request and decode its JSON body."""
    headers = {
        "Accept": "application/vnd.github.v3+json",
        "User-Agent": "Release-Note-Generator",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    data = None
    if payload is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(payload).encode("utf-8")

    request = urllib.request.Request(url, data=data, headers=headers)
    try:
        with urllib.request.urlopen(request) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        print(f"GitHub request failed ({error.code}): {detail}", file=sys.stderr)
    except urllib.error.URLError as error:
        print(f"GitHub request failed: {error.reason}", file=sys.stderr)
    return None


def get_commit_comparison(repo_owner, repo_name, base_tag, head_branch, token):
    """Get the GitHub comparison between the base tag and head branch."""
    base_url = (
        f"{GITHUB_API_BASE}/repos/{repo_owner}/{repo_name}/compare/"
        f"{base_tag}...{head_branch}"
    )
    comparison = None
    commits = []
    page = 1

    while True:
        response = github_request(f"{base_url}?per_page=100&page={page}", token)
        if not response:
            break
        if comparison is None:
            comparison = response.copy()

        page_commits = response.get("commits", [])
        commits.extend(page_commits)
        total_commits = comparison.get("total_commits", len(commits))
        if len(commits) >= total_commits or len(page_commits) < 100:
            break
        page += 1

    if comparison is not None:
        comparison["commits"] = commits
    return comparison


def get_pr_details_batch(repo_owner, repo_name, pr_numbers, token):
    """Fetch pull request title, body, and author in one GraphQL request."""
    if not pr_numbers:
        return {}

    query_parts = []
    for index, number in enumerate(pr_numbers):
        query_parts.append(
            f"pr{index}: pullRequest(number: {number}) {{ "
            "number title body author { login } "
            "labels(first: 100) { nodes { name } } }"
        )
    query = (
        "{ repository(owner: \""
        + repo_owner
        + "\", name: \""
        + repo_name
        + "\") { "
        + " ".join(query_parts)
        + " } }"
    )
    response = github_request(
        "https://api.github.com/graphql", token, {"query": query}
    )
    if not response:
        return None
    if response.get("errors"):
        print(f"GraphQL errors: {response['errors']}", file=sys.stderr)
        return None

    repository = response.get("data", {}).get("repository", {})
    result = {}
    for index, number in enumerate(pr_numbers):
        pull_request = repository.get(f"pr{index}")
        if pull_request:
            result[number] = pull_request
    return result


def normalize_release_note(content):
    """Normalize a candidate release note and reject standard empty values."""
    normalized = re.sub(r"\s+", " ", content.strip())
    if normalized.lower() in NO_RELEASE_NOTE_VALUES or len(normalized) <= 3:
        return None
    return normalized


def extract_user_facing_change(pr_body):
    """Extract a user-facing change from supported PR template formats."""
    if not pr_body:
        return None

    release_note_patterns = [
        r"```release-note[^\r\n]*[\r\n]+(.*?)^[ \t]*```[ \t]*$",
        r"```release-note\s*[\r\n]+([^\r\n]+(?:[\r\n]+[^\r\n`]+)*)",
    ]
    for pattern in release_note_patterns:
        match = re.search(pattern, pr_body, re.MULTILINE | re.DOTALL)
        if match:
            return normalize_release_note(match.group(1))

    fallback = re.search(
        r"\*\*Does this PR introduce a user-facing change\?\*\*:\s*"
        r"```[^\r\n]*[\r\n]+([\s\S]*?)```",
        pr_body,
        re.IGNORECASE,
    )
    if fallback:
        return normalize_release_note(fallback.group(1))
    return None


def extract_pr_kind_from_labels(labels):
    """Extract PR kinds from kind-prefixed labels."""
    kinds = []
    for label in (labels or {}).get("nodes", []):
        name = (label or {}).get("name", "")
        if name.startswith("kind/") and len(name) > len("kind/"):
            kind = name[len("kind/") :]
            if kind not in kinds:
                kinds.append(kind)
    return kinds or None


def extract_pr_kind_from_body(pr_body):
    """Extract PR kinds from the Karmada pull request template body."""
    if not pr_body:
        return None

    section = re.search(
        r"\*\*What type of PR is this\?\*\*(.*?)"
        r"\*\*What this PR does / why we need it\*\*:",
        pr_body,
        re.DOTALL | re.IGNORECASE,
    )
    if not section:
        return None

    without_comments = re.sub(r"<!--.*?-->", "", section.group(1), flags=re.DOTALL)
    kinds = re.findall(
        r"^\s*/kind\s+([a-zA-Z0-9-]+)\s*$", without_comments, re.MULTILINE
    )
    return kinds or None


def extract_pr_kind(labels, pr_body):
    """Extract PR kinds from labels, falling back to the PR body."""
    return extract_pr_kind_from_labels(labels) or extract_pr_kind_from_body(pr_body)


def extract_pr_number(commit_message):
    """Extract a PR number from a GitHub merge or squash commit title."""
    title = commit_message.splitlines()[0] if commit_message else ""
    merge_match = re.match(r"Merge pull request #(\d+)\b", title)
    if merge_match:
        return int(merge_match.group(1))

    squash_match = re.search(r"\(#(\d+)\)$", title)
    if squash_match:
        return int(squash_match.group(1))
    return None


def comparison_is_truncated(comparison):
    """Report whether the compare response omitted commits from its list."""
    commits = comparison.get("commits", [])
    return comparison.get("total_commits", len(commits)) > len(commits)


def parse_arguments():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="Fetch merged PR information for release note generation."
    )
    parser.add_argument("base_tag", help="Base tag or branch to compare from")
    parser.add_argument("head_branch", help="Head tag or branch to compare to")
    parser.add_argument(
        "--repo",
        required=True,
        help="GitHub repository in owner/name format",
    )
    return parser.parse_args()


def main():
    """Fetch and print PRs that contain user-facing changes."""
    args = parse_arguments()
    if args.repo.count("/") != 1:
        print("Error: --repo must use owner/name format", file=sys.stderr)
        return 2

    token = os.getenv("GITHUB_TOKEN")
    if not token:
        print("Error: GITHUB_TOKEN is not set", file=sys.stderr)
        return 2

    repo_owner, repo_name = args.repo.split("/", 1)
    print(f"Fetching PR information for {args.repo}...")
    print(f"Comparing {args.base_tag}...{args.head_branch}")
    print("=" * 80)

    comparison = get_commit_comparison(
        repo_owner, repo_name, args.base_tag, args.head_branch, token
    )
    if not comparison:
        print("Failed to get commit comparison", file=sys.stderr)
        return 1

    commits = comparison.get("commits", [])
    if comparison_is_truncated(comparison):
        print(
            "Error: GitHub returned only "
            f"{len(commits)} of {comparison['total_commits']} commits; "
            "refusing to generate incomplete release notes",
            file=sys.stderr,
        )
        return 1

    pr_numbers = set()
    for commit in commits:
        message = commit.get("commit", {}).get("message", "")
        pr_number = extract_pr_number(message)
        if pr_number is not None:
            pr_numbers.add(pr_number)

    sorted_pr_numbers = sorted(pr_numbers)
    print(f"Found {len(commits)} commits")
    print(f"Found {len(sorted_pr_numbers)} merged PRs")
    print("=" * 80)

    pr_details = get_pr_details_batch(
        repo_owner, repo_name, sorted_pr_numbers, token
    )
    if pr_details is None:
        print("Failed to get pull request details", file=sys.stderr)
        return 1

    release_notes = []
    for pr_number in sorted_pr_numbers:
        pull_request = pr_details.get(pr_number)
        if not pull_request:
            continue
        change = extract_user_facing_change(pull_request.get("body") or "")
        if not change:
            continue
        author = pull_request.get("author") or {}
        release_notes.append(
            {
                "number": pr_number,
                "title": pull_request.get("title", ""),
                "author": author.get("login", "unknown"),
                "kind": extract_pr_kind(
                    pull_request.get("labels"), pull_request.get("body") or ""
                ),
                "change": change,
            }
        )

    print("\n" + "=" * 80)
    print("SUMMARY OF PRS WITH USER-FACING CHANGES")
    print("=" * 80)
    if release_notes:
        for release_note in release_notes:
            print(f"\nPR #{release_note['number']} by @{release_note['author']}")
            print(f"Title: {release_note['title']}")
            print(f"Kind: {release_note['kind']}")
            print(f"Change: {release_note['change']}")
    else:
        print("No PRs with user-facing changes found.")
    print(f"\nTotal PRs with user-facing changes: {len(release_notes)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
