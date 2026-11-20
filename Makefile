GO ?= go

.PHONY: build test invariants gate doctor demo demo-live eval

build:
	$(GO) build ./...
	$(GO) vet ./...

test:
	$(GO) test ./...

# Property tests with the check count and seed the gate uses (gate.yaml).
invariants:
	$(GO) test ./sample/orders/invariants/ -rapid.checks=200 -rapid.seed=1 -rapid.nofailfile

gate:
	$(GO) build -o .factory/bin/gate ./cmd/gate

doctor: gate
	.factory/bin/gate doctor

# Offline: replays the default recording (demo/DEFAULT); the gate runs live.
# DEMO_NOPAUSE=1 skips the pauses.
demo:
	sh demo/demo.sh

# Live: needs the implementer endpoint in .env. Recorded into demo/recorded/live-<time>.
demo-live: gate
	@.factory/bin/gate doctor --role implementer || { echo "endpoint not ready: run make demo (replayed) instead"; exit 1; }
	.factory/bin/gate loop --spec specs/partial-fills.md --base main --stream --record demo/recorded/live-$$(date +%Y%m%d-%H%M%S)

eval:
	@echo "eval: not built yet (Phase 7)"; exit 1
