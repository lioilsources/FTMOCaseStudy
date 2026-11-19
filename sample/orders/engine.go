package orders

import (
	"fmt"
	"sort"
)

// Engine applies events to one account and its orders.
type Engine struct {
	account Account
	orders  map[string]*Order
}

// NewEngine returns an engine whose account starts with the given balance.
func NewEngine(balance int64) *Engine {
	return &Engine{account: Account{Balance: balance}, orders: map[string]*Order{}}
}

// Account returns a copy of the account.
func (e *Engine) Account() Account { return e.account }

// Order returns a copy of the order with the given id.
func (e *Engine) Order(id string) (Order, bool) {
	o, ok := e.orders[id]
	if !ok {
		return Order{}, false
	}
	return *o, true
}

// Orders returns copies of all orders, sorted by id.
func (e *Engine) Orders() []Order {
	out := make([]Order, 0, len(e.orders))
	for _, o := range e.orders {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Apply applies one event. On error the engine is unchanged.
func (e *Engine) Apply(ev Event) error {
	switch ev := ev.(type) {
	case PlaceOrder:
		return e.place(ev)
	case Fill:
		return e.fill(ev)
	case Cancel:
		return e.cancel(ev)
	}
	return fmt.Errorf("%w: %T", ErrInvalidEvent, ev)
}

func (e *Engine) place(ev PlaceOrder) error {
	if ev.OrderID == "" || (ev.Side != Buy && ev.Side != Sell) ||
		ev.Quantity <= 0 || ev.Quantity > MaxQuantity || ev.Price <= 0 || ev.Price > MaxPrice {
		return fmt.Errorf("%w: %+v", ErrInvalidEvent, ev)
	}
	if _, ok := e.orders[ev.OrderID]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateOrder, ev.OrderID)
	}
	if ev.Side == Buy {
		cost := ev.Quantity * ev.Price
		if e.account.Balance-e.account.Reserved < cost {
			return fmt.Errorf("%w: need %d", ErrInsufficientFunds, cost)
		}
		e.account.Reserved += cost
	}
	e.orders[ev.OrderID] = &Order{
		ID: ev.OrderID, Side: ev.Side, Quantity: ev.Quantity, Price: ev.Price, Status: Open,
	}
	return nil
}

// fill accepts only a fill for the full remaining quantity.
func (e *Engine) fill(ev Fill) error {
	if ev.FillID == "" || ev.Quantity <= 0 {
		return fmt.Errorf("%w: %+v", ErrInvalidEvent, ev)
	}
	o, ok := e.orders[ev.OrderID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownOrder, ev.OrderID)
	}
	if o.Status != Open {
		return fmt.Errorf("%w: %s is %s", ErrOrderNotOpen, o.ID, o.Status)
	}
	if ev.Quantity != o.Remaining() {
		return fmt.Errorf("%w: got %d, remaining %d", ErrFillQuantity, ev.Quantity, o.Remaining())
	}
	value := ev.Quantity * o.Price
	if o.Side == Buy {
		e.account.Reserved -= value
		e.account.Balance -= value
	} else {
		e.account.Balance += value
	}
	o.FilledQuantity += ev.Quantity
	o.Status = Filled
	return nil
}

func (e *Engine) cancel(ev Cancel) error {
	o, ok := e.orders[ev.OrderID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownOrder, ev.OrderID)
	}
	if o.Status != Open {
		return fmt.Errorf("%w: %s is %s", ErrOrderNotOpen, o.ID, o.Status)
	}
	if o.Side == Buy {
		e.account.Reserved -= o.Remaining() * o.Price
	}
	o.Status = Cancelled
	return nil
}
