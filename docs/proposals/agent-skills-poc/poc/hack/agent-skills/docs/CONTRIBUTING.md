# Contributing to `hack/agent-skills/`

This directory is a small dedicated subsystem inside the Karmada repo. The rules are
deliberately tighter than the rest of the tree because (a) skills are loaded into LLM
context windows where every kilobyte costs accuracy, and (b) skills make claims about
the API that, if wrong, propagate into every YAML the agent produces.

If you want to add a skill, a knowledge file, an example, or a helper script, read this
once. The CI gates below will catch most slips, but the design rules are not automated.

## Adding a new skill

A skill is a directory under `skills/<skill-name>/` with one mandatory file: `SKILL.md`.

```
skills/karmada-explain-placement/
├── SKILL.md                # mandatory, see "SKILL.md format" below
└── (optional supplemental files referenced by SKILL.md)
```

### SKILL.md format

The file is Markdown with YAML frontmatter. The frontmatter is the part the agent's
skill loader reads to decide whether to load this skill into context.

```markdown
---
name: karmada-explain-placement
description: One-sentence "when to load me" phrased as a *trigger*, not a summary.
              The skill loader matches user messages against this string.
metadata:
  type: workflow | knowledge | component
  loads:                   # knowledge/ files this skill depends on
    - knowledge/00-overview.md
  uses_scripts:            # scripts/ files this skill calls
    - scripts/explain_placement.py
  status: scaffold | beta | stable
---

# karmada-explain-placement

(Markdown body — workflow, decision tables, examples, failure modes, boundaries.)
```

Required body sections in this order:

1. **When to load this skill** — bullet list of triggers.
2. **Workflow** — numbered steps the agent should follow.
3. **Boundaries** — what this skill does NOT do; route to which sibling skill.
4. **Failure modes** — at least three failure modes the agent should anticipate.
5. **Examples** — at least one end-to-end example with user input + agent output.

Optional but encouraged:

- Decision tables — they convert prose into something an LLM follows reliably.
- Quoted CITATIONS to the upstream source (`pkg/apis/policy/v1alpha1/...`, CRDs in
  `charts/karmada/_crds/bases/policy/...`).

### Naming rules

- Skill name = directory name = frontmatter `name`. All three must match.
- Format: `karmada-<verb>-<noun>` (workflow) or `karmada-<component>` (component).
- ASCII, kebab-case, max 4 dashes.

## Adding a knowledge file

Knowledge lives in `knowledge/` with a numeric prefix that orders the files:

- `0X-*.md` — handwritten overview / patterns / troubleshooting / components / hard rules
- `1X-*.md` — auto-generated from CRDs by `scripts/extract_crd_schema.py`
- `2X-*.md` — future: handwritten reference per component

Pick a slot that does not collide. If you are adding handwritten content, put the
banner

```markdown
<!--
This file is human-authored. Auto-generated schema lives in 1X-*.md.
-->
```

at the top. If you are adding to the auto-generated tier, you should be editing
`scripts/extract_crd_schema.py`, not the Markdown.

### Knowledge file style

- Lead with a one-paragraph "why this file exists" so a reader who landed here from
  a link knows whether to keep reading.
- Cite source code paths (`pkg/...`, `charts/...`) inline whenever you state a fact
  about the API.
- Prefer tables over prose for catalogs.
- Cap at ~250 lines. Skills load multiple knowledge files at once; long files
  starve the context window.

## Adding a helper script

Helpers live in `scripts/` and exist to remove hallucination risk from the agent.
Every helper must:

1. Be deterministic (same input → same output, byte for byte). No random sampling.
   No "smart" fallback that calls an LLM internally.
2. Take its input as a flag or stdin, write its output to stdout, and return a
   non-zero exit code on any error. This makes it composable in shell pipelines.
3. Print errors to stderr with enough detail that an agent can quote the message
   back to the user.
4. Have at least one unit test in `tests/test_helpers.py` that runs against a
   fixture in `examples/`.
5. Stay in the standard library + PyYAML. New third-party dependencies need
   maintainer approval — they bloat the CI image and break offline contributors.

### Helper-script header template

```python
#!/usr/bin/env python3
"""
<one-line summary>

Usage:
    <one-line invocation example>

(longer description of what it does and what it deliberately does not do)
"""
```

The docstring is the first thing a reviewer reads, so it must explain *why* this
helper exists, not just what it does.

## Adding an example

Examples live in `examples/<NN-short-name>/`. Each example is a self-contained
scenario that exercises one or more skills end-to-end.

Required files:

- `README.md` — one paragraph framing the scenario, then a "Replay" section with
  the exact shell commands a reviewer can copy-paste.
- The input artifacts (workload YAML, intent JSON, snapshot bundle, ...).
- The expected output artifacts (generated policy, audit findings, debug verdict).

Examples are how the test suite stays honest. If you change a helper's output
format, the example's expected file changes too, and the test fails until both are
in sync.

## CI gates

The CI workflow at `.github/workflows/agent-skills.yaml` (planned during mentorship
week 2) runs:

1. `python3 -m unittest tests.test_helpers` — all helpers pass their fixtures.
2. `python3 scripts/extract_crd_schema.py …` against each CRD in
   `charts/karmada/_crds/bases/policy/` and asserts the result is byte-identical
   to the committed `knowledge/1X-*.md`. This guarantees the auto-generated files
   stay in sync with upstream.
3. A schema-validate step that pipes every YAML under `templates/` and `examples/`
   through `kubeconform` against the same CRDs. Any drift fails the build.
4. A skill-frontmatter check that asserts every `SKILL.md` has the required
   metadata fields and that `metadata.loads` / `metadata.uses_scripts` references
   actually exist on disk.

Skip the gates only with a maintainer's `/override` comment and a written reason.

## Reviewer's checklist

Use this when reviewing a PR that touches `hack/agent-skills/`:

- [ ] New skill has all five required body sections.
- [ ] New knowledge file is < 250 lines and cites source code inline.
- [ ] New helper script is deterministic and stdlib-only (modulo PyYAML).
- [ ] New helper has at least one test against an `examples/` fixture.
- [ ] Auto-generated schema files have not been hand-edited.
- [ ] Frontmatter `loads:` paths resolve.
- [ ] No skill claims a behavior the CRD does not actually have.
