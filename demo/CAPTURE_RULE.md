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
