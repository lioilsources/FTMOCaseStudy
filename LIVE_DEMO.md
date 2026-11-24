# Demo transcript

source: replayed (model replies from `demo/recorded/run-4`), seeded (step 6). The gate ran live
when this transcript was captured: `DEMO_NOPAUSE=1 make demo` on commit `51edd4b`, 2026-11-24 07:20.

A text backup of `make demo` in case the demo cannot run during the session. Generated from the
command output; ANSI colours removed, the local repository path shortened. Run ids and commit
hashes differ on every run.

## 1. The task, as a product owner wrote it (no word about duplicate events)

```text
$ sed -n 1,30p specs/partial-fills.md
---
id: partial-fills
tier: 2
allowed_paths:
  - sample/orders/*.go
plan_approved_by: "ol1n"
---

# Partial fills

## Intent

Today an order can only be filled in one go: a fill must equal the whole remaining
quantity, and anything else is rejected. Venues report executions in pieces, so large
orders sit rejected until someone fills them by hand. Orders should accept partial fills.

## Acceptance criteria

1. A fill for less than the remaining quantity is accepted. The order's filled quantity
   grows by the fill quantity and the order stays open.
2. A fill that brings the filled quantity to the order quantity marks the order filled.
3. A fill for more than the remaining quantity is rejected and changes nothing.
4. For a buy order, each fill moves its value (fill quantity times order price) out of the
   reservation and out of the balance. For a sell order, each fill adds its value to the balance.
5. Cancelling a partly filled buy order releases only what is still reserved for the
   unfilled part.
6. Fills for unknown, filled or cancelled orders are rejected as today.
7. Existing behaviour for full fills does not change, and existing unit tests keep passing.

## Out of scope
```

## 2. Build the gate

```text
$ go build -o .factory/demo/gate ./cmd/gate
recording: demo/recorded/run-4 (source: recorded live on the implementer model; replayed now)
```

## 3. Replay the recorded loop. The model's replies are replayed; the gate runs live

```text
$ .factory/demo/gate loop --spec specs/partial-fills.md --base main --replay demo/recorded/run-4
loop l-0015: spec specs/partial-fills.md, context spec-only, up to 3 attempts, replayed
attempt 1: asking qwen3-coder on l40s
 sample/orders/engine.go      | 8 +++++---
 sample/orders/engine_test.go | 4 +++-
 2 files changed, 8 insertions(+), 4 deletions(-)
attempt 1: FAIL blocking checks failed: invariants
attempt 2: asking qwen3-coder on l40s
 sample/orders/engine_test.go | 36 ++++++++++++++++++++++++++++++++++++
 1 file changed, 36 insertions(+)
attempt 2: FAIL blocking checks failed: build, invariants
attempt 3: asking qwen3-coder on l40s
 sample/orders/engine.go | 6 +++++-
 1 file changed, 5 insertions(+), 1 deletion(-)
attempt 3: ESCALATE blocking checks failed (build, invariants) on attempt 3 of 3
loop l-0015 finished: ESCALATE
  attempt 1  FAIL        r-0078   model qwen3-coder, 3483+2373 tokens, 17.9 s
  attempt 2  FAIL        r-0079   model qwen3-coder, 4206+2683 tokens, 21.2 s
  attempt 3  ESCALATE    r-0080   model qwen3-coder, 4617+919 tokens, 7.0 s
```

## 4.1 Attempt 1: what the model changed

```text
$ git --no-pager show --stat --format=%h %s%n%n%(trailers) loop/l-0015/attempt-1
a773479 l-0015: attempt 1 for specs/partial-fills.md

Agent: implementer/completion
Model: qwen3-coder
Endpoint: l40s
Prompt-SHA256: c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac
Run-ID: l-0015/attempt-1
Requested-By: lioilsources


 sample/orders/engine.go      | 8 +++++---
 sample/orders/engine_test.go | 4 +++-
 2 files changed, 8 insertions(+), 4 deletions(-)
```

## 4.1 Attempt 1: the failure report sent back to the model (head)

