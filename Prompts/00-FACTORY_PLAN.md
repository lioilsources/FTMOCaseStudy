# FACTORY_PLAN.md: Evidence Gate prototype

Handoff plan for the executing agent (Claude Code: the Director session and its sub-sessions).
Read this file and `PROGRESS.md` at the start of every session. Owner: Olin.

## 1. What we are building and why

This repository is the working slice of a take-home case study: design a software factory for a regulated trading company, and build a small piece of it.

The design thesis is one sentence: **a software factory does not produce code, it produces changes that arrive with evidence.** Agents do the work. A gate decides whether a human ever sees the result. A human who is not the author approves it.

The slice is that gate plus the loop around it: a model proposes a change, the gate judges it, a failure goes back to the model, and only a passing change with its evidence reaches a person. It is shown on a small synthetic order-handling service and answers three people from the case study:

- the senior engineer whose agents write plausible code that breaks invariants in order handling, and who spends longer reviewing than writing;
- the early adopter whose large PRs are tested by the same agent that wrote them;
- the security and compliance lead who needs an author, a different approver and an audit trail for every change.

**Two kinds of model work are kept apart.** You (Claude Code) build this repository. The agents the factory itself runs, the implementer and the test author, are local open-weights models on Olin's own hardware. The gate does not care which model produced a change. That is the point: the factory fixes the contract, not the model.

The readers of this repo are hiring engineers. They check that it runs, they read the scaffolding (agent instruction files, prompts, configs, tests, evals), and they read a build log of what was delegated, reviewed and written by hand. Optimise for that reader: a narrow slice that works and states its limits is worth more than a broad one that does not. Clear names, small files, no cleverness.

## 2. Ground rules

1. **Synthetic data only.** The sample domain is a generic "orders" service. No real company names, logos, data or internal details anywhere in the repo, including this plan, comments, fixtures and commit messages.
2. **No secrets, including fake ones.** Never commit a secret-shaped literal. Tests that need one assemble it at runtime (see Phase 5).
3. **Endpoints and credentials live in the environment only.** Base URLs and auth headers for the local models come from environment variables named in `gate.yaml`. `.env` is gitignored; `.env.example` lists variable names with empty values. Request headers are never written to logs, recordings, evidence or the audit file. There is a test for that.
4. **Never weaken a check to make something pass.** If a check is wrong, say so in `PROGRESS.md` and stop.
5. **Never present seeded or replayed material as natural or live.** Every demo artefact carries a `source:` label: `live`, `recorded`, `replayed` or `seeded`.
6. **No cherry-picking without disclosure.** Which recorded run ships as the default demo is decided by a rule written before the runs (Phase 4), and every run is listed.
7. **Every number in README, BUILD_LOG or RESULTS comes from a command run in this repo.** No estimates, no remembered figures.
8. **Reality beats this plan.** Tool flags and library APIs named here were checked against public docs on 2026-11-19, but versions move. Check `--help` on the installed version before relying on a flag. When the plan and reality disagree, follow reality and record the deviation in `PROGRESS.md`.
9. **Minimal dependencies.** Standard library, a YAML parser, and `pgregory.net/rapid`. Anything else: ask first. The gate treats a new dependency as a human decision, and so do we.
10. **Money and quantities are `int64` minor units.** No floats in `sample/`.

## 3. Decisions already taken

Olin owns these. Change them here, not in code.

| Decision | Value |
| --- | --- |
| Who builds the repo | Claude Code (Opus), directed by Olin |
| Who runs inside the factory | Local models from Olin's SwarmBattle stack, served on his DGX Spark and an L40S machine. No hosted model is called by the gate |
| Which role runs where | Set per role in `gate.yaml` by Olin. Suggested start: implementer on one machine, test author on the other, with a different model |
| Language | Go, one module (`github.com/lioilsources/factory-gate`, rename freely) |
| Repo language | English |
| Demo | `make demo` replays a recorded loop run offline with the gate running live. `make demo-live` runs the loop against the local models |
| Property testing | `pgregory.net/rapid` |
| Target | Cut line A fits the four-hour budget. Cut line B is the better story if Olin spends an extra hour |

## 4. Delegate, review, own

The split the factory proposes also governs how this repo is built.

