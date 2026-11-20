# Sample evidence

source: replayed (attempts), seeded (shortcut). The gate ran live on every one.

Produced by `make demo` in a fresh clone on 2026-11-20, replaying `demo/recorded/run-4`
(`demo/DEFAULT`). Model output is replayed from that recording; the verdicts and evidence are
the gate's own, computed when the demo ran.

| Directory | Verdict | What it shows |
| --- | --- | --- |
| `attempt-1/` | FAIL | The implementer's partial fills; the invariants find a duplicate fill applied twice. `failure_report.md` is what went back to the model |
| `attempt-2/` | FAIL | The second try, after the failure report |
| `attempt-3/` | ESCALATE | Still failing on the last attempt: a human decides |
| `seeded-shortcut/` | ESCALATE | `demo/attempts/weakened-invariant.patch`, hand-written: the change weakens an invariant; the gate escalates and the original invariant from base still fails |
