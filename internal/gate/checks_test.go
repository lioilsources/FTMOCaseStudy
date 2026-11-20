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