| Level | Who | What |
| --- | --- | --- |
| Own | Olin only | The three invariant assertions. `gate.yaml`: thresholds, protected paths, which model plays which role. The human sections of `BUILD_LOG.md`. Scope cuts. The recording |
| Review | Olin, at each STOP | Every phase before the next one starts. Any change to a protected path |
| Delegate | You | Everything else |

At a **STOP**: commit, update `PROGRESS.md`, write a summary of at most 10 lines saying what to look at and what you are unsure about, then wait. Do not start the next phase.

## 5. Target shape

### 5.1 Layout

```
AGENTS.md              rules for agents in this repo (CLAUDE.md imports it)
CLAUDE.md              one line: @AGENTS.md
FACTORY_PLAN.md        this file
PROGRESS.md            checkpoint: phase status, decisions, next step
BUILD_LOG.md           sessions, failures, human sections
README.md
gate.yaml              protected paths, tiers, budgets, checks, endpoints, roles
.env.example           variable names only
Makefile               test, gate, demo, demo-live, eval
.claude/settings.json  deny rules for protected paths
cmd/gate/              CLI entry point
internal/gate/         diff, worktree, checks, verdict, evidence, audit
internal/agent/        role contract, completion adapter, command adapter, record/replay
sample/orders/         the sample service under test
sample/orders/invariants/   human-owned property tests (protected)
specs/                 task specs with front matter
prompts/               implementer.md, test-author.md
evals/                 static cases, loop matrix config, generated RESULTS.md
demo/                  demo.sh, recorded/<run>/, attempts/*.patch, RUNS.md
docs/sample-evidence/  committed evidence from the demo run
.factory/              runtime output, gitignored
```

### 5.2 CLI

```
gate run   --base <ref> --head <ref> [--spec <file>] [--mode blocking|advisory]
           [--attempt N] [--replay <dir>] [--requested-by <name>]
gate loop  --spec <file> --base <ref> [--max-attempts 3] [--context spec-only|spec+invariants]
           [--record <dir> | --replay <dir>] [--stream]
gate doctor                       # are the configured roles reachable, and how fast
gate approve --run <id> --by <name>
gate eval  [--matrix]             # Phase 7
```

`gate run` exits 0 for PASS, 1 for FAIL, 2 for ESCALATE, 3 for an internal error. It writes `.factory/runs/<run-id>/` (`evidence.md`, `evidence.json`, `failure_report.md`, `logs/`) and appends one line to `.factory/audit.jsonl`.

### 5.3 The one design rule that matters

**The gate takes its rules from base, never from the change under test.**

`gate.yaml`, the invariants directory and the prompts are read from `--base`. Head is checked out into a temporary `git worktree`, and every protected path is overlaid from base before any check runs. A change that edits its own invariants is therefore judged by the original invariants, and escalated for touching them. This makes "an agent cannot change its own gate" a property of the system, not a sentence in a prompt.

### 5.4 Verdict rules, evaluated in this order

1. The diff touches a protected path: **ESCALATE**.
2. A required check cannot run (tool missing, timeout, inconclusive): **ESCALATE**. There are no silent skips.
3. A tier-2 path changed and the spec has an empty `plan_approved_by`: **ESCALATE**.
4. `go.mod` gains a module: **ESCALATE**.
5. A blocking check fails and `--attempt` has reached `max_attempts`: **ESCALATE**.
6. A blocking check fails: **FAIL**. It goes back to the agent, not to a human.
7. Otherwise: **PASS**.

`advisory` mode computes and prints the same verdict but exits 0. Run all checks even after an earlier one fails, so one report shows everything. The only exception is a failed build.

### 5.5 Checks

| # | Check | Blocking | How | Evidence it produces |
| --- | --- | --- | --- | --- |
| 1 | build | yes | `go build`, `go vet`, existing unit tests | failing packages and tests |
| 2 | invariants | yes | property tests from base, fixed number of checks, recorded seed | the shrunk counterexample, verbatim, plus the seed |
| 3 | independent tests | yes | tests written by a second model that sees only the spec and the exported API | both model ids, `live` or `replayed`, failing test names |
| 4 | mutation | report only | mutate changed lines, rerun checks 1 to 3 | killed / total, surviving mutant locations |
| 5 | security | yes | secret scan over `base..head`, known-vulnerability scan, new-dependency detection | findings |
| 6 | budget and scope | yes | changed non-test lines and file count against limits; every file inside the spec's `allowed_paths` | numbers against limits, out-of-scope files |

