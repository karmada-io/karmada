# hack/agent-skills

This directory contains deterministic knowledge, workflows, examples, and helper assets for AI coding agents working on Karmada.

The goal is simple: reduce hallucinations and make agent output reproducible.

Karmada has several schema-sensitive surfaces where vague natural-language guidance is not enough:
- PropagationPolicy and ClusterPropagationPolicy generation
- OverridePolicy and Override APIs
- placement and replica scheduling semantics
- troubleshooting controller behavior
- search and discovery across multi-cluster resources

This repository area gives agents a constrained, reviewable source of truth. It is intended to complement normal documentation, not replace it.

## What belongs here

Store assets that help an agent answer or generate something correctly on the first pass:
- dense knowledge files in Markdown
- step-by-step workflow guidance for one task
- deterministic helper scripts in Go or Bash
- YAML or JSON fixtures
- curated examples that are known to be valid
- troubleshooting notes tied to real Karmada behavior
- small validation utilities for schema-heavy outputs

Prefer explicit, machine-checkable content over prose when the task is sensitive to schema or API shape.

## How to organize a new skill

Each skill should have a narrow purpose and a predictable layout.

Recommended pattern:
- one directory per skill
- one short README that explains the user goal, inputs, outputs, and assumptions
- one or more dense Markdown knowledge files for rules and edge cases
- optional helper scripts when the skill needs deterministic generation or validation
- optional fixtures and examples that demonstrate valid and invalid cases

A good skill should answer these questions:
- What problem does this skill solve?
- What exact input should the agent expect?
- What exact output should the agent produce?
- What fields or invariants must never be violated?
- What validation should happen before output is returned?

## File conventions

Use Markdown for durable knowledge and operational guidance.

Use Go or Bash for helper scripts when the output must be deterministic, schema-aware, or easy to validate in CI.

Use YAML for examples that should look like real Karmada resources.

Use JSON only when the target tool or validator expects JSON.

Keep examples small. A skill is more useful when it is precise than when it is broad.

## Writing standards for skill content

When writing agent-facing content:
- be direct
- state invariants explicitly
- prefer rules over explanations
- include common failure modes
- avoid contradictory guidance
- avoid ambiguous words like should unless the behavior is actually optional

For schema-sensitive Karmada objects, always document:
- required fields
- mutually exclusive fields
- default behavior
- precedence rules
- behavior on empty or omitted values
- common mistakes that cause invalid resources or unexpected scheduling

## Deterministic helpers

If a skill needs generated output, validation, or sample data, use a helper script rather than asking the agent to infer the result from prose.

Preferred helper responsibilities:
- generate canonical YAML snippets
- validate required fields and mutual exclusions
- compare an output object against known-good fixtures
- print concise diagnostics for common mistakes
- normalize example manifests for review

Helper scripts should be:
- deterministic
- small enough to audit
- easy to run locally
- safe to execute in CI

## Adding a new skill

When adding a new skill:
1. Create a dedicated directory under hack/agent-skills/.
2. Add a README describing the skill goal and expected behavior.
3. Add dense Markdown knowledge files for rules and edge cases.
4. Add fixtures and examples that demonstrate valid and invalid cases.
5. Add helper scripts only when the skill benefits from deterministic generation or validation.
6. Keep the skill narrow. Do not mix unrelated workflows into one folder.
7. Update shared knowledge if the new skill reveals a reusable pattern.

## Review checklist

Before merging new skill content, confirm:
- the guidance is consistent with Karmada APIs
- required fields are called out explicitly
- mutually exclusive fields are documented
- the examples are valid and minimal
- helper scripts produce stable output
- the skill does not rely on implicit agent reasoning for schema-critical decisions

## Repository intent

The long-term purpose of this directory is to become the official Karmada skill base for AI coding agents.

That means the content here should evolve toward:
- shared policy knowledge
- focused workflow skills
- reusable troubleshooting notes
- deterministic generation helpers
- validated examples that reduce hallucination risk
