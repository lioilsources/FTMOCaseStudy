# PROGRESS.md

## Status

| Phase | State | Commit | Notes |
| --- | --- | --- | --- |
| 0 Scaffold and rules | done | (first commit) | |
| 1 Sample service and invariant harness | in progress | | |

## Decisions and deviations from the plan

- The plan lives in `Prompts/00-FACTORY_PLAN.md`, where Olin put it; `FACTORY_PLAN.md` at the root is a copy so the layout in 5.1 holds. Edit the root copy from now on.
- `.claude/settings.json` deny rules are added when a path becomes human-owned, not all at Phase 0: invariants at STOP H1, `gate.yaml` at STOP H2. Denying them earlier would block the delegated work that creates them (harness, draft config), or push it to shell writes.
- `cmd/gate`, `internal`, `prompts`, `evals` are protected against factory agents (by the gate), not against the session building the repo, which is delegated to build them.
- The YAML parser is `gopkg.in/yaml.v3`.
- `gitleaks` and `govulncheck` are not installed. Needed in Phase 5.

## Open questions for Olin

## Next step (one sentence, concrete enough to resume cold)

Build `sample/orders` and the invariant harness (Phase 1).