A check is `required: true` in `gate.yaml` only once it exists. Until then it is absent from the config, not skipped.

### 5.6 Agent roles and the contract

| Role | Used by | Sees | Can do | Built |
| --- | --- | --- | --- | --- |
| implementer | `gate loop` | the spec, current files under `allowed_paths`, the last `failure_report.md`; with `spec+invariants` also the invariant sources | propose file contents | Phase 3 |
| test author | check 3 | the spec and the exported API, nothing else | propose one test file | Phase 6 |
| planner, advisory reviewer | not used | | | not built; name them in the README as the next roles |

A role is bound to a model in `gate.yaml`:

```yaml
endpoints:
  spark: { base_url_env: FACTORY_SPARK_URL, headers_env: FACTORY_SPARK_HEADERS }
  l40s:  { base_url_env: FACTORY_L40S_URL,  headers_env: FACTORY_L40S_HEADERS }
roles:
  implementer:
    kind: completion          # or: command
    endpoint: spark
    model: ""                 # Olin fills in
    ladder: []                # optional: one model id per attempt, weakest first
    context: spec-only
    timeout_seconds: 180
  test_author:
    kind: completion
    endpoint: l40s
    model: ""                 # a different model from the implementer
```

Two adapter kinds implement the same contract: given a role, a prompt and a working directory, leave proposed files in that directory and return model id, token counts and duration.

- **`completion`**: one request to an OpenAI-compatible chat endpoint. The reply is parsed with a plain delimiter protocol (`=== FILE: <relative path> ===`, full content, `=== END FILE ===`). Do not depend on JSON mode or tool calling; local servers differ. Strip reasoning blocks. Reject absolute paths and paths that leave the working directory. Run `gofmt` on what is written. A reply that does not parse is an `agent_error`: retry once, then count the attempt as failed with that reason, kept separate from gate FAIL in all statistics.
- **`command`**: an external agent CLI run inside the worktree, for SwarmBattle through OpenCode's non-interactive mode. Olin supplies the invocation at STOP H2. Check `opencode run --help` for the model, agent and JSON-output flags on the installed version.

With `completion`, the model has no shell, no network and no credentials. Its only capability is proposing file contents; every action is taken by the gate. With `command`, the agent has a shell in the worktree and that property is lost. Say which one produced each run.

**Record and replay** sit at the model-call boundary. `--record <dir>` stores each call: role, model id, endpoint label, prompt hash, messages, reply, token counts, duration. `--replay <dir>` serves calls from a recording by sequence and prompt hash, and stops with "recording is stale" on a mismatch. The gate always runs live; only model output is replayed. This one mechanism gives the offline demo, the backup for the session, and reproducibility for readers without the hardware.

### 5.7 Evidence

`evidence.md` is what a reviewer reads instead of starting from the diff. Sections, in order:

1. Verdict and one sentence of why.
2. Check table: status, duration, the key number for each.
3. Counterexample or findings, verbatim from the tool.
4. **Read these first**: at most three locations, chosen by a fixed rule and never by a model. Order: lines with surviving mutants, hunks in tier-2 paths, the largest hunk.
5. Provenance: base and head SHA, requester, role, model id, endpoint label and prompt hash from the commit trailers, spec path and hash, attempt number, gate version, mode, and `live` or `replayed`. When implementer and test author are the same model, say "same model: reduced independence".
6. What this gate cannot tell you (static text, see Section 9).

`evidence.json` carries the same content for machines.

### 5.8 Audit record and authorship

One JSON object per line in `.factory/audit.jsonl`:

```json
{"run_id":"r-0007","ts":"2026-11-20T09:14:03Z","kind":"gate_run","requested_by":"olin",
 "agents":[{"role":"implementer","kind":"completion","model":"...","endpoint":"spark",
            "prompt_sha256":"...","tokens_in":0,"tokens_out":0,"seconds":0}],
 "spec":{"path":"specs/partial-fills.md","sha256":"..."},
 "base":"<sha>","head":"<sha>","attempt":1,"files":["sample/orders/engine.go"],
 "checks":[{"id":"invariants","status":"fail","ms":412}],
 "verdict":"FAIL","mode":"blocking","source":"live","gate_version":"<sha>"}
```

