package gate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

// Check statuses.
const (
	StatusPass         = "pass"
	StatusFail         = "fail"
	StatusInconclusive = "inconclusive" // the check could not reach a result
	StatusNotRun       = "not_run"      // skipped because the build failed
)

// CheckResult is the outcome of one check.
type CheckResult struct {
	ID       string `json:"id"`
	Required bool   `json:"required"`
	Blocking bool   `json:"blocking"`
	Status   string `json:"status"`
	Ms       int64  `json:"ms"`
	Key      string `json:"key"`              // the one number or fact for the table
	Detail   string `json:"detail,omitempty"` // verbatim tool output for the evidence
	Log      string `json:"-"`                // full output, written to logs/<id>.log
}

// maxDetailLines caps verbatim output in the evidence; the full output is in the log.
const maxDetailLines = 120

func capLines(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= maxDetailLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:maxDetailLines], "\n") + fmt.Sprintf("\n... (%d more lines in the log)", len(lines)-maxDetailLines)
}

// cmdResult is the outcome of one external command.
type cmdResult struct {
	out      string
	exitCode int
	timedOut bool
	err      error // the command could not start
}

func runCmd(dir string, timeout time.Duration, name string, args ...string) cmdResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly", "GOPROXY=off")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	res := cmdResult{out: out.String()}
	var exit *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.timedOut = true
	case errors.As(err, &exit):
		res.exitCode = exit.ExitCode()
	case err != nil:
		res.err = err
	}
	return res
}

func timeoutOf(c *CheckConfig, fallback int) time.Duration {
	if c != nil && c.TimeoutSeconds > 0 {
		return time.Duration(c.TimeoutSeconds) * time.Second
	}
	return time.Duration(fallback) * time.Second
}

// runBuild runs go build, go vet and the existing unit tests, except the invariants package.
func runBuild(dir string, cfg *Config) CheckResult {
	c := cfg.Checks.Build
	r := CheckResult{ID: "build", Required: c.Required, Blocking: true}
	start := time.Now()
	defer func() { r.Ms = time.Since(start).Milliseconds() }()
	timeout := timeoutOf(c, 300)
	var log strings.Builder

	for _, step := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		res := runCmd(dir, timeout, "go", step...)
		fmt.Fprintf(&log, "$ go %s\n%s\n", strings.Join(step, " "), res.out)
		if bad := badRun(&r, res, "go "+step[0]); bad {
			r.Log = log.String()
			return r
		}
		if res.exitCode != 0 {
			r.Status, r.Key, r.Detail, r.Log = StatusFail, "go "+step[0]+" failed", capLines(res.out), log.String()
			return r
		}
	}

	list := runCmd(dir, timeout, "go", "list", "./...")
	if badRun(&r, list, "go list") || list.exitCode != 0 {
		r.Status, r.Key, r.Log = StatusInconclusive, "go list failed", list.out
		return r
	}
	inv := ""
	if cfg.Checks.Invariants != nil {
		inv = path.Clean(cfg.Checks.Invariants.Package)
	}
	args := []string{"test", "-count=1"}
	for _, pkg := range strings.Fields(list.out) {
		if inv == "" || !strings.HasSuffix(pkg, "/"+inv) {
			args = append(args, pkg)
		}
	}
	res := runCmd(dir, timeout, "go", args...)
	fmt.Fprintf(&log, "$ go %s\n%s\n", strings.Join(args, " "), res.out)
	r.Log = log.String()
	if badRun(&r, res, "go test") {
		return r
	}
	failing := failLines(res.out)
	if res.exitCode != 0 {
		r.Status, r.Key, r.Detail = StatusFail, fmt.Sprintf("%d failing", len(failing)), capLines(strings.Join(failing, "\n")+"\n\n"+res.out)
		return r
	}
	r.Status, r.Key = StatusPass, "build, vet, unit tests ok"
	return r
}

// badRun marks r inconclusive when the command timed out or could not start.
func badRun(r *CheckResult, res cmdResult, what string) bool {
	switch {
	case res.timedOut:
		r.Status, r.Key = StatusInconclusive, what+" timed out"
	case res.err != nil:
		r.Status, r.Key = StatusInconclusive, fmt.Sprintf("%s could not run: %v", what, res.err)
	default:
		return false
	}
	return true
}

