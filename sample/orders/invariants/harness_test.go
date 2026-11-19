// Package invariants holds the human-owned property tests for sample/orders.
//
// It uses only the exported API of the orders package. The gate runs these tests from the
// base revision with a fixed number of checks and a recorded seed, for example:
//
//	go test ./sample/orders/invariants/ -rapid.checks=200 -rapid.seed=1 -rapid.nofailfile
//
// This file is the harness: generators, the runner and helpers. The assertions live in
// invariants_test.go.
package invariants

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/lioilsources/factory-gate/sample/orders"
	"pgregory.net/rapid"
)

// Snapshot is the observable state of an engine. Orders are sorted by id.
type Snapshot struct {
	Account orders.Account
	Orders  []orders.Order
}

// SnapshotOf captures the observable state of e.
func SnapshotOf(e *orders.Engine) Snapshot {
	return Snapshot{Account: e.Account(), Orders: e.Orders()}
}

// DiffSnapshots returns "" when a and b are equal, otherwise both states side by side.
func DiffSnapshots(a, b Snapshot) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	return fmt.Sprintf("\n  first:  %+v\n  second: %+v", a, b)
}

// World is what a check sees after each step.
type World struct {
	InitialBalance int64
	History        []orders.Event // every event offered to the engine, accepted or not
	Last           orders.Event   // the event of this step, also the last element of History
	LastErr        error          // what Apply returned for Last
	Engine         *orders.Engine // the engine after Last
	Before         Snapshot       // state before Last was applied
}

// Now is the state after the last step.
func (w *World) Now() Snapshot { return SnapshotOf(w.Engine) }

// Copy returns a new engine with the same initial balance and History replayed into it.
// It only uses Apply, so it is a faithful copy whatever the engine stores internally.
func (w *World) Copy() *orders.Engine {
	e := orders.NewEngine(w.InitialBalance)
	for _, ev := range w.History {
		_ = e.Apply(ev) // rejections are part of the history and are replayed too
	}
	return e
}

// apply offers ev to the engine and records it.
func (w *World) apply(ev orders.Event) {
	w.Before = w.Now()
	w.LastErr = w.Engine.Apply(ev)
	w.Last = ev
	w.History = append(w.History, ev)
}

// check is the shape of every invariant: fail t when w violates it.
type check func(t rapid.TB, w *World)

// Pools are small on purpose, so that generated events collide: the same order id placed
// twice, the same fill id reported twice, fills for unknown ids.
var (
	orderIDs = []string{"o1", "o2", "o3", "o4"}
	fillIDs  = []string{"f1", "f2", "f3", "f4", "f5", "f6"}
	unknown  = "ghost"
)

// drawEvent picks the next event. It may look at the current state so that the sequence
// reaches interesting places (full fills, fills after cancel) rather than only rejections.
func drawEvent(t *rapid.T, w *World) orders.Event {
	switch kind := rapid.IntRange(0, 9).Draw(t, "kind"); {
	case kind <= 2:
		return orders.PlaceOrder{
			OrderID:  rapid.SampledFrom(orderIDs).Draw(t, "order"),
			Side:     rapid.SampledFrom([]orders.Side{orders.Buy, orders.Sell}).Draw(t, "side"),
			Quantity: rapid.Int64Range(1, 10).Draw(t, "quantity"),
			Price:    rapid.Int64Range(1, 50).Draw(t, "price"),
		}
	case kind <= 5:
		id := rapid.SampledFrom(append([]string{unknown}, orderIDs...)).Draw(t, "order")
		qty := rapid.Int64Range(1, 12).Draw(t, "quantity")
		if o, ok := w.Engine.Order(id); ok && o.Remaining() > 0 && rapid.Bool().Draw(t, "fill remaining") {
			qty = o.Remaining()
		}
		return orders.Fill{FillID: rapid.SampledFrom(fillIDs).Draw(t, "fill"), OrderID: id, Quantity: qty}
	case kind == 6:
		return orders.Cancel{OrderID: rapid.SampledFrom(append([]string{unknown}, orderIDs...)).Draw(t, "order")}
	case kind == 7 && len(w.History) > 0: // duplicate: the previous event again
		return w.History[len(w.History)-1]
	case kind == 8 && len(w.History) > 0: // replay: any earlier event again
		return w.History[rapid.IntRange(0, len(w.History)-1).Draw(t, "replay index")]
	default: // fill after cancel, when there is a cancelled order
		for _, o := range w.Engine.Orders() {
			if o.Status == orders.Cancelled {
				return orders.Fill{FillID: rapid.SampledFrom(fillIDs).Draw(t, "fill"), OrderID: o.ID, Quantity: o.Quantity}
			}
		}
		return orders.Cancel{OrderID: rapid.SampledFrom(orderIDs).Draw(t, "order")}
	}
}

// runChecks generates an event sequence and calls every check after every step.
// The failing output lists each step, so the shrunk counterexample reads as a story.
func runChecks(t *testing.T, checks ...check) {
	probe(t, checks)
	rapid.Check(t, func(rt *rapid.T) {
		w := &World{InitialBalance: rapid.Int64Range(0, 1000).Draw(rt, "initial balance")}
		w.Engine = orders.NewEngine(w.InitialBalance)
		steps := rapid.IntRange(1, 30).Draw(rt, "steps")
		for i := range steps {
			ev := drawEvent(rt, w)
			w.apply(ev)
			rt.Logf("step %d: %T%+v -> err=%v; account=%+v", i+1, ev, ev, w.LastErr, w.Engine.Account())
			for _, c := range checks {
				c(rt, w)
			}
		}
	})
}

// probe runs each check once on a one-event world with the outer test. A check that is not
// written yet calls t.Skip there, which skips the whole test instead of rapid discarding
// every generated case as invalid.
func probe(t *testing.T, checks []check) {
	w := &World{InitialBalance: 100, Engine: orders.NewEngine(100)}
	w.apply(orders.PlaceOrder{OrderID: "o1", Side: orders.Buy, Quantity: 1, Price: 1})
	for _, c := range checks {
		c(t, w)
	}
}

func TestInvariants(t *testing.T) {
	runChecks(t, checkNoOverfill, checkNoNegativeBalance, checkIdempotentEvents)
}
