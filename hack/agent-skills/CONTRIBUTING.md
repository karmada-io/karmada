# Contributing to Karmada Agent Skills

## Adding a New Skill

1. Create a directory under `skills/` named after the skill (kebab-case).
2. Add a `SKILL.md` file with these sections:
   - **Description**: One sentence on what the skill does.
   - **When to Load**: Trigger conditions (keywords, user intents).
   - **Workflow**: Step-by-step instructions for the agent.
   - **Knowledge References**: Paths to `knowledge/` files to load.
   - **Template References**: Paths to `templates/` files to use.
   - **Example References**: Paths to `examples/` scenarios.

3. Add knowledge files under `knowledge/` if the skill introduces new concepts.
4. Add templates under `templates/` if the skill needs starter YAML.
5. Add example scenarios under `examples/` for complex use cases.
6. Add deterministic helpers under `scripts/` for schema-sensitive tasks.
7. Add tests under `tests/` to validate skill output correctness.

### SKILL.md Format

```markdown
# Skill Name

## Description
Brief description.

## When to Load
- Trigger condition 1
- Trigger condition 2

## Workflow
1. Step one
2. Step two

## Knowledge References
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/policy-patterns.md`

## Template References
- `hack/agent-skills/templates/propagation-policy.yaml`

## Example References
- `hack/agent-skills/examples/multi-country-setup/`
```

## Adding Knowledge

Knowledge files live under `knowledge/`. Use these formats:

- **`*.yaml`**: Structured reference data (API fields, enums, constants).
- **`*.md`**: Narrative guides, patterns, troubleshooting playbooks.

## Adding Templates

Templates live under `templates/`. Each template should:
- Use YAML format with `karmada-io/karmada` native Kinds.
- Include placeholder comments where values need to be filled.
- Be valid YAML (syntax-checked by CI).

## Adding Examples

Examples live under `examples/`. Each example should have:
- One or more policy/service YAML files.
- A `README.md` explaining the scenario and expected outcome.

## Testing

Run tests from the `tests/` directory:

```bash
cd hack/agent-skills/tests
./test-skills.sh
```

Tests validate that:
- Templates are valid YAML.
- Examples reference real API fields.
- Skills reference existing knowledge files.
- Scripts execute without errors.

## Conventions

- Skill names use kebab-case, matching their directory name.
- Knowledge file names use kebab-case.
- Template file names use kebab-case and match the Kind they template.
- Example directory names use kebab-case.
- Always reference the full API version `policy.karmada.io/v1alpha1`.
- Use correct Karmada-specific terminology: PropagationPolicy, OverridePolicy,
  ResourceBinding, Work, ClusterPropagationPolicy, ClusterOverridePolicy.