Tokens and seconds are the cost measure for local models. Do not invent a currency figure.

The requester defaults to `git config user.name` and can be set with `--requested-by`.

`gate approve` appends a record of kind `approval`. It refuses when the approver equals the requester and when the run is not PASS.

The author of record is always a human. A model-made commit keeps the human as git author and names the tool in trailers:

```
Agent: implementer/completion
Model: <model id>
Endpoint: spark
Prompt-SHA256: <hash of the prompt file>
Run-ID: <id>
Requested-By: <human>
```

## 6. Phases

Budgets are wall-clock minutes of agent work. If a phase runs past twice its budget, stop and report: cutting scope is Olin's call.

| Phase | Minutes | Cumulative | Ends with |
| --- | --- | --- | --- |
| 0 Scaffold and rules | 10 | 10 | |
| 1 Sample service and invariant harness | 30 | 40 | STOP H1 |
| 2 Gate core | 40 | 80 | STOP H2 |
| 3 Agent adapters and the loop | 45 | 125 | |
| 4 Demo: recorded and live | 30 | 155 | STOP H3, **cut line A** |
| 5 Security check and approval | 15 | 170 | |
| 6 Independent tests on a second model | 20 | 190 | |
| 7 Gate evals | 20 | 210 | STOP H4, **cut line B** |
| 8 Mutation check (stretch) | 20 | 230 | |
| 9 Wrap-up (mandatory after whichever cut line) | 15 | | STOP H5 |

### Phase 0: Scaffold and rules

- `git init`, `go mod init`, the layout from 5.1 as empty packages, `.gitignore` with `.factory/` and `.env`, `.env.example`.
- `AGENTS.md`, at most 60 lines: repo map, the commands from the Makefile, the ground rules from Section 2, the trailer convention, the PROGRESS protocol, and the list of protected paths with the sentence "you may read these, you may not change them".
- `CLAUDE.md` containing only `@AGENTS.md`.
- `.claude/settings.json` with `permissions.deny` rules of the form `Edit(/sample/orders/invariants/**)` for each protected path. Check the rule syntax against the current permissions docs. Add a comment in `AGENTS.md` that these rules stop the edit tools but not shell writes, so the gate's diff check is the real enforcement.
- `PROGRESS.md` and `BUILD_LOG.md` from the templates in Section 7.
- Record `go version`, `claude --version`, `gitleaks version`, `govulncheck -version` and `opencode --version` in `BUILD_LOG.md`. A missing tool is a line in the log, not a blocker yet.

Done when: `go build ./...` passes and the first commit exists.

### Phase 1: Sample service and invariant harness

- `sample/orders`: `Account` (balance, reserved), `Order` (id, side, quantity, price, filled quantity, status), events `PlaceOrder`, `Fill`, `Cancel`, and `Engine.Apply(event) error`. Baseline behaviour: a fill must equal the full remaining quantity. Under 250 lines, so that a local model can read and rewrite the package in one call.
- Example-based unit tests for the baseline.
- `sample/orders/invariants/harness_test.go`, an external test package using only the exported API: `rapid` generators for event sequences that include duplicates, replays, fills after cancel and unknown ids; a runner that applies a sequence and calls three check functions after every step; snapshot and equality helpers.
- `sample/orders/invariants/invariants_test.go`: three stubs with doc comments and `t.Skip("owner: human")`: `checkNoOverfill`, `checkNoNegativeBalance`, `checkIdempotentEvents`. **Do not implement them.** Shape the signatures so each body fits in about ten lines. The idempotency check has to re-apply the last event to a copy of the engine and compare snapshots, so the harness must offer both.
- `specs/partial-fills.md` with front matter `id`, `tier: 2`, `allowed_paths`, `plan_approved_by: ""`, then intent, acceptance criteria and out-of-scope. Write the criteria as a product owner would. Do not mention duplicate events or idempotency: real tickets do not list every invariant, and that gap is what the gate exists to cover. Say so in the README.

**STOP H1.** Olin writes the three assertions by hand, runs the invariants against the baseline (they must pass), reviews the generators and fills in `plan_approved_by`. Explain the harness API to him in at most 10 lines. From this commit on, the invariants directory is protected.

### Phase 2: Gate core

