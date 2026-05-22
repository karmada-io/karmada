---
name: karmada-knowledge
description: Read-only Karmada reference. Use when the user asks how Karmada works, what a CRD field means, which component owns a behavior, or what a policy pattern looks like. Pulls in the shared knowledge base under hack/agent-skills/knowledge/.
metadata:
  type: knowledge
  loads:
    - knowledge/00-overview.md
    - knowledge/01-policy-patterns.md
    - knowledge/02-troubleshooting.md
    - knowledge/03-components.md
    - knowledge/04-hard-rules.md
    - knowledge/10-propagation-schema.md
    - knowledge/11-override-schema.md
---

# karmada-knowledge

This skill is the agent's *map* of Karmada. Other skills depend on it: create-policy,
audit-policy, debug-propagation, explain-placement all load this skill first so they
share a common vocabulary.

## When to load this skill

Load when the user message contains any of:
- conceptual questions ("what is a ResourceBinding?", "when does Karmada use Work objects?")
- API field questions ("can I set replicaScheduling at the spec root?")
- component questions ("which component schedules?", "what does karmada-search do?")
- pattern questions ("how do I do regional failover?")

Do NOT load this skill alone when the user wants YAML produced or audited — load
`karmada-create-policy` or `karmada-audit-policy` instead; they pull in the relevant
slices of this knowledge automatically.

## How to answer with this skill

1. Read the user's question and identify *which* knowledge file is most relevant. The
   files are ordered by depth: 00 (overview) → 04 (hard rules) → 10/11 (auto-generated
   schemas).
2. Quote the relevant section. Never paraphrase a schema field or rule from memory —
   the auto-generated schema files exist precisely so the agent does not have to.
3. If the answer involves a code path, cite the file under `pkg/apis/policy/v1alpha1/`,
   `pkg/controllers/`, or the CRD path under `charts/karmada/_crds/bases/policy/`.
4. If the question crosses skills (e.g. "how do I fix this policy?"), recommend the
   appropriate sibling skill rather than answering yourself.

## Boundaries

- This skill does NOT generate YAML. Route to `karmada-create-policy`.
- This skill does NOT audit user YAML. Route to `karmada-audit-policy`.
- This skill does NOT debug live failures. Route to `karmada-debug-propagation`.
- When a question is ambiguous (e.g. "tell me about overrides"), prefer to point at
  the most specific knowledge file and let the user follow up, rather than dumping
  the entire knowledge base into the response.

## Examples of good answers

> **User:** "What's the difference between clusterAffinity and clusterAffinities?"
>
> **Agent (with this skill):** Quotes the table in `knowledge/01-policy-patterns.md`
> entries P1–P3, links to the schema rows in `knowledge/10-propagation-schema.md`,
> mentions rule R4 from `knowledge/04-hard-rules.md` (mutually exclusive). Three
> sentences, no invented fields.
