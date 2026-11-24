# factory-gate

**A software factory does not produce code, it produces changes that arrive with evidence.**
Agents do the work. A gate decides whether a human ever sees the result. A human who is not
the author approves it. This repository is a working slice of that idea: a model proposes a
change to a small synthetic order-handling service, the gate judges it, a failure goes back
to the model with a counterexample, and only a passing change with its evidence reaches a
person. The gate does not care which model produced a change: the factory fixes the
contract, not the model.

State of the work: `PROGRESS.md`. How it was built and by whom: `BUILD_LOG.md`.
The plan the build followed: `FACTORY_PLAN.md`.

## Quickstart (offline, about a minute)

Needs Go 1.26 and git. No model, no network once the Go modules are downloaded.

```sh
git clone <this repo> && cd <this repo>
make demo                  # press enter between steps; DEMO_NOPAUSE=1 make demo runs straight through
```

The demo replays a recorded loop (`demo/DEFAULT`, currently `run-4`). The model's replies
come from the recording; **the gate runs live on every step**. In a fresh clone it finished
with exit 0 in 115 s and 57 s on two consecutive runs. It shows:

1. the spec, written as a product owner would write it (see the note below);
2. three attempts by the implementer: FAIL, FAIL, ESCALATE, with the failure report each time;
3. the evidence a reviewer reads instead of the diff;
4. a seeded shortcut, where the change weakens an invariant: the gate escalates, and the
   original invariant from base still fails;
5. the audit trail.

The same output, captured from a fresh clone, is in `docs/sample-evidence/`.

## Running live

Any OpenAI-compatible chat-completions endpoint works. Put the base URL and, if needed,
headers in `.env` (names in `.env.example`, referenced from `gate.yaml`), then:

```sh
make doctor                # is the implementer reachable, and how fast
make demo-live             # live loop, recorded into demo/recorded/live-<time>
```

Headers never reach logs, recordings, evidence or the audit file; a test checks that.

## The one design rule

**The gate takes its rules from base, never from the change under test.** `gate.yaml`, the
invariants and the prompts are read from `--base`. Head is checked out into a temporary
worktree and every protected path is overlaid from base before any check runs. A change that
edits its own invariants is judged by the original ones and escalated for touching them.
"An agent cannot change its own gate" is a property of the system, not a sentence in a prompt.

Verdicts, in order: protected path touched → ESCALATE; required check could not run →
ESCALATE; tier-2 path without an approved plan → ESCALATE; new Go module → ESCALATE; blocking
check fails on the last attempt → ESCALATE; blocking check fails → FAIL (back to the agent);
otherwise PASS. `gate run` exits 0 / 1 / 2 for PASS / FAIL / ESCALATE and 3 on internal error.

## Checks

| Check | Built | Blocking | What it does |
| --- | --- | --- | --- |
| build | yes | yes | `go build`, `go vet`, unit tests |
| invariants | yes | yes | property tests from base (`pgregory.net/rapid`), 200 cases, seed 1; the shrunk counterexample goes into the evidence verbatim |
| budget and scope | yes | yes | at most 200 changed non-test lines and 8 files; every file inside the spec's `allowed_paths` |
| independent tests | no | | tests written by a second model that sees only the spec and the exported API |
| security | no | | secret scan, known-vulnerability scan, new-dependency detection |
| mutation | no | report only | mutate changed lines, rerun the checks |

A check is listed in `gate.yaml` only once it exists, so nothing is silently skipped.

## Roles and models

| Role | Model | Where it ran | State |
| --- | --- | --- | --- |
| building this repository | Claude Code (Claude Opus) | Anthropic | done through Phase 4 |
| implementer | `qwen3-coder`, open weights | own L40S GPU host, `completion` adapter, spec-only context | built |
| test author | `bench-qwen36`, a different model | own GPU host | configured, not built |
| planner, advisory reviewer | | | not built; the next roles |

## Results so far

Every recorded run is in `demo/RUNS.md`, generated from the recordings; the rule that picks
the demo run was written before the first run (`demo/CAPTURE_RULE.md`, with three
amendments, each recorded with its reason).

Five valid live runs (`run-4` to `run-8`). In **all five**, the implementer's first attempt
failed the idempotency invariant: a duplicate fill event was applied twice. The spec never
mentions duplicates, so this is exactly the gap the invariants exist to cover. No run reached
PASS within three attempts; after the counterexample the model either repeated the bug or
returned no change (`agent_error`). Every broken change was stopped before a human saw it.
Gate evals (`evals/RESULTS.md`) are not built yet.

## What is live, recorded, replayed and seeded

Every demo artefact carries a `source:` label.

- **live**: a model call made now (`make demo-live`), and the gate on every run.
- **recorded**: `demo/recorded/run-*`, live model replies saved at the model-call boundary.
- **replayed**: `make demo` feeds those replies back; a changed prompt makes the replay stop as stale.
- **seeded**: `demo/attempts/weakened-invariant.patch`, written by hand to show the shortcut.

## A note on the sample spec

`specs/partial-fills.md` is written as a product owner would write it. It does not mention
duplicate events or idempotency, on purpose: real tickets do not list every invariant, and
that gap is what the gate exists to cover. The invariants in `sample/orders/invariants/`
cover it instead.

## Limits

- The gate is only as good as its invariants. With none written, it catches nothing that tests would not.
- The invariants here are illustrative. Real ones have to come from the people who know the domain.
- Passing the gate is not proof of correctness. It is evidence that named properties held under a stated number of generated cases with a recorded seed.
- Loop results vary from run to run. One demo run proves nothing by itself; `demo/RUNS.md` is the honest picture.
- The live path needs the author's hardware or any OpenAI-compatible endpoint a reader points it at. Without one, the repo runs from recordings.
- The `completion` implementer rewrites whole files and cannot run tests itself. That suits one small package; a large codebase needs a patch protocol or an agentic harness.
- An independent test author may share blind spots with the implementer. A different model and a spec-only context reduce this; they do not remove it.
- Identities are asserted, not authenticated. In production, requester and approver come from the platform: signed commits, required reviewers, branch protection.
- The audit log is a local file standing in for the platform's record.
- One language, one repository, one task.

## Not built yet

Security check and `gate approve` (Phase 5), the independent test author (Phase 6), gate
evals and the model matrix (Phase 7), the mutation check (Phase 8), a CI workflow running the
gate in advisory mode on pull requests. `make demo-live` has not yet been run end to end.

## Layout

`cmd/gate/` CLI. `internal/gate/` diff, worktree, checks, verdict, evidence, audit.
`internal/agent/` role contract, completion adapter, record/replay. `internal/loop/` the
attempt loop. `sample/orders/` the service under test; `sample/orders/invariants/` the
human-owned property tests. `specs/`, `prompts/`, `demo/`. Rules for agents: `AGENTS.md`.
