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

Drafted with Claude in a chat session from my outline. [Olin: confirm or correct]

## What I reviewed (Olin)

## What I wrote by hand (Olin)

## Where I overrode the agent (Olin)

## Time spent and why (Olin)
