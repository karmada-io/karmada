# hack/agent-skills/ — Karmada skill set for AI coding agents

> **Status:** Proof of concept built against the LFX 2026 Term 2 mentorship proposal
> "Build Karmada Agent Skills" (CNCF / karmada-io/community#190). This tree shows the
> shape of the working subsystem, three skills fully implemented with deterministic
> helpers, four skills as scaffolds, and a passing test suite (14 tests).

## Why this exists

> **Want the 30-second version?** Open
> [`examples/00-before-after-comparison/`](examples/00-before-after-comparison/) —
> same prompt, two outputs, side-by-side. That's the entire pitch in one page.

A user pastes a Deployment into an AI coding agent and says "make a Karmada policy
for this, run it in member1 and member2, use a different image registry in member2."
Without a dedicated skill set, the agent guesses the policy shape from training data
and is wrong in mechanical, reproducible ways:

- `replicaScheduling` ends up at `spec.replicaScheduling` instead of under
  `spec.placement.replicaScheduling`
- `apiVersion` comes out as `Apps/v1` or `apps`
- the OverridePolicy `path` is `$.spec.template.spec.containers[0].image` (JSONPath)
  instead of `/spec/template/spec/containers/0/image` (JSON Pointer)
- `resourceSelectors` is empty, which the CRD rejects (MinItems=1)

Each of these is a small mistake. Each costs the user 10 minutes. Every Karmada
operator runs into all of them. This subsystem fixes that for any agent that loads
it — Claude Code, Cursor, Codex, anything that consumes a Markdown-frontmatter skill
format.

## How it is organised

```
hack/agent-skills/
├── README.md                  # you are here
├── demo.sh                    # one-command POC walkthrough
│
├── skills/                    # the seven initial skills
│   ├── karmada-knowledge/        ← read-only reference skill
│   ├── karmada-create-policy/    ← workflow: produce policy YAML from intent
│   ├── karmada-audit-policy/     ← workflow: lint user-supplied policy YAML
│   ├── karmada-debug-propagation/← workflow: walk Object→Policy→RB→Work→Member
│   ├── karmada-explain-placement/← workflow: predict which clusters get replicas (scaffold)
│   ├── karmada-search/           ← component: karmada-search ResourceRegistry (scaffold)
│   └── karmada-controller-manager/ ← component: which controller owns which behavior (scaffold)
│
├── knowledge/                 # shared knowledge base loaded by skills
│   ├── 00-overview.md            ← architecture, components, terms
│   ├── 01-policy-patterns.md     ← reference catalog of placement + override shapes
│   ├── 02-troubleshooting.md     ← T-NoRB, T-NoSchedule, T-NoWork, etc.
│   ├── 03-components.md          ← what each binary does, decision tree
│   ├── 04-hard-rules.md          ← R1–R15 schema constraints LLMs hallucinate
│   ├── 10-propagation-schema.md  ← AUTO-GENERATED from CRD by scripts/extract_crd_schema.py
│   └── 11-override-schema.md     ← AUTO-GENERATED from CRD by scripts/extract_crd_schema.py
│
├── scripts/                   # deterministic helpers — no LLM at runtime
│   ├── extract_crd_schema.py     ← CRD → Markdown schema reference (sync mechanism)
│   ├── generate_policy.py        ← structured intent → PropagationPolicy + OverridePolicy YAML
│   ├── audit_policy.py           ← YAML → JSON findings keyed to R1–R15
│   └── debug_propagation.py      ← snapshot bundle + target → first-failed-hop verdict
│
├── templates/                 # minimal-valid YAML and intent JSON
│   ├── intent.example.json
│   ├── propagationpolicy.minimal.yaml
│   └── overridepolicy.minimal.yaml
│
├── examples/                  # full end-to-end scenarios with expected outputs
│   ├── 01-multi-country-frontend/    ← Singapore/Indonesia/Germany/Brazil, exercises all skills
│   ├── 02-bad-policy-audit-targets/  ← six broken YAMLs, one per common hallucination
│   └── 03-propagation-failure-bundle/← T-NoSchedule snapshot for the debugger
│
├── tests/
│   └── test_helpers.py        ← unittest: 14 tests covering all four helpers
│
└── docs/
    └── CONTRIBUTING.md        ← how to add skills, knowledge, helpers, examples
```

## Try it (one command)

```
bash hack/agent-skills/demo.sh
```

Walks through schema regeneration, policy generation, two audits (one clean, one
broken), a propagation-failure diagnosis, and the unit-test run. Total wall time
~3s on a laptop. No live Karmada cluster needed — the debugger consumes captured
YAML snapshots, which is what makes the whole subsystem CI-friendly.

![demo](docs/media/demo.svg)

> The animated SVG above (`docs/media/demo.svg`, 36 KB) replays the live terminal
> session at full speed. The raw asciicast is at `docs/media/demo.cast` — open it
> with `asciinema play docs/media/demo.cast` to step through frame-by-frame.

## Design decisions worth highlighting

**1. The schema docs are auto-generated, not hand-written.**
`scripts/extract_crd_schema.py` reads the CRD files under
`charts/karmada/_crds/bases/policy/` and emits `knowledge/10-propagation-schema.md`
and `knowledge/11-override-schema.md`. Every CRD bump regenerates them via CI; the
diff is committed in the same PR. The agent never reasons about field names from
training data — it always reads from the in-tree schema.

This is the cure to the central failure mode the proposal author called out: agents
hallucinate fields because they were trained on Karmada v1.6 docs and the user is
running v1.10. With auto-extraction, the skill set is locked to the *exact* CRD
shipped in the same commit.

**2. Deterministic helpers do the structural work; the LLM owns judgment.**
The `karmada-create-policy` skill does not ask the LLM to emit YAML. It asks the
LLM to produce a small structured JSON intent (which clusters, which placement mode,
which overrides), then calls `generate_policy.py` to serialize that intent. The
script refuses to emit YAML if R1, R2, R3, R5, R11, or R14 would be violated.

The remaining rules — R6 (which interpreters exist), R7 (whether `propagateDeps`
will actually work for a custom CRD), R8 (whether `Overwrite` is appropriate),
R10 (which scope to use) — are *semantic* and remain the LLM's job. The split is
deliberate: ask the LLM to do the thing it is good at (read user intent), let the
script do the thing it is good at (mechanical correctness).

**3. The audit skill outputs structured findings, not prose.**
`audit_policy.py` returns JSON with `severity`, `rule`, `path`, `message`. The skill
loader reads this and reformats. A future "auto-fix" workflow (mentorship week 10)
can consume the same JSON.

**4. The debugger operates on captured snapshots, not a live cluster.**
The reality is that 80% of "X did not propagate" questions arrive in a forum post
with `kubectl get -o yaml` output already pasted. The debugger reads that snapshot
and walks the pipeline. No kubeconfig needed; no Karmada cluster needed; trivial
to run in CI against fixtures.

**5. The skill format follows the mature multi-skill pattern.**
Each SKILL.md has YAML frontmatter (`name`, `description`, `metadata`) plus a body
with required sections (When to load, Workflow, Boundaries, Failure modes, Examples).
This is identical to the Claude Skills, Codex skill, and Anthropic Agent Skills
formats — anything that consumes Markdown-frontmatter skills can load this tree
unchanged.

## What this POC proves

- The skill format works: three workflow skills are functional end-to-end with
  deterministic helpers.
- The CRD-to-knowledge sync works: `demo.sh` step 1 regenerates the schema docs
  from the upstream CRDs.
- The auditor catches every common LLM hallucination listed in the proposal (R1
  misplaced replicaScheduling, R5 JSONPath path, R2 empty selectors, R14 deprecated
  fields, R4 mutex affinities, R11 float weights).
- The debugger correctly diagnoses the canonical T-NoSchedule failure from a
  realistic snapshot bundle.
- The 14-test suite is green; replays with `python3 -m unittest tests.test_helpers`.

## What the LFX mentorship would build on top

The scaffolds for `karmada-explain-placement`, `karmada-search`, and
`karmada-controller-manager` document the planned helpers and outputs but stop
short of full implementation. Mentorship weeks 6–10 fill them in. Likewise, the
CI workflow described in `docs/CONTRIBUTING.md` is documented but not yet
committed — mentorship week 2.

The deeper expansion targets are:

- More knowledge files (search queries, controller log patterns, FederatedHPA,
  state preservation for stateful failover, MultiClusterService).
- More examples (cluster-scoped CRDs, FederatedHPA with metrics adapter, mTLS
  cluster registration, ResourceInterpreter for a custom CRD).
- `kubeconform` integration in CI for full OpenAPI validation of every
  example YAML against the in-tree CRDs.
- Extending the auditor with semantic checks (e.g. "this label selector references
  a label that no Cluster carries").

## Credits / context

- Upstream issue: [karmada-io/community#190](https://github.com/karmada-io/community/issues/190)
- Skill proposal author: @NickYadance.
- Mentors named on the LFX listing: @XiShanYongYe-Chang (Zhen Chang),
  @RainbowMango (Hongcai Ren).
- Karmada source paths cited throughout:
  - `pkg/apis/policy/v1alpha1/propagation_types.go`
  - `pkg/apis/policy/v1alpha1/override_types.go`
  - `pkg/apis/work/v1alpha2/binding_types.go`
  - `pkg/controllers/binding/binding_controller.go`
  - `pkg/controllers/execution/execution_controller.go`
  - `charts/karmada/_crds/bases/policy/policy.karmada.io_propagationpolicies.yaml`
  - `charts/karmada/_crds/bases/policy/policy.karmada.io_overridepolicies.yaml`
