# PROGRESS.md

## Status

| Phase | State | Commit | Notes |
| --- | --- | --- | --- |
| 0 Scaffold and rules | done | 773043b | |
| 1 Sample service and invariant harness | STOP H1, waiting for Olin | (this commit) | assertions are stubs |

## Decisions and deviations from the plan

- The plan lives in `Prompts/00-FACTORY_PLAN.md`, where Olin put it; `FACTORY_PLAN.md` at the root is a copy so the layout in 5.1 holds. Edit the root copy from now on.
- `.claude/settings.json` deny rules are added when a path becomes human-owned, not all at Phase 0: invariants at STOP H1, `gate.yaml` at STOP H2. Denying them earlier would block the delegated work that creates them (harness, draft config), or push it to shell writes.
- `cmd/gate`, `internal`, `prompts`, `evals` are protected against factory agents (by the gate), not against the session building the repo, which is delegated to build them.
- The YAML parser is `gopkg.in/yaml.v3`.
- `gitleaks` and `govulncheck` are not installed. Needed in Phase 5.
- Checks take `rapid.TB`, not `*rapid.T`. Reason: `t.Skip` inside `rapid.Check` discards the generated case as invalid, and when every case is invalid rapid fails the test. The harness therefore probes each check once on a one-event world with the outer `*testing.T` first, so an unwritten stub skips the whole test cleanly.
- The copy for the idempotency check is built by replaying the history through `Apply` (`World.Copy`), not by a `Clone` method. The engine API stays minimal and the copy is faithful whatever the engine stores internally.
- `Fill` carries a `FillID` already in the baseline, unused there: the full-fill rule makes a duplicate fill fail anyway. A correct partial-fills change needs it to drop duplicates.
- Rapid flags verified on v1.3.0: `-rapid.checks`, `-rapid.seed`, `-rapid.nofailfile`.
- Harness checked in a throwaway copy (not committed) with trial assertions: baseline passes 500 checks with seed 1; a naive partial-fills change fails idempotency with a two-step shrunk counterexample; an overfill bug fails the overfill check.

## Open questions for Olin

- STOP H1: write the three assertions in `sample/orders/invariants/invariants_test.go`, run `make invariants` against the baseline (must pass), review the generators in `harness_test.go`, and fill in `plan_approved_by` in `specs/partial-fills.md`.

## Next step (one sentence, concrete enough to resume cold)

After Olin's H1 commit with the assertions, start Phase 2: gate core in `internal/gate` and the draft `gate.yaml`.
