package invariants

import "pgregory.net/rapid"

// The three invariants of the orders service. Owner: Olin. Each body fits in about ten lines.
// Use w.Now() for the state after the step, w.Before for the state before it, w.Last for
// the event, w.Copy() for a fresh engine with the same history, DiffSnapshots to compare.
// Fail with t.Fatalf and say which order or field broke the rule.

// checkNoOverfill: no order is ever filled beyond its quantity.
// For every order in w.Now(): 0 <= FilledQuantity <= Quantity.
func checkNoOverfill(t rapid.TB, w *World) {
	t.Skip("owner: human")
}

// checkNoNegativeBalance: the account never goes below zero, and never reserves more than
// it holds. In w.Now().Account: Balance >= 0, Reserved >= 0, Reserved <= Balance.
func checkNoNegativeBalance(t rapid.TB, w *World) {
	t.Skip("owner: human")
}

// checkIdempotentEvents: an event delivered twice has the effect of delivering it once.
// Take e := w.Copy() (already contains w.Last), apply w.Last to e again, and require
// DiffSnapshots(w.Now(), SnapshotOf(e)) == "".
func checkIdempotentEvents(t rapid.TB, w *World) {
	t.Skip("owner: human")
}