- Config loaded with `git show <base>:gate.yaml`. Worktree checkout of head. Overlay of protected paths from base. Diff stats from `git diff --numstat` and `--name-status`.
- Checks 1, 2 and 6, the protected-path rule, the tier and plan-approval rule, verdict, `evidence.md` and `evidence.json`, audit append, exit codes, `--mode`.
- On FAIL, write `failure_report.md`: the failing checks and their evidence, nothing else, and no suggested fix. This is what goes back to the model.
- Run the invariants with a fixed check count and seed from `gate.yaml` (verify the `rapid` flag names). Capture the failing output verbatim. Do not parse it beyond locating the block.
- Draft `gate.yaml`: protected paths (`sample/orders/invariants/**`, `gate.yaml`, `AGENTS.md`, `CLAUDE.md`, `.claude/**`, `prompts/**`, `cmd/gate/**`, `internal/**`, `evals/**`, `.github/**`), tiers (`sample/orders/**` is tier 2, docs are tier 0, default 1), budget (200 changed non-test lines, 8 files), `max_attempts: 3`, and the `endpoints` and `roles` blocks from 5.6 with empty model ids.
- Table-driven unit tests for verdict logic and path matching.

Done when, on scratch branches: a README-only change gives PASS; a deliberate overfill bug gives FAIL with a counterexample in the evidence; an edit to the invariants gives ESCALATE while the original invariants still run; `audit.jsonl` has three lines.

**STOP H2.** Olin reviews `gate.yaml`, which he owns, and reads one `evidence.md` end to end. He also provides what Phase 3 needs: the environment variables for both endpoints set in his local `.env`, a model id per role, and how SwarmBattle is invoked from the command line if he wants the `command` adapter.

From here on, run `gate run --mode advisory` on your own phase branch at each STOP and paste the verdict into `PROGRESS.md`. ESCALATE is the expected verdict for changes to the gate itself.

### Phase 3: Agent adapters and the loop

Build in this order, so that every step is testable without the hardware.

1. The role contract and config loading in `internal/agent`.
2. The `completion` adapter, tested against a fake server from `net/http/httptest`: happy path, reasoning block stripped, unparseable reply, path escape rejected, timeout, headers never logged.
3. Record and replay as in 5.6, with a test that a recording contains no header values.
4. `prompts/implementer.md`: the role, the file protocol with one worked example, "return complete files", "change only what the spec needs". Keep it short; local models follow short prompts better.
5. `gate loop`: for attempt N, create a worktree from base (attempt 1) or from the previous attempt; build the prompt from the spec, the current files under `allowed_paths` and the previous `failure_report.md`; call the implementer; write and format the files; commit with the trailers from 5.8 on `loop/<run-id>/attempt-N`; call `gate run --attempt N`. PASS stops. FAIL starts the next attempt. ESCALATE stops. With `ladder` set, attempt N uses the Nth model id.
6. An end-to-end loop test with scripted fake-server replies: a buggy change, then a fixed one. Expected verdicts FAIL, PASS.
7. `gate doctor`: for each configured role, resolve the environment, list models or make a one-token request, print reachability and latency. Non-zero exit when a role is unreachable.
8. `--stream` prints model output as it arrives, so a watching audience sees progress.
9. Only if Olin supplied the invocation: the `command` adapter for SwarmBattle. It is a thin wrapper. Do not rebuild his swarm here.

Done when: the fake-server loop test passes; `gate doctor` is green against Olin's endpoints; one live loop run completes with `--record`; replaying that recording reproduces the same verdict sequence.

If the endpoints are not reachable at this point, do not block: finish with the fake server, mark the live items open in `PROGRESS.md`, and tell Olin.

### Phase 4: Demo: recorded and live

