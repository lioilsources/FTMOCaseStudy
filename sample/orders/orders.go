// Package orders is a small synthetic order-handling service used as the system under test.
//
// One engine holds one cash account and its orders. Money and quantities are int64 minor
// units. Events are applied one at a time with Engine.Apply; a rejected event returns an
// error and leaves the engine unchanged.
package orders

import (
	"errors"
	"fmt"
)

// Limits keep quantity*price far from int64 overflow.
const (
	MaxQuantity int64 = 1_000_000
	MaxPrice    int64 = 1_000_000_000
)

// Side is the direction of an order.
type Side int

const (
	Buy Side = iota + 1
	Sell
)

func (s Side) String() string {
	switch s {
	case Buy:
		return "buy"
	case Sell:
		return "sell"
	}
	return fmt.Sprintf("side(%d)", int(s))
}

// Status is the lifecycle state of an order.
type Status int

const (
	Open Status = iota + 1
	Filled
	Cancelled
)

func (s Status) String() string {
	switch s {
	case Open:
		return "open"
	case Filled:
		return "filled"
	case Cancelled:
		return "cancelled"
	}
	return fmt.Sprintf("status(%d)", int(s))
}

// Account is the cash account behind all orders of an engine.
// Reserved is the part of Balance held for open buy orders.
type Account struct {
	Balance  int64
	Reserved int64
}

// Order is one order as the engine sees it.
type Order struct {
	ID             string
	Side           Side
	Quantity       int64
	Price          int64
	FilledQuantity int64
	Status         Status
}

// Remaining is the quantity still to be filled.
func (o Order) Remaining() int64 { return o.Quantity - o.FilledQuantity }

// Event is something that happens to the engine: PlaceOrder, Fill or Cancel.
type Event interface{ isEvent() }

// PlaceOrder opens a new order. A buy order reserves Quantity*Price from the account.
type PlaceOrder struct {
	OrderID  string
	Side     Side
	Quantity int64
	Price    int64
}

// Fill reports an execution against an open order at the order's price.
// FillID identifies the execution report and is unique per execution.
type Fill struct {
	FillID   string
	OrderID  string
	Quantity int64
}

// Cancel closes an open order and releases what it still reserves.
type Cancel struct {
	OrderID string
}

func (PlaceOrder) isEvent() {}
func (Fill) isEvent()       {}
func (Cancel) isEvent()     {}

// Errors returned by Apply. A rejected event never changes the engine.
var (
	ErrInvalidEvent      = errors.New("invalid event")
	ErrDuplicateOrder    = errors.New("duplicate order id")
	ErrUnknownOrder      = errors.New("unknown order id")
	ErrOrderNotOpen      = errors.New("order is not open")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrFillQuantity      = errors.New("fill quantity must equal the remaining quantity")
)
