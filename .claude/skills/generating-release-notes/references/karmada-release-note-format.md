# Karmada Release Note Format

## Repository Match

- Git repository root directory name: `karmada`

Select this format whenever the Git repository root directory is named `karmada`.

## Upstream Data Source

- GitHub owner: `karmada-io`
- GitHub repository: `karmada`
- Script `--repo` value: `karmada-io/karmada`

Always collect comparison and pull request data from `karmada-io/karmada`, regardless of the local fork owner. Do not derive the data source owner from the local `origin` URL.

## Version Resolution

Classify versions as follows:

- Patch: `vX.Y.Z` where `Z > 0`. The base is `vX.Y.(Z-1)`, and the head is `release-X.Y`.
- Minor: `vX.Y.0`. Use `vX.Y.0-alpha.0` as the base and `master` as the head.
- Preview: `vX.Y.0-alpha.1`, `vX.Y.0-alpha.2`, `vX.Y.0-beta.0`, or `vX.Y.0-rc.0`. Use the previous preview tag and `master` as the head.

The expected preview sequence is:

1. `vX.Y.0-alpha.1`, based on `vX.Y.0-alpha.0`
2. `vX.Y.0-alpha.2`, based on `vX.Y.0-alpha.1`
3. `vX.Y.0-beta.0`, based on `vX.Y.0-alpha.2`
4. `vX.Y.0-rc.0`, based on `vX.Y.0-beta.0`

Confirm unusual or skipped preview sequences with the user.

## Output Location

Update `docs/CHANGELOG/CHANGELOG-X.Y.md`. This file contains the minor, preview, and all patch release notes for release line `X.Y`.

Order patch sections by version descending. Preserve the established placement of minor and preview sections in the existing file.

## Document Structure

### Patch Release

```markdown
# vX.Y.Z
## Downloads for vX.Y.Z
## Changelog since vX.Y.(Z-1)
### Changes by Kind
#### Bug Fixes
#### Others
```

Place fixes under `Bug Fixes`. Place other changes, including dependency upgrades, under `Others`. Preserve the exact capitalization already used by the target changelog when it differs from this schematic.

### Minor Release

```markdown
# vX.Y.0
## Downloads for vX.Y.0
## Urgent Update Notes
## What's New
## Other Notable Changes
### API Changes
### Features & Enhancements
### Deprecation
### Bug Fixes
### Security
## Other
### Dependencies
### Helm Charts
### Instrumentation
### Performance
## Contributors

Thank you to everyone who contributed to this release!

Users whose commits are in this release (alphabetically by username)

- @xxxA
- @xxxB
```

`Urgent Update Notes` and `What's New` remain empty and are reserved for manual completion. Build `Contributors` from the commit authors collected with the same comparison range used for pull requests, excluding accounts whose usernames end in `[bot]`, such as `dependabot[bot]` and `github-actions[bot]`.

### Preview Release

```markdown
# vX.Y.0-<preview>
## Downloads for vX.Y.0-<preview>
## Changelog since <previous-preview-version>
## Urgent Update Notes
## Changes by Kind
### API Changes
### Features & Enhancements
### Deprecation
### Bug Fixes
### Security
## Other
### Dependencies
### Helm Charts
### Instrumentation
### Performance
```

## Entry Format and Classification

Every entry uses this syntax:

```markdown
- `<component>`: <change> ([#<PR>](https://github.com/karmada-io/karmada/pull/<PR>), @<author>)
```

Rules:

- Mark component names with backticks, for example `karmada-controller-manager`.
- Keep entries for the same component together within a category.
- If one PR contains multiple user-facing changes, write one entry for each change and retain the same PR attribution.
- Use the PR's kind as the primary classification signal. For example, `feature` maps to `Features & Enhancements` and `bug` maps to `Bug Fixes`.
- Only use the title, description, and user-facing change as the primary signal when the kind is empty.
- Specialized categories may override the kind-based category when the user-facing change clearly belongs to them. Classify API changes in `API Changes`, dependency version changes in `Dependencies`, Helm packaging changes in `Helm Charts`, metrics or observability changes in `Instrumentation`, performance work in `Performance`, deprecations in `Deprecation`, and security corrections in `Security`.
- Outside those specialized categories, wording such as "fix" or "feature" in the title, description, or user-facing change must not override an available kind.
- When one PR contains changes belonging to different categories, classify each split entry independently.

## Language and Style

- Write all release-note content in English.
- Match the leading verb to the final category, even when the original user-facing change uses different framing.
- `Features & Enhancements` entries describe the capability that was introduced or enabled and normally begin with verbs such as "Added," "Introduced," "Enabled," or "Supported." Do not leave a feature entry framed as "Fixed" merely because the original PR description used that word.
- `Bug Fixes` entries describe the corrected defect and normally begin with "Fixed" or "Corrected."
- `API Changes` entries describe the API operation directly, using verbs such as "Added," "Introduced," "Updated," "Removed," or "Deprecated" as appropriate.
- Deprecation entries use present perfect tense, for example, "has been deprecated."
- Dependency entries use present perfect or simple past tense, for example, "has been upgraded to" or "Upgraded to."
- Features, fixes, and all other categories use simple past tense.
- Present-tense constructions such as "now supports" or "no longer relies on" are allowed only for a newly introduced capability or behavioral change.
- Correct spelling and grammar while preserving technical meaning and exact identifiers.

## Validation

1. Confirm category, component grouping, tense, PR URL, and author syntax for every entry.
2. For minor releases, confirm contributors exclude usernames ending in `[bot]`, are unique, and are alphabetically sorted.
3. Run `doctoc docs/CHANGELOG/CHANGELOG-X.Y.md` after the release section is complete.
4. The final changelog should have correct release ordering, an updated TOC, and no unrelated changes.
