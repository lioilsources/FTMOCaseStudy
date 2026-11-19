---
id: partial-fills
tier: 2
allowed_paths:
  - sample/orders/*.go
plan_approved_by: ""
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

- Fill prices that differ from the order price.
- Positions, fees, multiple accounts.
- Any change to the exported API beyond what the criteria need.
