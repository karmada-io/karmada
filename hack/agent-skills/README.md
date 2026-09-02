# Karmada Agent Skills

## Overview

Karmada is a multi-cluster orchestration system for Kubernetes, but AI coding agents still lack an official Karmada-specific skill set. Without a dedicated knowledge and workflow repository, AI agents often need to:
- search fragmented documentation
- infer policy structure from incomplete context
- generate incorrect `PropagationPolicy` or `OverridePolicy` resources
- provide unreliable troubleshooting guidance

This directory provides structured knowledge, reusable templates, workflow-oriented skills, deterministic helper scripts, and example scenarios that help AI agents interact with Karmada more safely and accurately.

The goal is to establish a shared foundation for:
- policy generation
- policy auditing
- placement explanation
- propagation debugging
- component understanding
- multi-cluster operational guidance

## Directory Structure

```text
hack/agent-skills/
├── skills/
│   ├── karmada-knowledge/
│   ├── karmada-create-policy/
│   ├── karmada-audit-policy/
│   ├── karmada-explain-placement/
│   ├── karmada-debug-propagation/
│   ├── karmada-search/
│   └── karmada-controller-manager/
├── knowledge/
├── templates/
├── examples/
└── tests/
```

## Directory Purpose

### `skills/`
Workflow-oriented skill packages for AI agents.

Each skill contains:
- task-specific instructions
- recommended workflows
- required knowledge references
- optional helper utilities

Example:
- `karmada-create-policy`
- `karmada-debug-propagation`

### `knowledge/`
Shared factual reference material for Karmada APIs, components, behaviors, and operational concepts.

Examples:
- PropagationPolicy API
- OverridePolicy behavior
- ResourceBinding workflow
- Scheduler concepts
- Troubleshooting references

### `templates/`
Reusable manifest templates and structured response patterns.

Examples:
- starter `PropagationPolicy`
- cluster placement templates
- override examples
- troubleshooting response structures

### `examples/`
Example-driven learning scenarios and expected outputs.

Examples:
- multi-region placement
- failover strategies
- cluster-specific image overrides
- propagation debugging walkthroughs

### `tests/`
Validation cases and helper scripts used to verify generated manifests, examples, and skill behavior.

## How AI Agents Use These Skills

Typical workflow:

1. Load knowledge files from `knowledge/`
2. Select a workflow skill from `skills/`
3. Reuse templates from `templates/`
4. Generate manifests or explanations
5. Validate results using `tests/`

## Example Use Cases

| User Request | Agent Workflow |
| --- | --- |
| Create a policy for member1 and member2 with different images | Load `karmada-create-policy` → read policy knowledge → generate `PropagationPolicy` and `OverridePolicy` |
| Review this `PropagationPolicy` | Load `karmada-audit-policy` → validate selectors, placement rules, and policy structure |
| Explain multi-country placement design | Load `karmada-knowledge` → explain scheduling, failover, and override strategies |
| Why was my workload not propagated to member2? | Load `karmada-debug-propagation` → trace object → policy → `ResourceBinding` → `Work` → member cluster |

## Adding a New Skill

1. Create directory: mkdir -p skills/your-skill-name/helpers
2. Write SKILL.md using the template below
3. Add helper scripts (if deterministic logic needed)
4. Add examples to examples/
5. Update this README with the new skill
6. Add validation to tests/validate-skills.sh

## SKILL.md Template

```markdown
# Skill: your-skill-name

## Purpose
What problem does this skill solve? (1-2 sentences)

## When to Use
Trigger conditions for AI agents (e.g., "user asks to create a policy")

## Prerequisites
- Required knowledge files (list from `knowledge/`)
- Required templates (list from `templates/`)

## Workflow
1. Load required knowledge
2. Analyze user input
3. Execute deterministic logic (if any)
4. Generate output (YAML, explanation, or debug info)
5. Validate output

## Output Format
Describe the structure of the response (e.g., YAML with specific fields)

## Required Knowledge
- `../../knowledge/propagation-policy-api.md`
- `../../knowledge/override-policy-api.md`

## Example
Provide a concrete example of user input → skill execution → output
```

## Initial Skill Set

The initial implementation includes:

- `karmada-knowledge`
- `karmada-create-policy`
- `karmada-audit-policy`
- `karmada-explain-placement`
- `karmada-debug-propagation`
- `karmada-search`
- `karmada-controller-manager`

## Future Expansion

Potential future additions include:
- policy recommendation helpers
- topology-aware placement guidance
- workload failover analysis
- scheduler decision explanation
- policy linting utilities
- cluster capability awareness
- AI-assisted operational debugging

## Status

This directory currently provides the foundational structure for AI-agent-oriented Karmada skills and knowledge. Additional skills, examples, helper utilities, validation workflows, and reusable policy patterns will be added incrementally.