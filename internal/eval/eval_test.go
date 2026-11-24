package eval

import (
	"strings"
	"testing"

	"github.com/lioilsources/factory-gate/internal/gate"
)

func mkCase(verdict, fires, knownMiss string) Case {
	var c Case
	c.Name, c.Kind = "c", "bad"
	c.Expect.Verdict, c.Expect.Fires, c.KnownMiss = verdict, fires, knownMiss
	return c
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		c       Case
		verdict string
		fired   []string
		want    string
	}{
		{"match", mkCase("FAIL", "invariants", ""), "FAIL", []string{"build", "invariants"}, ResultOK},
		{"right verdict, wrong check", mkCase("FAIL", "invariants", ""), "FAIL", []string{"build"}, ResultMiss},
		{"wrong verdict", mkCase("FAIL", "invariants", ""), "PASS", nil, ResultMiss},
		{"known miss", mkCase("FAIL", "security", "not built"), "PASS", nil, ResultMissKnown},
		{"known miss resolved", mkCase("FAIL", "security", "not built"), "FAIL", []string{"security"}, ResultResolved},
		{"good case", mkCase("PASS", "", ""), "PASS", nil, ResultOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.c, tt.verdict, tt.fired); got != tt.want {
				t.Fatalf("Classify = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnexpected(t *testing.T) {
	for result, want := range map[string]bool{
		ResultOK: false, ResultMissKnown: false, ResultMiss: true, ResultResolved: true, ResultError: true,
	} {
		if got := (Outcome{Result: result}).Unexpected(); got != want {
			t.Errorf("Unexpected(%q) = %v, want %v", result, got, want)
		}
	}
}

func TestFired(t *testing.T) {
	ev := &gate.Evidence{
		Reason:           "x",
		ProtectedTouched: []string{"gate.yaml"},
		NewModules:       []string{"example.com/m"},
		Checks: []gate.CheckResult{
			{ID: "build", Blocking: true, Required: true, Status: gate.StatusPass},
			{ID: "invariants", Blocking: true, Required: true, Status: gate.StatusFail},
			{ID: "budget", Blocking: true, Required: true, Status: gate.StatusInconclusive},
		},
	}
	got := strings.Join(Fired(ev), ",")
	if want := "protected-path,invariants,budget:inconclusive,new-module"; got != want {
		t.Fatalf("Fired = %q, want %q", got, want)
	}
}

// The shipped cases must all load: names match directories, verdicts are valid, patches exist.
func TestShippedCasesLoad(t *testing.T) {
	cases, err := LoadCases("../../evals/cases")
	if err != nil {
		t.Fatal(err)
	}
	good, bad := 0, 0
	for _, c := range cases {
		if c.Kind == "good" {
			good++
		} else {
			bad++
		}
	}
	if good < 3 || bad < 6 {
		t.Fatalf("want at least 3 good and 6 bad cases, got %d and %d", good, bad)
	}
}

func TestMarkdownCounts(t *testing.T) {
	ok := Outcome{Case: mkCase("FAIL", "invariants", ""), Verdict: "FAIL", Fired: []string{"invariants"}, Result: ResultOK}
	miss := Outcome{Case: mkCase("FAIL", "security", "no security check"), Verdict: "PASS", Result: ResultMissKnown}
	md := Markdown([]Outcome{ok, miss}, "2000-01-01", "v", "main")
	for _, want := range []string{"source: seeded", "2 cases: 1 OK, 1 known misses, 0 unexpected.", "no security check", "## Loop matrix"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}
