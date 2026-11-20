package gate

import (
	"strings"
	"testing"
)

func TestFailBlock(t *testing.T) {
	out := "$ go test -v\n=== RUN   TestInvariants\n    harness_test.go:120: [rapid] failed after 7 tests: overfill\n" +
		"    invariants_test.go:17: overfill: order o1 has filled 2 of 1\n--- FAIL: TestInvariants (0.11s)\nFAIL\nFAIL\tpkg\t0.3s\nFAIL\n"
	got := failBlock(out)
	if !strings.HasPrefix(got, "=== RUN") || !strings.HasSuffix(got, "--- FAIL: TestInvariants (0.11s)") {
		t.Fatalf("block = %q", got)
	}
	if !strings.Contains(got, "filled 2 of 1") {
		t.Fatalf("counterexample missing: %q", got)
	}
}

func TestFailingOutput(t *testing.T) {
	out := "ok  \tpkg/a\t0.1s\n?   \tpkg/b\t[no test files]\n--- FAIL: TestX (0.00s)\nFAIL\tpkg/c\t0.2s\n"
	if got := failingOutput(out); strings.Contains(got, "pkg/a") || strings.Contains(got, "pkg/b") || !strings.Contains(got, "TestX") {
		t.Fatalf("output = %q", got)
	}
}

func TestNormalize(t *testing.T) {
	in := "--- FAIL: TestInvariants (0.09s)\nFAIL\tpkg/x\t0.349s\n/var/folders/x/T/factory-gate-123456/r-0007/sample/orders/engine.go:3: bad"
	want := "--- FAIL: TestInvariants \nFAIL\tpkg/x\nsample/orders/engine.go:3: bad"
	if got := Normalize(in); got != want {
		t.Fatalf("Normalize = %q, want %q", got, want)
	}
}
