package orders

import (
	"errors"
	"testing"
)

func TestApply(t *testing.T) {
	buy := PlaceOrder{OrderID: "o1", Side: Buy, Quantity: 10, Price: 5}
	sell := PlaceOrder{OrderID: "o2", Side: Sell, Quantity: 4, Price: 7}

	tests := []struct {
		name        string
		events      []Event
		wantErr     error // error of the last event
		wantAccount Account
		wantStatus  map[string]Status
	}{
		{"place buy reserves", []Event{buy}, nil, Account{100, 50}, map[string]Status{"o1": Open}},
		{"place sell reserves nothing", []Event{sell}, nil, Account{100, 0}, map[string]Status{"o2": Open}},
		{"full buy fill", []Event{buy, Fill{"f1", "o1", 10}}, nil, Account{50, 0}, map[string]Status{"o1": Filled}},
		{"full sell fill", []Event{sell, Fill{"f1", "o2", 4}}, nil, Account{128, 0}, map[string]Status{"o2": Filled}},
		{"partial fill rejected", []Event{buy, Fill{"f1", "o1", 3}}, ErrFillQuantity, Account{100, 50}, map[string]Status{"o1": Open}},
		{"overfill rejected", []Event{buy, Fill{"f1", "o1", 11}}, ErrFillQuantity, Account{100, 50}, map[string]Status{"o1": Open}},
		{"cancel releases", []Event{buy, Cancel{"o1"}}, nil, Account{100, 0}, map[string]Status{"o1": Cancelled}},
		{"fill after cancel", []Event{buy, Cancel{"o1"}, Fill{"f1", "o1", 10}}, ErrOrderNotOpen, Account{100, 0}, map[string]Status{"o1": Cancelled}},
		{"second fill", []Event{buy, Fill{"f1", "o1", 10}, Fill{"f1", "o1", 10}}, ErrOrderNotOpen, Account{50, 0}, map[string]Status{"o1": Filled}},
		{"cancel after fill", []Event{buy, Fill{"f1", "o1", 10}, Cancel{"o1"}}, ErrOrderNotOpen, Account{50, 0}, map[string]Status{"o1": Filled}},
		{"duplicate place", []Event{buy, buy}, ErrDuplicateOrder, Account{100, 50}, map[string]Status{"o1": Open}},
		{"insufficient funds", []Event{PlaceOrder{"o1", Buy, 30, 5}}, ErrInsufficientFunds, Account{100, 0}, map[string]Status{}},
		{"unknown fill", []Event{Fill{"f1", "nope", 1}}, ErrUnknownOrder, Account{100, 0}, map[string]Status{}},
		{"unknown cancel", []Event{Cancel{"nope"}}, ErrUnknownOrder, Account{100, 0}, map[string]Status{}},
		{"zero quantity", []Event{PlaceOrder{"o1", Buy, 0, 5}}, ErrInvalidEvent, Account{100, 0}, map[string]Status{}},
		{"bad side", []Event{PlaceOrder{"o1", Side(9), 1, 5}}, ErrInvalidEvent, Account{100, 0}, map[string]Status{}},
		{"price above limit", []Event{PlaceOrder{"o1", Sell, 1, MaxPrice + 1}}, ErrInvalidEvent, Account{100, 0}, map[string]Status{}},
		{"fill without id", []Event{buy, Fill{"", "o1", 10}}, ErrInvalidEvent, Account{100, 50}, map[string]Status{"o1": Open}},
		{"nil event", []Event{nil}, ErrInvalidEvent, Account{100, 0}, map[string]Status{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine(100)
			var err error
			for _, ev := range tt.events {
				err = e.Apply(ev)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("last error = %v, want %v", err, tt.wantErr)
			}
			if got := e.Account(); got != tt.wantAccount {
				t.Errorf("account = %+v, want %+v", got, tt.wantAccount)
			}
			if got := len(e.Orders()); got != len(tt.wantStatus) {
				t.Errorf("orders = %d, want %d", got, len(tt.wantStatus))
			}
			for id, want := range tt.wantStatus {
				o, ok := e.Order(id)
				if !ok || o.Status != want {
					t.Errorf("order %s = %+v (found %v), want status %s", id, o, ok, want)
				}
			}
		})
	}
}

func TestOrdersSortedAndCopied(t *testing.T) {
	e := NewEngine(1000)
	for _, id := range []string{"c", "a", "b"} {
		if err := e.Apply(PlaceOrder{id, Sell, 1, 1}); err != nil {
			t.Fatal(err)
		}
	}
	got := e.Orders()
	if got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "c" {
		t.Fatalf("orders not sorted: %+v", got)
	}
	got[0].FilledQuantity = 99
	if o, _ := e.Order("a"); o.FilledQuantity != 0 {
		t.Fatal("Orders returned a reference, not a copy")
	}
}
