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
| Implementer | not chosen yet (STOP H2) | |
| Test author | not chosen yet (STOP H2) | |

## Sessions (agent-maintained)

| Start | End | Phase | Model | Delegated task | Outcome |
| --- | --- | --- | --- | --- | --- |
| 2026-11-19 20:12 | | 0, 1 | Claude Opus 5.5 | Scaffold; sample service and invariant harness | STOP H1 reached |

## What went wrong (agent-maintained, factual: failed approaches, rework, wrong assumptions)

- Phase 1: the plan's stub shape (`t.Skip` in each check) does not work inside `rapid.Check`; rapid treats a skip as an invalid case and fails after too many. Fixed with a probe on the outer test (see PROGRESS).
- Phase 0: denying edits on all protected paths at once would have blocked the delegated work that creates them. Deny rules are added per path when it becomes human-owned.

## Plan provenance

Drafted with Claude in a chat session from my outline. [Olin: confirm or correct]

## What I reviewed (Olin)

## What I wrote by hand (Olin)

## Where I overrode the agent (Olin)

## Time spent and why (Olin)
