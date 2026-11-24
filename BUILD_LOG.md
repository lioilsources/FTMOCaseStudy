# BUILD_LOG.md

## Tool versions

Recorded at Phase 0 on 2026-11-19.

| Tool | Version |
| --- | --- |
| go | go1.26.0 darwin/arm64 |
| claude | 2.1.328 (Claude Code) |
| gitleaks | not installed |
| govulncheck | not installed |
| opencode | 1.16.2 |

## Models

| Purpose | Model | Where it ran |
| --- | --- | --- |
| Building the repo | Claude Opus 5.5 (Claude Code) | hosted, directed by Olin |
| Implementer | qwen3-coder (Qwen3-Coder-30B-A3B-Instruct FP8, vLLM) | L40S VM, via an SSH forward |
| Test author | bench-qwen36 (Qwen3.6-35B-A3B), not used yet | DGX Spark, via ai-gateway (Phase 6) |

## Sessions (agent-maintained)

| Start | End | Phase | Model | Delegated task | Outcome |
| --- | --- | --- | --- | --- | --- |
| 2026-11-19 20:12 | | 0, 1 | Claude Opus 5.5 | Scaffold; sample service and invariant harness | STOP H1 reached |
| 2026-11-20 | | 2 | Claude Opus 5.5 | Gate core | STOP H2 reached |
| 2026-11-20 03:00 | 07:50 | 3, 4 | Claude Opus 5.5 | Agent adapters, record/replay, loop, doctor; live capture; demo | STOP H3 reached |
| 2026-11-24 05:50 | | 7, 9 | Claude Opus 5.5 | README; static gate evals (sub-agent in a worktree); draft of the human sections below; presentation draft | in progress |

## What went wrong (agent-maintained, factual: failed approaches, rework, wrong assumptions)

- Phase 1: the plan's stub shape (`t.Skip` in each check) does not work inside `rapid.Check`; rapid treats a skip as an invalid case and fails after too many. Fixed with a probe on the outer test (see PROGRESS).
- Phase 4: run-1 escalated on a build-check timeout: the build check ran the gate's own end-to-end tests (175 s for one package). Fixed with `go test -short`.
- Phase 4: run-2 escalated on an invariants timeout after a 540 s build: the repository volume was slow (a 50 MB write took 62 s while a backup ran; `go list` 24 s at 2 % CPU). Fixed by putting check worktrees and their build cache on the local temp disk (build 7 s).
- Phase 4: after an agent error the next prompt carried only the agent-error note, not the last gate report; seen in runs 5 to 8. Fixed after the capture.
- Phase 4: the seeded shortcut was first labelled `source: live` in its evidence. Fixed with `gate run --source seeded` before the sample evidence was taken.
- Phase 4: an earlier scripted edit of the Makefile replaced a match inside an echo string and broke two targets; rewritten.
- Phase 4: the agent told the L40S session "33 requests" before counting; the recordings hold 19. Corrected in a follow-up message.
- Phase 2: two-dot diff from the base tip blamed a branch for commits that landed on main after it forked. Found by Olin running the gate himself; fixed to diff from the merge base, with a regression test.
- Phase 2: the first counterexample capture cut the block at the wrong place (with `-v`, rapid prints the steps before the `--- FAIL` line); fixed and covered by a test.
- Phase 2: the invariants check reported 0 ms because the deferred timer wrote to a copy; fixed with a named return.
- Phase 2: the first fixture used a plain Go test as the invariants; `-rapid.*` flags made it fail to start, so it reported inconclusive. The fixture now uses rapid.
- Phase 0: denying edits on all protected paths at once would have blocked the delegated work that creates them. Deny rules are added per path when it becomes human-owned.

## Plan provenance

Drafted with Claude in a claude.ai chat on 2026-11-19, from the assignment and my stance-and-outline
document (thesis, problem order, design, prototype scope, demo, limits, time budget). The plan was
then handed to Claude Code as `Prompts/00-FACTORY_PLAN.md`. I pasted the assignment PDF into the
chat, talked it through for a while, and chose the path that seemed to matter to the largest number
of the people in the brief.

> The four sections below were drafted by the agent from the session transcript and the commit
> history, at my request; I corrected and completed them.

## What I reviewed (Olin)

- At each STOP (H1, H2, H3) I read the summary and `PROGRESS.md` and answered the open questions.
- Phase 1: ran `make invariants` after the assertions were in place (ok, 200 cases, seed 1).
- Phase 2: ran the gate myself on a scratch branch (`gate run --base main --head scratch/overfill`,
  run r-0003). The evidence blamed my branch for commits that had landed on main after it forked:
  the gate diffed against the base tip instead of the merge base. Reported it; fixed with a
  regression test (commit `6830582`).
- Phase 4: read the capture rule before the live runs and each amendment before it was applied.
- The agent configuration and the invariants.

## What I wrote by hand (Olin)

- `gate.yaml`: model ids per role, the role swap (implementer on the L40S coder, test author on
  SPARK), the fix of a typo (`rroles:`) that broke the config.
- `specs/partial-fills.md`: `plan_approved_by` (the tier-2 plan approval).
- The invariant assertions in `sample/orders/invariants/invariants_test.go`: **drafted by the
  agent, pasted and committed by me** (the edit tools are denied on that path), with my own
  comments. I did not write the assertion logic from scratch.
- `.env` with the endpoints (not in the repo).
- I barely opened an IDE. Apart from the gate config and the invariants, which I had to understand
  and change, I did not look at the files.

## Where I overrode the agent (Olin)

- Default demo recording: the capture rule named `run-1`; I chose `run-4`, the first run made with
  the final gate that replays to the end (`demo/CAPTURE_RULE.md`, amendment 3).
- After `run-1` timed out on infrastructure, kept it listed and ran `run-2` to `run-6` with the
  fixed gate instead of replacing it (amendment 1); after the slow-disk failure, moved check
  worktrees to the system disk and ran `run-4` to `run-8` (amendment 2).
- Swapped the model roles in `gate.yaml` against the original plan, which had the implementer on
  SPARK. The L40S coder was available that night and SPARK was busy. The L40S has less memory, so
  weaker models, but they run faster; I also wanted to see whether a 30B model is enough for this task.
- Renamed the public repository to `factory-gate` so that no real company name appears in it.
- The most important decision was what to build. I chose the most ambitious option that also serves
  the largest number of the people in the brief.

## Time spent and why (Olin)

Measured from the session transcript and commit times (wall clock, not hands-on time):

| Block | Wall clock | Notes |
| --- | --- | --- |
| Stance, outline and plan (claude.ai chat) | 2026-11-19, 30 minutes | |
| Phases 0 to 4 (Claude Code session) | 2026-11-19 20:11 to 2026-11-20 09:30 | 13 messages from me; most of the time the agent worked or the live runs ran unattended |
| README, evals, presentation | 2026-11-24 from 05:50 | |

- My hands-on time: 30 minutes on the plan, the rest under 2 hours, up to `make demo`, which I have
  not run myself yet; I trusted it. The suggested budget was about 4 hours.
- What took longer than expected: live capture on the local models (two infrastructure
  failures: a build-check timeout and a slow repository volume during a backup), and setting up
  the endpoints and access windows on my own hardware.
- What I chose to invest in: running the implementer on my own local models rather than a hosted
  API, so the loop and the matrix cost time, not money; recording every run honestly.
- What I cut: the security check, the independent test author and the mutation check
  (Phases 5, 6, 8). I have not run an independent multi-agent review of the code either.
