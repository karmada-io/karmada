# Karmada Agent Skills — POC

A working skill set that turns AI coding agents into Karmada experts.
Built as a proof of concept for the LFX 2026 Term 2 mentorship project
**"Build Karmada Agent Skills"** ([karmada-io/community#190](https://github.com/karmada-io/community/issues/190)).

![demo](hack/agent-skills/docs/media/demo.svg)

---

## The 30-second pitch

Ask any stock LLM to "make a Karmada policy for this Deployment, place it on
member1/member2, use mirror.id.internal on member2," and you usually get YAML that
**fails admission** — capitalised `Apps/v1`, `replicaScheduling` at the wrong nesting
level, JSONPath where JSON Pointer is required, missing `propagateDeps`, deprecated
`association: true`. Every Karmada operator has hit it.

This POC fixes that. **Same prompt → clean YAML → zero audit findings → works on apply.**

→ Read the [side-by-side comparison](hack/agent-skills/examples/00-before-after-comparison/)
(one page, 30 seconds).

---

## Try it in one command

```
bash hack/agent-skills/demo.sh
```

That walks through: auto-extracting schema docs from real Karmada CRDs, generating a
multi-country `PropagationPolicy` + `OverridePolicy` from a JSON intent, auditing one
clean policy and two broken ones, diagnosing a `T-NoSchedule` propagation failure
from a captured snapshot, and running 14 unit tests. No Karmada cluster required.

The animated SVG above replays the live session at readable speed.
For step-through: `asciinema play hack/agent-skills/docs/media/demo.cast`.

---

## What it does

Four deterministic Python helpers, called by Markdown-frontmatter skill files that any
agent loader (Claude Skills, Codex, Cursor, etc.) can consume:

| Helper                       | Job                                                                  |
|------------------------------|----------------------------------------------------------------------|
| `extract_crd_schema.py`      | CRD → Markdown schema reference. Auto-sync, no hand-written field names. |
| `generate_policy.py`         | Structured intent JSON → valid `PropagationPolicy` + `OverridePolicy` YAML. |
| `audit_policy.py`            | YAML → JSON findings keyed to 15 hard rules (R1–R15).               |
| `debug_propagation.py`       | Snapshot bundle + target → first-failed-hop verdict (Object → Policy → RB → Work → Member). |

The split: **structure is mechanical (scripts) and intent is semantic (LLM).** Skills
ask the LLM to produce structured intent (which clusters, which placement mode, which
overrides); helpers produce the YAML. Neither half has to remember the API.

---

## What's here

```
hack/agent-skills/
├── README.md                     ← design rationale + full layout
├── demo.sh                       ← one-command walkthrough
│
├── skills/                       ← 7 skills (3 fully implemented, 4 scaffold)
│   ├── karmada-knowledge/
│   ├── karmada-create-policy/
│   ├── karmada-audit-policy/
│   ├── karmada-debug-propagation/
│   ├── karmada-explain-placement/    (scaffold)
│   ├── karmada-search/               (scaffold)
│   └── karmada-controller-manager/   (scaffold)
│
├── knowledge/                    ← 7 files (5 hand-written, 2 auto-extracted from CRDs)
├── scripts/                      ← 4 deterministic Python helpers (~750 LOC)
├── templates/                    ← minimal valid YAML + intent JSON
├── examples/                     ← 4 end-to-end scenarios with expected outputs
│   ├── 00-before-after-comparison/   ← the 30-second artifact ★
│   ├── 01-multi-country-frontend/    ← SG / ID / DE / BR fleet
│   ├── 02-bad-policy-audit-targets/  ← 6 broken YAMLs, one per hallucination class
│   └── 03-propagation-failure-bundle/← T-NoSchedule snapshot for the debugger
│
├── tests/test_helpers.py         ← 14 unittest cases, all green
└── docs/CONTRIBUTING.md          ← how mentees add skills, knowledge, helpers
```

Read [`hack/agent-skills/README.md`](hack/agent-skills/README.md) for the four
"Design decisions worth highlighting" sections (auto-extraction, structure-vs-intent
split, structured audit findings, snapshot-based debugger).

---

## Coverage map (proposal deliverables → POC files)

| Proposal deliverable                                  | POC location                                                              |
|-------------------------------------------------------|---------------------------------------------------------------------------|
| Shared knowledge base                                 | `knowledge/00-overview.md` · `01-policy-patterns.md` · `02-troubleshooting.md` · `03-components.md` · `04-hard-rules.md` |
| `karmada-knowledge` skill                             | `skills/karmada-knowledge/SKILL.md`                                       |
| `karmada-create-policy` skill                         | `skills/karmada-create-policy/SKILL.md` + `scripts/generate_policy.py`    |
| `karmada-audit-policy` skill                          | `skills/karmada-audit-policy/SKILL.md` + `scripts/audit_policy.py`        |
| `karmada-debug-propagation` skill                     | `skills/karmada-debug-propagation/SKILL.md` + `scripts/debug_propagation.py` |
| `karmada-explain-placement` skill                     | `skills/karmada-explain-placement/SKILL.md` *(scaffold, week 6-7)*        |
| `karmada-search` skill                                | `skills/karmada-search/SKILL.md` *(scaffold, week 8-9)*                   |
| `karmada-controller-manager` skill                    | `skills/karmada-controller-manager/SKILL.md` *(scaffold, week 9-10)*      |
| Deterministic helper scripts                          | `scripts/extract_crd_schema.py` · `generate_policy.py` · `audit_policy.py` · `debug_propagation.py` |
| Fixtures + example scenarios                          | `examples/00…` · `01…` · `02…` · `03…`                                    |
| Contributor docs for adding skills                    | `docs/CONTRIBUTING.md`                                                    |

---

## What this POC proves

- The skill format works end-to-end. Three workflow skills go from natural-language
  prompt to validated YAML with zero human field-name typing.
- The CRD-to-knowledge sync works. `demo.sh` step 1 regenerates the schema docs
  byte-identically from upstream CRDs.
- The auditor catches every common LLM hallucination listed in the upstream
  proposal — R1, R2, R3, R4, R5, R11, R14 all have failing-fixture coverage.
- The debugger correctly diagnoses the canonical `T-NoSchedule` failure from a
  realistic snapshot bundle.
- All 14 unit tests are green.

## What's left for the mentorship

- Fill in the three scaffolded skills (`karmada-explain-placement`, `karmada-search`,
  `karmada-controller-manager`) — design and helper shapes documented in their
  `SKILL.md` files.
- Add the CI workflow at `.github/workflows/agent-skills.yaml` that runs the test
  suite, asserts schema regeneration is byte-stable, and pipes example YAML through
  `kubeconform` for full OpenAPI validation.
- Extend the knowledge base (search-query cookbook, controller log-pattern map,
  FederatedHPA, state preservation, MultiClusterService).
- Add more examples (cluster-scoped CRDs, custom-CRD `ResourceInterpreter`, FederatedHPA).

---

## Credits

- Project owner & mentors: **@XiShanYongYe-Chang** (Zhen Chang), **@RainbowMango**
  (Hongcai Ren).
- Skill proposal author: **@NickYadance** ([comment 4248950867](https://github.com/karmada-io/community/issues/190#issuecomment-4248950867)).
- Upstream issue: [karmada-io/community#190](https://github.com/karmada-io/community/issues/190).
