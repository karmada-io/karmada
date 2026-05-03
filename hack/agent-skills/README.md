# Karmada Agent Skills

Official Karmada skill set for AI coding agents. Provides deterministic guidance for
policy creation, auditing, placement explanation, and propagation debugging.

## Directory Structure

```
hack/agent-skills/
├── skills/          # Agent skill definitions (SKILL.md per skill)
├── knowledge/       # Shared knowledge base (policy APIs, patterns, troubleshooting)
├── templates/       # Reusable YAML templates for all policy types
├── examples/        # Worked scenarios (multi-country, failover, image-override)
├── scripts/         # Deterministic helpers for schema-sensitive tasks
├── tests/           # Skill validation tests
├── CONTRIBUTING.md  # How to add new skills and knowledge
└── README.md        # This file
```

## Available Skills

| Skill | Purpose |
|-------|---------|
| `karmada-knowledge` | Load Karmada concepts, APIs, components, and policy patterns |
| `karmada-create-policy` | Generate PropagationPolicy and OverridePolicy YAML for workloads |
| `karmada-audit-policy` | Review policies for selector mismatch, scope misuse, and risks |
| `karmada-explain-placement` | Explain how a workload will be placed across member clusters |
| `karmada-debug-propagation` | Debug why a resource was not propagated to a member cluster |
| `karmada-search` | Search Karmada resources, bindings, and works |
| `karmada-controller-manager` | Understand and interact with karmada-controller-manager |

## Quick Start

To use these skills with an AI coding agent, reference the skill directory:

```
# Example: create a PropagationPolicy for a Deployment
Load skill: hack/agent-skills/skills/karmada-create-policy/SKILL.md
```

Each skill's SKILL.md describes when to load, the workflow to follow, and which
knowledge files and templates to reference.