// failLines picks the "--- FAIL" and "FAIL\t<pkg>" lines out of go test output.
func failLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "--- FAIL") || strings.HasPrefix(l, "FAIL\t") {
			lines = append(lines, t)
		}
	}
	return lines
}

// runInvariants runs the property tests (already overlaid from base) with a fixed number
// of checks and seed. The failing output is captured verbatim, not parsed.
func runInvariants(dir string, cfg *Config) CheckResult {
	c := cfg.Checks.Invariants
	r := CheckResult{ID: "invariants", Required: c.Required, Blocking: true}
	start := time.Now()
	defer func() { r.Ms = time.Since(start).Milliseconds() }()
	args := []string{"test", "-count=1", "-v", c.Package,
		fmt.Sprintf("-rapid.checks=%d", c.Checks), fmt.Sprintf("-rapid.seed=%d", c.Seed), "-rapid.nofailfile"}
	res := runCmd(dir, timeoutOf(c, 120), "go", args...)
	r.Log = "$ go " + strings.Join(args, " ") + "\n" + res.out
	if badRun(&r, res, "invariants") {
		return r
	}
	seed := fmt.Sprintf("seed %d, %d checks", c.Seed, c.Checks)
	switch {
	case res.exitCode == 0 && strings.Contains(res.out, "--- SKIP"):
		r.Status, r.Key = StatusInconclusive, "invariants skipped (not written?), "+seed
	case res.exitCode == 0 && strings.Contains(res.out, "--- PASS"):
		r.Status, r.Key = StatusPass, "held, "+seed
	case res.exitCode == 0:
		r.Status, r.Key = StatusInconclusive, "no invariant test ran, "+seed
	case strings.Contains(res.out, "--- FAIL"):
		r.Status, r.Key, r.Detail = StatusFail, "counterexample found, "+seed, capLines(failBlock(res.out))
	default: // for example, the base invariants do not compile against head's API
		r.Status, r.Key, r.Detail = StatusInconclusive, "invariants did not run, "+seed, capLines(res.out)
	}
	return r
}

// failBlock returns the output from the first "--- FAIL" line up to the final FAIL line.
func failBlock(out string) string {
	i := strings.Index(out, "--- FAIL")
	if i < 0 {
		return out
	}
	block := out[i:]
	if j := strings.Index(block, "\nFAIL"); j >= 0 {
		block = block[:j]
	}
	return block
}

// runBudget compares the size of the change with the budget and the spec's scope.
func runBudget(cfg *Config, spec *Spec, changes []FileChange) CheckResult {
	r := CheckResult{ID: "budget", Required: cfg.Checks.Budget.Required, Blocking: true}
	lines := 0
	var outOfScope []string
	for _, f := range changes {
		if !f.IsTest() {
			lines += f.Added + f.Deleted
		}
		if spec != nil && !MatchAny(spec.AllowedPaths, f.Path) {
			outOfScope = append(outOfScope, f.Path)
		}
	}
	b := cfg.Budget
	r.Key = fmt.Sprintf("%d/%d lines, %d/%d files", lines, b.MaxChangedLines, len(changes), b.MaxFiles)
	var problems []string
	if lines > b.MaxChangedLines {
		problems = append(problems, fmt.Sprintf("changed non-test lines %d exceed %d", lines, b.MaxChangedLines))
	}
	if len(changes) > b.MaxFiles {
		problems = append(problems, fmt.Sprintf("changed files %d exceed %d", len(changes), b.MaxFiles))
	}
	if len(outOfScope) > 0 {
		problems = append(problems, "outside the spec's allowed_paths: "+strings.Join(outOfScope, ", "))
		r.Key += fmt.Sprintf(", %d out of scope", len(outOfScope))
	}
	r.Status = StatusPass
	if len(problems) > 0 {
		r.Status, r.Detail = StatusFail, strings.Join(problems, "\n")
	}
	return r
}
