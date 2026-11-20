package gate

import (
	"fmt"
	"strings"
)

// Verdicts.
const (
	Pass     = "PASS"
	Fail     = "FAIL"     // goes back to the agent
	Escalate = "ESCALATE" // goes to a human
)

// VerdictInput is everything the verdict depends on. Decide is a pure function of it.
type VerdictInput struct {
	ProtectedTouched []string
	Tier2Changed     []string
	Spec             *Spec // nil when the run has no spec
	NewModules       []string
	Checks           []CheckResult
	Attempt          int
	MaxAttempts      int
}

// Decide applies the verdict rules in order and returns the verdict and one sentence of why.
func Decide(in VerdictInput) (string, string) {
	if len(in.ProtectedTouched) > 0 {
		return Escalate, "the change touches protected paths: " + strings.Join(in.ProtectedTouched, ", ")
	}
	var cannotRun, failed []string
	for _, c := range in.Checks {
		if c.Required && c.Status == StatusInconclusive {
			cannotRun = append(cannotRun, c.ID)
		}
		if c.Blocking && c.Status == StatusFail {
			failed = append(failed, c.ID)
		}
	}
	if len(cannotRun) > 0 {
		return Escalate, "required checks could not reach a result: " + strings.Join(cannotRun, ", ")
	}
	if len(in.Tier2Changed) > 0 && (in.Spec == nil || strings.TrimSpace(in.Spec.PlanApprovedBy) == "") {
		return Escalate, "tier-2 paths changed without an approved plan (plan_approved_by is empty): " + strings.Join(in.Tier2Changed, ", ")
	}
	if len(in.NewModules) > 0 {
		return Escalate, "go.mod gains modules, a human decision: " + strings.Join(in.NewModules, ", ")
	}
	if len(failed) > 0 && in.Attempt >= in.MaxAttempts {
		return Escalate, fmt.Sprintf("blocking checks failed (%s) on attempt %d of %d", strings.Join(failed, ", "), in.Attempt, in.MaxAttempts)
	}
	if len(failed) > 0 {
		return Fail, "blocking checks failed: " + strings.Join(failed, ", ")
	}
	return Pass, "every required check passed and no rule needs a human"
}

// ExitCode maps a verdict to the exit code of gate run in blocking mode.
func ExitCode(verdict string) int {
	switch verdict {
	case Pass:
		return 0
	case Fail:
		return 1
	case Escalate:
		return 2
	}
	return 3
}

// NewModules returns the modules required by head's go.mod and not by base's.
func NewModules(baseMod, headMod []byte) []string {
	have := map[string]bool{}
	for _, m := range requires(baseMod) {
		have[m] = true
	}
	var added []string
	for _, m := range requires(headMod) {
		if !have[m] {
			added = append(added, m)
		}
	}
	return added
}

// requires lists module paths in the require directives of a go.mod file.
func requires(mod []byte) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(string(mod), "\n") {
		line, _, _ = strings.Cut(line, "//")
		f := strings.Fields(line)
		switch {
		case len(f) == 0:
		case inBlock && f[0] == ")":
			inBlock = false
		case inBlock:
			out = append(out, f[0])
		case f[0] == "require" && len(f) >= 2 && f[1] == "(":
			inBlock = true
		case f[0] == "require" && len(f) >= 3:
			out = append(out, f[1])
		}
	}
	return out
}