```text
$ sed -n 1,25p .factory/runs/r-0078/failure_report.md
# Failure report (attempt 1)

## invariants: counterexample found, seed 1, 200 checks

```
=== RUN   TestInvariants
    harness_test.go:120: [rapid] failed after 17 tests: not idempotent: delivering orders.Fill{FillID:f1 OrderID:o1 Quantity:1} twice changed the state:
          first:  {Account:{Balance:1 Reserved:1} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:1 Status:open}]}
          second: {Account:{Balance:0 Reserved:0} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:2 Status:filled}]}
        To reproduce, specify -run="TestInvariants" -rapid.seed=154
        Failed test output:
    harness_test.go:121: [rapid] draw initial balance: 2
    harness_test.go:123: [rapid] draw steps: 2
    harness_test.go:85: [rapid] draw kind: 0
    harness_test.go:88: [rapid] draw order: "o1"
    harness_test.go:89: [rapid] draw side: 1
    harness_test.go:90: [rapid] draw quantity: 2
    harness_test.go:91: [rapid] draw price: 1
    harness_test.go:127: step 1: orders.PlaceOrder{OrderID:o1 Side:buy Quantity:2 Price:1} -> err=<nil>; account={Balance:2 Reserved:2}
    harness_test.go:85: [rapid] draw kind: 3
    harness_test.go:94: [rapid] draw order: "o1"
    harness_test.go:95: [rapid] draw quantity: 1
    harness_test.go:96: [rapid] draw fill remaining: false
    harness_test.go:99: [rapid] draw fill: "f1"
    harness_test.go:127: step 2: orders.Fill{FillID:f1 OrderID:o1 Quantity:1} -> err=<nil>; account={Balance:1 Reserved:1}
```

## 4.2 Attempt 2: what the model changed

```text
$ git --no-pager show --stat --format=%h %s%n%n%(trailers) loop/l-0015/attempt-2
abca58c l-0015: attempt 2 for specs/partial-fills.md

Agent: implementer/completion
Model: qwen3-coder
Endpoint: l40s
Prompt-SHA256: c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac
Run-ID: l-0015/attempt-2
Requested-By: lioilsources


 sample/orders/engine_test.go | 36 ++++++++++++++++++++++++++++++++++++
 1 file changed, 36 insertions(+)
```

## 4.2 Attempt 2: the failure report sent back to the model (head)

```text
$ sed -n 1,25p .factory/runs/r-0079/failure_report.md
# Failure report (attempt 2)

## build: 2 failing

```
--- FAIL: TestPartialFills 
    engine_test.go:106: Account changed on duplicate fill: before {1 1}, after {0 0}
    engine_test.go:115: Order changed on duplicate fill: before {o1 buy 2 1 1 open}, after {o1 buy 2 1 2 filled}
FAIL
FAIL	github.com/lioilsources/factory-gate/sample/orders
FAIL
```

## invariants: counterexample found, seed 1, 200 checks

```
=== RUN   TestInvariants
    harness_test.go:120: [rapid] failed after 17 tests: not idempotent: delivering orders.Fill{FillID:f1 OrderID:o1 Quantity:1} twice changed the state:
          first:  {Account:{Balance:1 Reserved:1} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:1 Status:open}]}
          second: {Account:{Balance:0 Reserved:0} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:2 Status:filled}]}
        To reproduce, specify -run="TestInvariants" -rapid.seed=154
        Failed test output:
    harness_test.go:121: [rapid] draw initial balance: 2
    harness_test.go:123: [rapid] draw steps: 2
    harness_test.go:85: [rapid] draw kind: 0
```

## 4.3 Attempt 3: what the model changed

```text
$ git --no-pager show --stat --format=%h %s%n%n%(trailers) loop/l-0015/attempt-3
d56899f l-0015: attempt 3 for specs/partial-fills.md

Agent: implementer/completion
Model: qwen3-coder
Endpoint: l40s
Prompt-SHA256: c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac
Run-ID: l-0015/attempt-3
Requested-By: lioilsources


 sample/orders/engine.go | 6 +++++-
 1 file changed, 5 insertions(+), 1 deletion(-)
```

## 5. Evidence of the final attempt: what a reviewer reads instead of the diff

```text
$ sed -n 1,/^## Counterexample/p .factory/runs/r-0080/evidence.md
# Evidence r-0080

source: replayed

## Verdict: ESCALATE

Blocking checks failed (build, invariants) on attempt 3 of 3.

## Checks

| Check | Status | Required | ms | Key |
| --- | --- | --- | --- | --- |
| build | fail | true | 2390 | 2 failing |
| invariants | fail | true | 601 | counterexample found, seed 1, 200 checks |
| budget | pass | true | 0 | 12/200 lines, 2/8 files |

## Counterexample and findings
$ sed -n /^## Read these first/,/^## What this gate/p .factory/runs/r-0080/evidence.md
## Read these first

- `sample/orders/engine_test.go:83` (36 changed lines, tier-2 path)
- `sample/orders/engine.go:90` (4 changed lines, tier-2 path)
- `sample/orders/engine.go:102` (4 changed lines, tier-2 path)

## Provenance

- base: `51edd4b5b98d15e3bb5f4861fb49112ab8df0f42` (rules)
- merge base: `51edd4b5b98d15e3bb5f4861fb49112ab8df0f42` (diff from here)
- head: `d56899f133911ed5c777d81144ff46c827095f3c`
- requested by: lioilsources
- agent: implementer/completion, model `qwen3-coder`, endpoint l40s, prompt sha256 `c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac`, tokens 4617 in / 919 out, 7.0 s, replayed
- spec: `specs/partial-fills.md` sha256 `3a84ef8a4f1771c8acb48cec173a9d81a804fd69833aa32301962e2e0b501ae0`
- attempt: 3 of 3
- gate version: `51edd4b5b98d15e3bb5f4861fb49112ab8df0f42`
- mode: blocking
- source: replayed

## What this gate cannot tell you
```

