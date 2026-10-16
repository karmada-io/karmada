---
name: generating-release-notes
description: "Use when preparing changelogs or release notes from merged pull requests for a patch, minor, or preview release."
argument-hint: "Target release version, for example v1.14.5 or v1.15.0-rc.0"
---

# Generating Release Notes

## Overview

Generate release notes from merged pull requests. Complete each gate in order. Do not start writing until the target version, comparison range, and format plugin are known.

## Inputs

A target version is required. If it is missing or unclear, ask for it and stop. Do not guess "the next release."

Work out these values only after the target is known:

- Release type: patch, minor, or preview.
- Previous version or preview tag.
- Comparison branch.
- Output changelog path.

The selected format plugin defines how to work out these values for its repository.

## Select a Format Plugin

1. Determine the Git repository root directory name with `basename "$(git rev-parse --show-toplevel)"`.
2. Look at the Markdown files under [references](./references/). Select a plugin whose `Repository Match` includes that directory name.
3. If exactly one plugin matches, load it and follow it exactly.
4. If multiple plugins match, ask the user to choose.
5. If no plugin matches, stop and report the directory name. Ask for a new repository format plugin. Never silently use another repository's format.

You can add a format plugin for a new repository. Add `<repository>-release-note-format.md` under `references/` with these sections:

- `Repository Match`
- `Upstream Data Source`
- `Version Resolution`
- `Output Location`
- `Document Structure`
- `Entry Format and Classification`
- `Language and Style`
- `Validation`

## Required Workflow

### Gate 1: Resolve the Release Range

1. Classify the target as patch, minor, or preview using the selected plugin.
2. Resolve the previous tag and head branch using the plugin's rules.
3. Check the target, previous tag, branch, and output path against what the user asked for.
4. Stop if anything is unclear. Do not switch to a different tag or branch without confirmation.

Do not continue until Gate 1 passes.

### Gate 2: Collect Merged Pull Requests

1. Check that `python3` and the included [fetch_pr_info.py](./scripts/fetch_pr_info.py) exist.
2. Check that `GITHUB_TOKEN` is set, but do not check its scopes. If it is not set but `gh auth status` reports a logged-in account, reuse that credential with `export GITHUB_TOKEN=$(gh auth token)` in the same shell as the scripts. Otherwise, ask the user to set a token with the `public_repo` scope in their environment. Never ask the user to send a token through chat, and never print the token value.
3. From the repository root, run the included script with the base tag and head branch from Gate 1 and the `owner/repository` value listed under the plugin's `Upstream Data Source`:

	```bash
	python3 .claude/skills/generating-release-notes/scripts/fetch_pr_info.py <base-tag> <head-branch> --repo <owner/repository>
	```

4. Use only the `SUMMARY OF PRS WITH USER-FACING CHANGES` section. Ignore debug output and PRs without user-facing changes.
5. Keep the PR number, title, author, kind, and every user-facing change. If a PR has more than one user-facing change, split them into separate entries.

Do not continue if collection fails or credentials are invalid.

### Gate 3: Collect Contributors

Run this gate only when the selected format plugin requires a Contributors section.

1. Reuse the same base tag and head branch from Gate 1. Contributor collection and PR collection must use the same comparison range.
2. From the repository root, run the included [fetch_contributors.sh](./scripts/fetch_contributors.sh) with the owner and repository listed under `Upstream Data Source`:

	```bash
	.claude/skills/generating-release-notes/scripts/fetch_contributors.sh <owner> <repository> <base-tag> <head-branch>
	```

3. The output is the list of GitHub authors whose commits were merged in that comparison range.
4. Keep unique GitHub handles sorted alphabetically.

Do not continue if contributor collection fails.

### Gate 4: Classify and Draft

1. Use the PR kind returned by the collection script as the main signal for choosing a category.
2. Apply the plugin's rules for which category wins when more than one applies.
3. Rewrite each entry in the tense and entry syntax the plugin requires, without changing its technical meaning. Make the leading verb and wording match the final category. If you move an entry to another category, re-read its wording and rewrite it if needed.
4. Group entries as required by the plugin.
5. Build the complete release section using the structure for the target release type.
6. Add the commit authors collected in Gate 3 when the release requires a Contributors section.

Do not make up user-facing changes, and do not guess at behavior from a PR title alone.

### Gate 5: Update the Changelog

1. Read the existing changelog before editing.
2. Insert the new release in the order the plugin requires.
3. Keep existing release sections and unrelated user changes as they are.
4. Run the plugin's TOC or formatting command after content is final.

### Gate 6: Validate

Check the requirements that apply to every repository:

- No user-facing change is duplicated or missing.
- Existing changelog content and unrelated user changes are unchanged.

Run every check in the selected plugin's `Validation` section. Fix all failures before finishing.

### Gate 7: Improve this skill

After the release notes are done, think about what you learned. If you found something that would help future runs — a new failure pattern, a better command, or a wrong assumption in this document — propose an update to this skill file:

- Show the user the proposed change and let them decide whether to apply it.
- Do not commit the change yourself.

## Common Failures

| Failure | Required response |
|---|---|
| Target version is missing | Ask for it; do not guess. |
| No format plugin matches | Stop and request a plugin. |
| PR or contributor collection fails | Report the error and stop; do not make up or replace data. |
