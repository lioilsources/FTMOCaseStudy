# Capture rule

source: recorded

Fixed on 2026-11-20, before any live run, and committed before the first one.

1. Five live loop runs on `specs/partial-fills.md`, in order, named `run-1` to `run-5`:
   `gate loop --spec specs/partial-fills.md --base main --context spec-only --record demo/recorded/run-N`
2. The prompt, the spec and `gate.yaml` do not change between the runs. They are not
   adjusted to provoke a failure.
3. Every run is listed in `demo/RUNS.md`, which is generated from the recordings and never
   edited by hand: run id, model ids, context, outcome per attempt, tokens, seconds.
4. The default recording for `make demo` is the first run whose outcomes are FAIL then PASS.
   If no run has that shape, the default is `run-1` as it is, and Olin is told. A run that
   ends in ESCALATE, or five first-attempt passes, is a result to show, not to hide.
5. Runs that fail for infrastructure reasons (endpoint down, timeout before any reply) are
   kept and listed too; they are not replaced.

## Amendment, 2026-11-20, after run-1 and before run-2 (decided by Olin)

`run-1` ended in ESCALATE because the build check timed out (300 s): it ran the gate's own
end-to-end tests, which build fixture repositories. That is an infrastructure failure, so by
rule 5 `run-1` is kept and listed, with its gate version. The build check now runs unit
tests with `-short`, which skips only those end-to-end tests (protected, always from base).
Five more runs, `run-2` to `run-6`, are made with the fixed gate and the same prompt, spec
and `gate.yaml`. Rule 4 picks the default from all runs in order. `run-2` was started once
before the fix and stopped before the model replied; nothing was recorded for it.

## Amendment 2, 2026-11-20, after run-2 and an interrupted run-3 (decided by Olin)

The repository volume became very slow (a 50 MB write took 62 s while a backup ran).
`run-2` ended in ESCALATE because the invariants timed out after a 540 s build; `run-3` was
stopped after one recorded call. Both are kept and listed (rule 5). The gate now checks out
worktrees on the system temp disk and uses its own build cache there. Five runs, `run-4` to
`run-8`, are made with that gate and the same prompt, spec and `gate.yaml`. Rule 4 still
picks the default from all runs in order.

## Amendment 3, 2026-11-20, after all runs (decided by Olin)

No run has outcomes FAIL, PASS, so rule 4 names `run-1`. `run-1` holds one recorded call
made with the gate version that timed out; with the current gate its attempt 1 ends in FAIL,
attempt 2 asks for a call that was never recorded, and the replay stops as stale. Olin chose
`run-4` instead: the first run made with the final gate that replays to the end. Its replay
was checked once: FAIL, FAIL, ESCALATE, the same as recorded. The choice is in
`demo/DEFAULT`; `demo/RUNS.md` still lists every run.

After the runs, the loop was changed to resend the last gate report after an agent error
(runs 5 to 8 lost it on their third attempt). Recordings with an agent error therefore no
longer replay byte for byte; `run-4` has none.