## 6. The shortcut (source: seeded): fake the fix by weakening the invariant

```text
$ git worktree add -q -b demo/seeded-20261124071959 .factory/demo/seeded-20261124071959 main
$ .factory/demo/gate run --base main --head demo/seeded-20261124071959 --spec specs/partial-fills.md --source seeded
r-0081 ESCALATE: the change touches protected paths: sample/orders/invariants/invariants_test.go
  build      fail         3 failing
  invariants fail         counterexample found, seed 1, 200 checks
  budget     fail         6/200 lines, 2/8 files, 1 out of scope
evidence: .factory/runs/r-0081/evidence.md
```

## 7. The audit trail

```text
$ tail -n 4 .factory/audit.jsonl
{"run_id":"r-0078","ts":"2026-11-24T06:20:14Z","kind":"gate_run","requested_by":"lioilsources","agents":[{"role":"implementer","kind":"completion","model":"qwen3-coder","endpoint":"l40s","prompt_sha256":"c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac","tokens_in":3483,"tokens_out":2373,"seconds":17.870627542,"source":"replayed"}],"spec":{"path":"specs/partial-fills.md","sha256":"3a84ef8a4f1771c8acb48cec173a9d81a804fd69833aa32301962e2e0b501ae0"},"base":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42","head":"a773479ff522a0b7b91d343596af25232ceb939f","attempt":1,"files":["sample/orders/engine.go","sample/orders/engine_test.go"],"checks":[{"id":"build","status":"pass","ms":3066},{"id":"invariants","status":"fail","ms":689},{"id":"budget","status":"pass","ms":0}],"verdict":"FAIL","mode":"blocking","source":"replayed","gate_version":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42"}
{"run_id":"r-0079","ts":"2026-11-24T06:20:18Z","kind":"gate_run","requested_by":"lioilsources","agents":[{"role":"implementer","kind":"completion","model":"qwen3-coder","endpoint":"l40s","prompt_sha256":"c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac","tokens_in":4206,"tokens_out":2683,"seconds":21.244999458,"source":"replayed"}],"spec":{"path":"specs/partial-fills.md","sha256":"3a84ef8a4f1771c8acb48cec173a9d81a804fd69833aa32301962e2e0b501ae0"},"base":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42","head":"abca58c74de9af9c92e7afb36c7f50da90ade94f","attempt":2,"files":["sample/orders/engine.go","sample/orders/engine_test.go"],"checks":[{"id":"build","status":"fail","ms":2364},{"id":"invariants","status":"fail","ms":613},{"id":"budget","status":"pass","ms":0}],"verdict":"FAIL","mode":"blocking","source":"replayed","gate_version":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42"}
{"run_id":"r-0080","ts":"2026-11-24T06:20:22Z","kind":"gate_run","requested_by":"lioilsources","agents":[{"role":"implementer","kind":"completion","model":"qwen3-coder","endpoint":"l40s","prompt_sha256":"c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac","tokens_in":4617,"tokens_out":919,"seconds":7.014739916,"source":"replayed"}],"spec":{"path":"specs/partial-fills.md","sha256":"3a84ef8a4f1771c8acb48cec173a9d81a804fd69833aa32301962e2e0b501ae0"},"base":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42","head":"d56899f133911ed5c777d81144ff46c827095f3c","attempt":3,"files":["sample/orders/engine.go","sample/orders/engine_test.go"],"checks":[{"id":"build","status":"fail","ms":2390},{"id":"invariants","status":"fail","ms":601},{"id":"budget","status":"pass","ms":0}],"verdict":"ESCALATE","mode":"blocking","source":"replayed","gate_version":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42"}
{"run_id":"r-0081","ts":"2026-11-24T06:20:27Z","kind":"gate_run","requested_by":"lioilsources","spec":{"path":"specs/partial-fills.md","sha256":"3a84ef8a4f1771c8acb48cec173a9d81a804fd69833aa32301962e2e0b501ae0"},"base":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42","head":"a084d6f1b9ba45ae53c2e9beba705747775c9a05","attempt":1,"files":["sample/orders/engine.go","sample/orders/invariants/invariants_test.go"],"checks":[{"id":"build","status":"fail","ms":2325},{"id":"invariants","status":"fail","ms":625},{"id":"budget","status":"fail","ms":0}],"verdict":"ESCALATE","mode":"blocking","source":"seeded","gate_version":"51edd4b5b98d15e3bb5f4861fb49112ab8df0f42"}

Done. Every run is listed in demo/RUNS.md; the capture rule is demo/CAPTURE_RULE.md.
```