- **The capture rule, fixed before any run:** make five live loop runs on the spec with `--context spec-only --record`. List every run in `demo/RUNS.md`, generated, never hand-edited: run id, model ids, context, verdict per attempt, tokens, seconds. Ship as the default recording the first run whose verdicts are FAIL then PASS. If no run has that shape, ship the first run as it is and tell Olin; a run that ends in ESCALATE is a legitimate result to show, and five first-attempt passes are a finding too. Do not adjust the prompt or the spec to provoke a failure.
- **The shortcut case** stays a patch: `demo/attempts/weakened-invariant.patch` with `source: seeded`. It makes a failing change green by weakening an invariant test. The `completion` implementer is unlikely to produce it, so it is seeded and labelled.
- `demo/demo.sh` for `make demo`, fully offline: show the spec; replay the default recording through `gate loop --replay` so that each attempt's diff, the gate's verdict and the failure report sent back are visible; open the final evidence; apply the seeded patch and show ESCALATE; show the audit lines; `gate approve` refused for the requester and accepted for someone else (once Phase 5 exists). It prints each command before running it, pauses between steps, and finishes in under three minutes.
- `make demo-live`: `gate doctor`, then `gate loop --stream --record` against the local models. If `doctor` fails, it says so and points to `make demo`.
- Commit the evidence files of the default run to `docs/sample-evidence/`.

Done when: `make demo` runs clean twice in a row from a fresh clone in a temporary directory with no network; `make demo-live` completes once end to end.

**STOP H3, cut line A.** Olin runs both demos himself. The prototype is presentable from here. Everything below is ordered by value.

### Phase 5: Security check and approval

- Check 5. Secrets: `gitleaks git --log-opts="<base>..<head>"` (older versions use `detect`; check `--help`). Vulnerabilities: `govulncheck ./...`, which needs the network; when the database is unreachable the result is inconclusive, and because `vuln_scan_required` is false in `gate.yaml` the evidence says "not verified offline" instead of escalating. New dependencies: compare the `require` blocks of `go.mod` at base and head.
- `gate approve` as specified in 5.8.
- Test fixture for secrets: build the secret-shaped string at test time from parts, apply it in a temporary worktree, and confirm the installed scanner detects it. A committed literal would trip the host's push protection and our own scan.

Done when: the runtime-built secret gives FAIL on check 5; a new module in `go.mod` gives ESCALATE; approval by the requester is refused and by another name is accepted.

### Phase 6: Independent tests on a second model

- `prompts/test-author.md`.
- The gate builds the test author's prompt from the spec and the output of `go doc -all` for the head version of the package: signatures and doc comments, no bodies. Nothing else is sent. The test author cannot be influenced by the implementation because it never receives it.
- The reply is one file, `independent_test.go`, through the same `completion` adapter and the same record and replay.
- The gate copies it into the head worktree and runs it. A compile failure triggers one regeneration, then counts as inconclusive.
- The evidence names both models. Same model on both roles is allowed but flagged as reduced independence.
- Set `independent: required: true` in `gate.yaml` and ask Olin to confirm, since the file is his.

Done when: independent tests exist for the default recording's attempts and their result is reported truthfully. It is acceptable for only the invariants to catch the bug. `make demo` still runs offline.

### Phase 7: Gate evals

The gate is itself measured. Without this, "the gate works" is another thing that only looks right.

**Static cases**, deterministic, no model involved:

- `evals/cases/`: each case has a name, a patch applied to main in a temporary worktree, an expected verdict and the check expected to fire.
- Bad cases, target six to eight: overfill, duplicate fill applied twice, fill after cancel, negative balance through a reserve-release bug, weakened invariant, out-of-scope file or oversized diff, runtime-built secret, new dependency.
- Good cases, target three: correct partial fills, a pure refactor, a docs-only change.
- **If a bad case passes the gate, do not tune the gate to the case.** Record the miss and tell Olin. A documented miss is more useful to the presentation than a perfect table.

**Loop matrix**, unattended, the reason local models are worth having here: runs cost time, not money.

- `gate eval --matrix` runs the loop N times per cell over implementer model and context profile (`spec-only`, `spec+invariants`). Start with N = 5 and whatever models Olin lists in `evals/matrix.yaml`.
- Per cell: first-attempt pass rate, mean attempts to PASS, escalations, agent errors, tokens, seconds.
- It runs in the background and can be interrupted; partial results are written with the run count stated.

`gate eval` writes both tables to `evals/RESULTS.md` with the date and gate version. Never edit that file by hand.

Done when: `make eval` regenerates the static table and exits non-zero on any unexpected verdict; the matrix has produced at least one complete cell.

**STOP H4, cut line B.**

### Phase 8: Mutation check (stretch, strict timebox)

