package gate

import "testing"

func TestDecide(t *testing.T) {
	approved := &Spec{ID: "s", PlanApprovedBy: "olin"}
	unapproved := &Spec{ID: "s"}
	pass := CheckResult{ID: "build", Required: true, Blocking: true, Status: StatusPass}
	fail := CheckResult{ID: "invariants", Required: true, Blocking: true, Status: StatusFail}
	stuck := CheckResult{ID: "invariants", Required: true, Blocking: true, Status: StatusInconclusive}
	notRun := CheckResult{ID: "invariants", Required: true, Blocking: true, Status: StatusNotRun}
	reportOnly := CheckResult{ID: "mutation", Required: false, Blocking: false, Status: StatusFail}
	optionalStuck := CheckResult{ID: "vuln", Required: false, Blocking: true, Status: StatusInconclusive}

	tests := []struct {
		name string
		in   VerdictInput
		want string
	}{
		{"all pass", VerdictInput{Checks: []CheckResult{pass}, Attempt: 1, MaxAttempts: 3}, Pass},
		{"blocking fail goes back to the agent", VerdictInput{Checks: []CheckResult{pass, fail}, Attempt: 1, MaxAttempts: 3}, Fail},
		{"fail on the last attempt escalates", VerdictInput{Checks: []CheckResult{fail}, Attempt: 3, MaxAttempts: 3}, Escalate},
		{"protected path beats everything", VerdictInput{ProtectedTouched: []string{"gate.yaml"}, Checks: []CheckResult{pass}, Attempt: 1, MaxAttempts: 3}, Escalate},
		{"protected path beats a failure", VerdictInput{ProtectedTouched: []string{"gate.yaml"}, Checks: []CheckResult{fail}, Attempt: 1, MaxAttempts: 3}, Escalate},
		{"required check cannot run", VerdictInput{Checks: []CheckResult{pass, stuck}, Attempt: 1, MaxAttempts: 3}, Escalate},
		{"optional check cannot run", VerdictInput{Checks: []CheckResult{pass, optionalStuck}, Attempt: 1, MaxAttempts: 3}, Pass},
		{"not run after a failed build is not an escalation", VerdictInput{Checks: []CheckResult{{ID: "build", Required: true, Blocking: true, Status: StatusFail}, notRun}, Attempt: 1, MaxAttempts: 3}, Fail},
		{"tier 2 without spec", VerdictInput{Tier2Changed: []string{"sample/orders/engine.go"}, Checks: []CheckResult{pass}, Attempt: 1, MaxAttempts: 3}, Escalate},
		{"tier 2 with unapproved plan", VerdictInput{Tier2Changed: []string{"sample/orders/engine.go"}, Spec: unapproved, Checks: []CheckResult{pass}, Attempt: 1, MaxAttempts: 3}, Escalate},
		{"tier 2 with approved plan", VerdictInput{Tier2Changed: []string{"sample/orders/engine.go"}, Spec: approved, Checks: []CheckResult{pass}, Attempt: 1, MaxAttempts: 3}, Pass},
		{"tier 2 approval does not hide a failure", VerdictInput{Tier2Changed: []string{"sample/orders/engine.go"}, Spec: approved, Checks: []CheckResult{fail}, Attempt: 1, MaxAttempts: 3}, Fail},
		{"new module", VerdictInput{NewModules: []string{"example.com/x"}, Checks: []CheckResult{pass}, Attempt: 1, MaxAttempts: 3}, Escalate},
		{"report-only failure", VerdictInput{Checks: []CheckResult{pass, reportOnly}, Attempt: 1, MaxAttempts: 3}, Pass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, why := Decide(tt.in)
			if got != tt.want {
				t.Fatalf("Decide = %s (%s), want %s", got, why, tt.want)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	for v, want := range map[string]int{Pass: 0, Fail: 1, Escalate: 2, "?": 3} {
		if got := ExitCode(v); got != want {
			t.Errorf("ExitCode(%s) = %d, want %d", v, got, want)
		}
	}
}

func TestNewModules(t *testing.T) {
	base := []byte("module m\n\ngo 1.25\n\nrequire (\n\tpgregory.net/rapid v1.3.0\n\tgopkg.in/yaml.v3 v3.0.1 // indirect\n)\n")
	tests := []struct {
		name string
		head string
		want []string
	}{
		{"unchanged", string(base), nil},
		{"version bump is not a new module", "module m\nrequire (\n\tpgregory.net/rapid v1.4.0\n\tgopkg.in/yaml.v3 v3.0.1\n)\n", nil},
		{"block addition", "module m\nrequire (\n\tpgregory.net/rapid v1.3.0\n\tgopkg.in/yaml.v3 v3.0.1\n\texample.com/new v0.1.0\n)\n", []string{"example.com/new"}},
		{"single-line addition", string(base) + "require example.com/one v1.0.0\n", []string{"example.com/one"}},
		{"removal is not an addition", "module m\nrequire pgregory.net/rapid v1.3.0\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewModules(base, []byte(tt.head))
			if len(got) != len(tt.want) || (len(got) > 0 && got[0] != tt.want[0]) {
				t.Fatalf("NewModules = %v, want %v", got, tt.want)
			}
		})
	}
}
