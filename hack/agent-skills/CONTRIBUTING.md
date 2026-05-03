# Contributing to Karmada Agent Skills

Skills constrain what AI agents do; their value comes from being
**precise** and **deterministic**, not exhaustive. A skill that says
"agents should consider edge cases" does nothing. A skill that says
"always invoke `policy-generator.py`, never free-hand YAML" prevents
an entire class of bug.

## Anatomy of a skill

```
skills/<skill-name>/SKILL.md
```

`SKILL.md` MUST contain YAML front matter:

```yaml
---
name: skill-name
description: One-sentence purpose statement (used in skill loaders).
when_to_load: |
  Trigger conditions: keywords, file types, user phrasings.
---
```

Followed by these sections (in order):

1. **Hard rules** — what the agent must always / never do. Imperative.
2. **Workflow** — numbered steps the agent walks deterministically.
3. **Failure modes** — anti-patterns the agent must catch.
4. **References** — paths to knowledge/templates/examples.

## Style rules for SKILL.md

- **Concrete over abstract.** Show a command, not a description of a
  command. Show YAML, not "the YAML structure".
- **Decision tables beat prose.** When the agent must pick between two
  shapes, give a two-column table; do not write a paragraph.
- **Cite the source of truth.** API claims must reference
  `pkg/apis/policy/v1alpha1/*.go` or `knowledge/policy-apis.yaml`.
- **No hedging.** "May", "could", "consider" produce hedged behavior.
  Use "must" and "do not" — or remove the rule entirely.

## Adding a knowledge file

`knowledge/*.md` is human-shaped reference text. `knowledge/*.yaml` is
data consumed by skills programmatically (e.g., the `pitfalls:` block
in `policy-apis.yaml` is read by `validate-policy.py` rule logic).

When you add a new pitfall to `policy-apis.yaml`, also:

1. Add a corresponding rule (KMDxxx) in `scripts/validate-policy.py`.
2. Add a unit test in `tests/test_validate_policy.py`.
3. Reference the rule ID in `skills/karmada-audit-policy/SKILL.md`.

## Adding a template

- Templates use `${VARIABLE}` placeholders, not Helm or Go templating.
- Every required field must be present (commented `OPTIONAL` for the
  rest).
- A template MUST `validate-policy.py`-clean once placeholders are
  filled. Add a smoke test in `tests/test-skills.sh`.

## Adding an example

Each example lives in `examples/<name>/` with:

- `README.md` — scenario, manifests list, reproduce command, verify
  step.
- The actual manifests, **regenerable** by the documented command.

Hand-written manifests rot. The CI shell test re-runs the generator
and diffs against the committed file.

## Tests

Run before opening a PR:

```bash
python3 -m unittest discover -s hack/agent-skills/tests -p 'test_*.py' -v
bash    hack/agent-skills/tests/test-skills.sh
```

## What NOT to add

- Skills that "help the agent think". Skills are levers, not advice.
- Knowledge files duplicating what's already in
  `knowledge/policy-apis.yaml` or upstream Go source.
- Templates with TODO/FIXME placeholders. If you don't know the value,
  don't ship the template.
