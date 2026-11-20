package invariants

import "pgregory.net/rapid"

// The three invariants of the orders service. Owner: Olin. Each body fits in about ten lines.
// Use w.Now() for the state after the step, w.Before for the state before it, w.Last for
// the event, w.Copy() for a fresh engine with the same history, DiffSnapshots to compare.
// Fail with t.Fatalf and say which order or field broke the rule.

// checkNoOverfill: no order is ever filled beyond its quantity.
// For every order in w.Now(): 0 <= FilledQuantity <= Quantity.

// 1. Objednávka nikdy není přeplněná.
func checkNoOverfill(t rapid.TB, w *World) {
	for _, o := range w.Now().Orders {
		if o.FilledQuantity < 0 || o.FilledQuantity > o.Quantity {
			t.Fatalf("overfill: order %s has filled %d of %d", o.ID, o.FilledQuantity, o.Quantity)
		}
	}
}

// checkNoNegativeBalance: the account never goes below zero, and never reserves more than
// it holds. In w.Now().Account: Balance >= 0, Reserved >= 0, Reserved <= Balance.

// 2. Účet nejde do mínusu a nerezervuje víc, než má.
func checkNoNegativeBalance(t rapid.TB, w *World) {
	a := w.Now().Account
	switch {
	case a.Balance < 0:
		t.Fatalf("negative balance: %+v", a)
	case a.Reserved < 0:
		t.Fatalf("negative reservation: %+v", a)
	case a.Reserved > a.Balance:
		t.Fatalf("reserved more than the balance: %+v", a)
	}
}

// checkIdempotentEvents: an event delivered twice has the effect of delivering it once.
// Take e := w.Copy() (already contains w.Last), apply w.Last to e again, and require
// DiffSnapshots(w.Now(), SnapshotOf(e)) == "".

// 3. Událost doručená dvakrát má stejný efekt jako jednou.
func checkIdempotentEvents(t rapid.TB, w *World) {
	e := w.Copy()       // nový engine s celou historií, včetně poslední události
	_ = e.Apply(w.Last) // doruč poslední událost ještě jednou; odmítnutí je v pořádku
	if d := DiffSnapshots(w.Now(), SnapshotOf(e)); d != "" {
		t.Fatalf("not idempotent: delivering %T%+v twice changed the state:%s", w.Last, w.Last, d)
	}
}
