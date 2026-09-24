---
name: generating-release-notes
description: "Use when preparing changelogs or release notes from merged pull requests for a patch, minor, or preview release."
argument-hint: "Target release version, for example v1.14.5 or v1.15.0-rc.0"
---

# Generating Release Notes

## Overview

Generate release notes from merged pull requests. Complete each gate in order; do not draft content until the target version, comparison range, and format plugin are resolved.

## Inputs

Require a target version. If it is absent or ambiguous, ask for it and stop. Do not infer "the next release."

Derive these values only after the target is known:

- Release type: patch, minor, or preview.
- Previous version or preview tag.
- Comparison branch.
- Output changelog path.

The selected format plugin defines repository-specific derivation rules.

## Select a Format Plugin

1. Determine the Git repository root directory name with `basename "$(git rev-parse --show-toplevel)"`.
2. Inspect Markdown files under [references](./references/). Select a plugin whose `Repository Match` includes that directory name.
3. If exactly one plugin matches, load it and follow it as authoritative.
4. If multiple plugins match, ask the user to choose.
5. If no plugin matches, stop and report the directory name. Ask for a new repository format plugin. Never silently use another repository's format.

Repository format plugins are pluggable. Add `<repository>-release-note-format.md` under `references/` with these sections:

- `Repository Match`
- `Upstream Data Source`
- `Version Resolution`
- `Output Location`
- `Document Structure`
- `Entry Format and Classification`
- `Language and Style`
- `Validation`

## Mandatory Workflow

### Gate 1: Resolve the Release Range

1. Classify the target as patch, minor, or preview using the selected plugin.
2. Resolve the previous tag and head branch using the plugin's rules.
3. Verify the target, previous tag, branch, and output path against the user-visible task context.
4. Stop on any ambiguity. Do not substitute a different tag or branch without confirmation.

Do not continue until Gate 1 passes.

### Gate 2: Collect Merged Pull Requests

1. Verify that `python3` and the bundled [fetch_pr_info.py](./scripts/fetch_pr_info.py) exist.
2. Verify that `GITHUB_TOKEN` is set, but do not verify its scopes. If it is not set, ask the user to set a token with the `public_repo` scope in their environment. Never ask the user to send a token through chat.
3. From the repository root, run the bundled script with the base tag and head branch resolved in Gate 1 and the `owner/repository` value declared under the plugin's `Upstream Data Source`:

	```bash
	python3 .claude/skills/generating-release-notes/scripts/fetch_pr_info.py <base-tag> <head-branch> --repo <owner/repository>
	```

4. Use only the `SUMMARY OF PRS WITH USER-FACING CHANGES` section. Ignore diagnostic and non-user-facing PR output.
5. Preserve PR number, title, author, kind, and every user-facing change. Split multiple user-facing changes into separate candidate entries.

Do not continue if collection fails or credentials are invalid.

### Gate 3: Collect Contributors

Run this gate only when the selected format plugin requires a Contributors section.

1. Reuse the same base tag and head branch resolved in Gate 1. Contributor collection and PR collection must use the same comparison range.
2. From the repository root, run the bundled [fetch_contributors.sh](./scripts/fetch_contributors.sh) with the owner and repository declared under `Upstream Data Source`:

	```bash
	.claude/skills/generating-release-notes/scripts/fetch_contributors.sh <owner> <repository> <base-tag> <head-branch>
	```

3. Treat the output as the GitHub authors of commits merged in that comparison range.
4. Keep unique GitHub handles sorted alphabetically.

Do not continue if contributor collection fails.

### Gate 4: Classify and Draft

1. Use the PR kind supplied by the collection script as the primary classification signal.
2. Apply the plugin's category precedence rules.
3. Rewrite each entry into the plugin's required tense and entry syntax without changing its technical meaning. Make the leading verb and sentence framing match the final category; moving an entry between categories requires reviewing and, when necessary, rewriting its wording.
4. Group entries as required by the plugin.
5. Build the complete release section using the structure for the target release type.
6. Add the commit authors collected in Gate 3 when the release requires a Contributors section.

Do not invent user-facing changes or infer undocumented behavior from a PR title alone.

### Gate 5: Update the Changelog

1. Read the existing changelog before editing.
2. Insert the new release in the ordering required by the plugin.
3. Preserve existing release sections and unrelated user changes.
4. Run the plugin's TOC or formatting command after content is final.

### Gate 6: Validate

Verify the repository-independent requirements:

- No user-facing change is duplicated or omitted.
- Existing changelog content and unrelated user changes are preserved.

Run every check in the selected plugin's `Validation` section. Fix all failures before finishing.

### Gate 7: Improve this skill

After the release-note generation is done, review what you learned. If you found something that would help future release-note runs — a new failure pattern, a better command, a log location not listed here, or a wrong assumption in this document — propose an update to this skill file:

- Show the user the proposed change and let them decide whether to apply it.
- Do not commit the change yourself.

## Common Failures

| Failure | Required response |
|---|---|
| Target version is missing | Ask for it; do not guess. |
| No format plugin matches | Stop and request a plugin. |
| PR or contributor collection fails | Report the error and stop; do not invent or substitute data. |
