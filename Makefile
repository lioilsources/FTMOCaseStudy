GO ?= go

.PHONY: build test invariants gate doctor demo demo-live eval

build:
	$(GO) build ./...
	$(GO) vet ./...

test:
	$(GO) test ./...

# Property tests with a fixed check count and seed (values move to gate.yaml in Phase 2).
invariants:
	$(GO) test ./sample/orders/invariants/ -rapid.checks=200 -rapid.seed=1 -rapid.nofailfile

gate:
	$(GO) build -o .factory/bin/gate ./cmd/gate

doctor: gate
	.factory/bin/gate doctor

demo:
	@echo "doctor: gate
	.factory/bin/gate doctor

demo: not built yet (Phase 4)"; exit 1

demo-live:
	@echo "demo-live: not built yet (Phase 4)"; exit 1

eval:
	@echo "eval: not built yet (Phase 7)"; exit 1
