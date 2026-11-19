# factory-gate

Work in progress. See `FACTORY_PLAN.md` and `PROGRESS.md`. The full README is written in Phase 9.

## A note on the sample spec

`specs/partial-fills.md` is written as a product owner would write it. It does not mention
duplicate events or idempotency, on purpose: real tickets do not list every invariant, and
that gap is what the gate exists to cover. The invariants in `sample/orders/invariants/`
cover it instead.