- Use an existing tool only if the installed version has a diff-scoped mode (check `gremlins unleash --help`). Otherwise write a minimal mutator over changed lines in non-test files with five operators: `<` and `<=`, `>` and `>=`, `==` and `!=`, `+` and `-`, `&&` and `||`.
- One mutant at a time in a temporary copy, rerunning checks 1 to 3. Cap at 30 mutants and 120 seconds. Report only: killed, total, surviving locations.
- If it is not working after 20 minutes, remove it and say so in the limits.

### Phase 9: Wrap-up (mandatory)

- `README.md`: the thesis in one paragraph; a 60-second quickstart with `make demo`; how to run live against any OpenAI-compatible endpoint; the design rule from 5.3; the check table; the role table and which model played which role; `demo/RUNS.md` and `evals/RESULTS.md`; what is live, recorded, replayed and seeded; the limits from Section 9; what is not built.
- Complete the agent-maintained sections of `BUILD_LOG.md`. Leave the human sections empty.
- Stretch: `.github/workflows/gate.yml` running `gate run` in advisory mode on pull requests and posting `evidence.md` as a comment.

**STOP H5.** Olin writes his sections of the build log and records the demo.

## 7. Session protocol

**Start of session:** read this plan and `PROGRESS.md`. Append a session row to `BUILD_LOG.md`.

**End of phase or session:** update `PROGRESS.md`, commit.

`PROGRESS.md`:

```
## Status
| Phase | State | Commit | Notes |
## Decisions and deviations from the plan
## Open questions for Olin
## Next step (one sentence, concrete enough to resume cold)
```

`BUILD_LOG.md`:

```
## Tool versions
## Models
| Purpose | Model | Where it ran |
(building the repo; implementer; test author)
## Sessions (agent-maintained)
| Start | End | Phase | Model | Delegated task | Outcome |
## What went wrong (agent-maintained, factual: failed approaches, rework, wrong assumptions)
## Plan provenance
Drafted with Claude in a chat session from my outline. [Olin: confirm or correct]
## What I reviewed (Olin)
## What I wrote by hand (Olin)
## Where I overrode the agent (Olin)
## Time spent and why (Olin)
```

**Parallel work.** Phases 0 to 4 are sequential. After H3, Phases 5, 6 and 7 touch separate packages and can run as parallel sub-sessions, each on its own branch, merged by the Director. Two sessions never edit `internal/gate/verdict*` or `gate.yaml` at the same time.

## 8. Done checklist, against the assignment

- [ ] It runs: `make demo` from a fresh clone, offline, twice in a row.
- [ ] It runs live: `make demo-live` against the local models, with `gate doctor` as the pre-flight.
- [ ] Readable scaffolding: `AGENTS.md`, `CLAUDE.md`, `prompts/`, `gate.yaml`, tests, `evals/`.
- [ ] Build log: sessions, models, failures, human sections filled in by Olin, real time spent.
- [ ] Synthetic data only: a case-insensitive search for real company names returns nothing.
- [ ] No secret-shaped literals and no endpoint credentials: the gate's own secret scan over the full history is clean, and no recording contains a header value.
- [ ] Every artefact in `demo/` carries a `source:` label, and `demo/RUNS.md` lists every run.
- [ ] README states the limits.

## 9. Limits to state plainly

Put these in the README and in the static section of every `evidence.md`.

- The gate is only as good as its invariants. With none written, it catches nothing that tests would not.
- The invariants here are illustrative. Real ones have to come from the people who know the domain.
- Passing the gate is not proof of correctness. It is evidence that named properties held under a stated number of generated cases with a recorded seed.
- Loop results vary from run to run. One demo run proves nothing by itself; `demo/RUNS.md` and the matrix are the honest picture.
- The live path needs Olin's hardware, or any OpenAI-compatible endpoint a reader points it at. Without one, the repo runs from recordings.
- The `completion` implementer rewrites whole files and cannot run tests itself. That suits one small package; a large codebase needs a patch protocol or an agentic harness.
- The independent test author may share blind spots with the implementer. A different model and a spec-only context reduce this; they do not remove it.
- Identities are asserted, not authenticated. In production, requester and approver come from the platform: signed commits, required reviewers, branch protection.
- The audit log is a local file standing in for the platform's record.
- With the `command` adapter the agent has a shell in the worktree and is not sandboxed. There is no separate agent identity and no scoped credentials.
- One language, one repository, one task.
