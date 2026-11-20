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
    invariants_test.go:47: not idempotent: delivering orders.Fill{FillID:f1 OrderID:o1 Quantity:1} twice changed the state:
          first:  {Account:{Balance:1 Reserved:1} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:1 Status:open}]}
          second: {Account:{Balance:0 Reserved:0} Orders:[{ID:o1 Side:buy Quantity:2 Price:1 FilledQuantity:2 Status:filled}]}
--- FAIL: TestInvariants 
```

