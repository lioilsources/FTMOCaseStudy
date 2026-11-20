# Evidence r-0001

source: replayed

## Verdict: FAIL

Blocking checks failed: invariants.

## Checks

| Check | Status | Required | ms | Key |
| --- | --- | --- | --- | --- |
| build | pass | true | 18499 | build, vet, unit tests ok |
| invariants | fail | true | 656 | counterexample found, seed 1, 200 checks |
| budget | pass | true | 0 | 8/200 lines, 2/8 files |

## Counterexample and findings

### invariants

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
    invariants_test.go:47: not idempotent: delivering orders.Fill{FillID:f1 OrderID:o1 Quantity:1} twice changed the state:
          first:  {Account:{Balance:1 Reserved:1} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:1 Status:open}]}
          second: {Account:{Balance:0 Reserved:0} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:2 Status:filled}]}
--- FAIL: TestInvariants (0.12s)
```

## Read these first

- `sample/orders/engine.go:98` (4 changed lines, tier-2 path)
- `sample/orders/engine_test.go:23` (4 changed lines, tier-2 path)
- `sample/orders/engine.go:75` (2 changed lines, tier-2 path)

## Provenance

- base: `5a9c83d02cb76e3a911e4da8a5dc5cb07ab19afe` (rules)
- merge base: `5a9c83d02cb76e3a911e4da8a5dc5cb07ab19afe` (diff from here)
- head: `cd16026b5e5642298568e0aee5368363ba7b7879`
- requested by: lioilsources
- agent: implementer/completion, model `qwen3-coder`, endpoint l40s, prompt sha256 `c2f4f9032f8ec394376917cec801c39f31a815c6c80c1dcd70407309ec08e6ac`, tokens 3483 in / 2373 out, 17.9 s, replayed
- spec: `specs/partial-fills.md` sha256 `3a84ef8a4f1771c8acb48cec173a9d81a804fd69833aa32301962e2e0b501ae0`
- attempt: 1 of 3
- gate version: `5a9c83d02cb76e3a911e4da8a5dc5cb07ab19afe`
- mode: blocking
- source: replayed

## What this gate cannot tell you

- The gate is only as good as its invariants. With none written, it catches nothing that tests would not.
- The invariants here are illustrative. Real ones have to come from the people who know the domain.
- Passing the gate is not proof of correctness. It is evidence that named properties held under a stated number of generated cases with a recorded seed.
- Loop results vary from run to run. One run proves nothing by itself.
- The completion implementer rewrites whole files and cannot run tests itself.
- The independent test author may share blind spots with the implementer.
- Identities are asserted, not authenticated. In production, requester and approver come from the platform.
- The audit log is a local file standing in for the platform's record.
- With the command adapter the agent has a shell in the worktree and is not sandboxed.
