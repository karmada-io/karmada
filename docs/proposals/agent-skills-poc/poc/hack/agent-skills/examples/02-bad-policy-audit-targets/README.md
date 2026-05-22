# Example 02 — Bad policies that exercise the auditor

This directory holds intentionally broken YAMLs that the `karmada-audit-policy` skill
must catch. Each file has a sibling `.expected.json` with the audit findings the user
should see. The CI test asserts that the auditor's output matches.

| File                       | Violates       | Severity       |
|----------------------------|----------------|----------------|
| `bad-1-misplaced-rs.yaml`  | R1             | error          |
| `bad-2-jsonpath-path.yaml` | R5             | error          |
| `bad-3-empty-selectors.yaml` | R2           | error          |
| `bad-4-deprecated.yaml`    | R14            | warn           |
| `bad-5-mutex-affinity.yaml`| R4             | error          |
| `bad-6-float-weight.yaml`  | R11            | error          |

These are the most common LLM hallucinations in practice, drawn directly from the
LFX proposal discussion in karmada-io/community#190.
