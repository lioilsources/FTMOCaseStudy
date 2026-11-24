// Package eval measures the gate itself on static cases: hand-written changes with a known
// right answer. Each case is a patch applied to base in a temporary worktree, committed on a
// temporary branch and judged by the real gate. No model is involved.
package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lioilsources/factory-gate/internal/gate"
	"gopkg.in/yaml.v3"
)

// DefaultSpec is the spec a case is judged against when case.yaml does not name one.
const DefaultSpec = "specs/partial-fills.md"

// Case is one evals/cases/<name>/ directory.
type Case struct {
	Name        string  `yaml:"name"`
	Kind        string  `yaml:"kind"` // good or bad
	Description string  `yaml:"description"`
	Spec        *string `yaml:"spec"` // nil: DefaultSpec; "": no spec
	Expect      struct {
		Verdict string `yaml:"verdict"`
		Fires   string `yaml:"fires"` // a check id or a rule: protected-path, new-module, no-approved-plan
	} `yaml:"expect"`
	KnownMiss string `yaml:"known_miss"` // why the gate is known to get this case wrong

	Dir string `yaml:"-"`
}

// SpecPath is the spec the case is judged against, "" for none.
func (c Case) SpecPath() string {
	if c.Spec == nil {
		return DefaultSpec
	}
	return *c.Spec
}

// Results of a case.
const (
	ResultOK        = "OK"
	ResultMissKnown = "MISS (known)"
	ResultMiss      = "MISS"
	ResultResolved  = "OK, but marked known_miss" // the miss is gone: remove known_miss
	ResultError     = "ERROR"
)

// Outcome is what the gate did with one case.
type Outcome struct {
	Case     Case
	Verdict  string
	Fired    []string
	Result   string
	RunID    string
	Seconds  float64
	ErrorMsg string
}

// Unexpected reports whether the outcome should fail the eval run.
func (o Outcome) Unexpected() bool {
	return o.Result != ResultOK && o.Result != ResultMissKnown
}

// LoadCases reads every case under dir, sorted by name.
func LoadCases(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := LoadCase(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadCase reads one case directory.
func LoadCase(dir string) (Case, error) {
	var c Case
	data, err := os.ReadFile(filepath.Join(dir, "case.yaml"))
	if err != nil {
		return c, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", dir, err)
	}
	c.Dir = dir
	switch {
	case c.Name != filepath.Base(dir):
		return c, fmt.Errorf("%s: name %q does not match the directory", dir, c.Name)
	case c.Kind != "good" && c.Kind != "bad":
		return c, fmt.Errorf("%s: kind must be good or bad", dir)
	case c.Expect.Verdict != gate.Pass && c.Expect.Verdict != gate.Fail && c.Expect.Verdict != gate.Escalate:
		return c, fmt.Errorf("%s: expect.verdict must be PASS, FAIL or ESCALATE", dir)
	case c.Kind == "bad" && c.Expect.Fires == "":
		return c, fmt.Errorf("%s: a bad case names the check or rule expected to fire", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "change.patch")); err != nil {
		return c, fmt.Errorf("%s: %w", dir, err)
	}
	return c, nil
}

// Fired lists what stopped the change: failed blocking checks, checks that could not reach a
// result, and the verdict rules that need a human.
func Fired(ev *gate.Evidence) []string {
	var out []string
	if len(ev.ProtectedTouched) > 0 {
		out = append(out, "protected-path")
	}
	for _, c := range ev.Checks {
		switch {
		case c.Blocking && c.Status == gate.StatusFail:
			out = append(out, c.ID)
		case c.Required && c.Status == gate.StatusInconclusive:
			out = append(out, c.ID+":inconclusive")
		}
	}
	if strings.Contains(ev.Reason, "without an approved plan") {
		out = append(out, "no-approved-plan")
	}
	if len(ev.NewModules) > 0 {
		out = append(out, "new-module")
	}
	return out
}

// Classify compares what the gate did with what the case expects.
func Classify(c Case, verdict string, fired []string) string {
	ok := verdict == c.Expect.Verdict
	if ok && c.Expect.Fires != "" {
		ok = contains(fired, c.Expect.Fires)
	}
	switch {
	case ok && c.KnownMiss != "":
		return ResultResolved
	case ok:
		return ResultOK
	case c.KnownMiss != "":
		return ResultMissKnown
	}
	return ResultMiss
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Options configure an eval run.
type Options struct {
	RepoDir string
	Base    string
	Now     func() time.Time
	Log     func(format string, args ...any)
}

// RunCase applies the case to base on a temporary branch and runs the gate on it.
func RunCase(o Options, c Case) (out Outcome) {
	start := o.Now()
	out.Case = c
	// Named result: the deferred timer writes to the value the caller receives.
	defer func() { out.Seconds = o.Now().Sub(start).Seconds() }()
	ev, runID, err := judge(o, c)
	if err != nil {
		out.Result, out.ErrorMsg = ResultError, err.Error()
		return out
	}
	out.RunID, out.Verdict, out.Fired = runID, ev.Verdict, Fired(ev)
	out.Result = Classify(c, out.Verdict, out.Fired)
	return out
}

func judge(o Options, c Case) (*gate.Evidence, string, error) {
	g := gate.Git{Dir: o.RepoDir}
	patch, err := filepath.Abs(filepath.Join(c.Dir, "change.patch"))
	if err != nil {
		return nil, "", err
	}
	branch := fmt.Sprintf("eval/%s-%d", c.Name, o.Now().UnixNano())
	wt, err := gate.WorktreeDir("eval-" + c.Name)
	if err != nil {
		return nil, "", err
	}
	if _, err := g.Run("worktree", "add", "-q", "-b", branch, wt, o.Base); err != nil {
		return nil, "", err
	}
	defer func() { _, _ = g.Run("branch", "-q", "-D", branch) }()
	wg := gate.Git{Dir: wt}
	_, applyErr := wg.Run("apply", "--index", patch)
	if applyErr == nil {
		// --no-verify: an eval commit is a throwaway fixture on a local branch. Local hooks
		// would make the result depend on the machine, and a bad case may trip them on purpose.
		_, applyErr = wg.Run("-c", "user.name=eval", "-c", "user.email=eval@example.invalid",
			"commit", "-q", "--no-verify", "-m", "eval: "+c.Name)
	}
	g.RemoveWorktree(wt)
	if applyErr != nil {
		return nil, "", applyErr
	}
	res, err := gate.Run(gate.RunOptions{
		RepoDir: o.RepoDir, Base: o.Base, Head: branch, SpecPath: c.SpecPath(),
		Attempt: 1, RequestedBy: "eval", Source: "seeded", Now: o.Now,
	})
	if err != nil {
		return nil, "", err
	}
	return res.Evidence, res.Evidence.RunID, nil
}

// Run runs every case in order.
func Run(o Options, cases []Case) []Outcome {
	var out []Outcome
	for _, c := range cases {
		if o.Log != nil {
			o.Log("%-24s ", c.Name)
		}
		r := RunCase(o, c)
		if o.Log != nil {
			o.Log("%-8s %-22s %s (%.0fs)\n", r.Verdict, strings.Join(r.Fired, ","), r.Result, r.Seconds)
		}
		out = append(out, r)
	}
	return out
}

// Markdown renders evals/RESULTS.md.
func Markdown(outcomes []Outcome, date, gateVersion, base string) string {
	var b strings.Builder
	b.WriteString("# Gate evals\n\n")
	b.WriteString("source: seeded. Generated by `gate eval` (`make eval`); do not edit by hand.\n\n")
	fmt.Fprintf(&b, "Date: %s. Gate version: `%s`. Base: `%s`.\n\n", date, gateVersion, base)
	b.WriteString("Each case in `evals/cases/` is a hand-written change with a known right answer. It is applied to\n")
	b.WriteString("base on a temporary branch and judged by the real gate as attempt 1 of 3, so a blocking\n")
	b.WriteString("failure reads FAIL. A known miss is a case the gate gets wrong today, with the reason; the\n")
	b.WriteString("gate is not tuned to the case.\n\n")
	b.WriteString("## Static cases\n\n")
	b.WriteString("| Case | Kind | Expected | Got | Fired | Result | Seconds |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	counts := map[string]int{}
	for _, o := range outcomes {
		exp := o.Case.Expect.Verdict
		if o.Case.Expect.Fires != "" {
			exp += " (" + o.Case.Expect.Fires + ")"
		}
		got := o.Verdict
		if o.Result == ResultError {
			got = "error"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %.0f |\n",
			o.Case.Name, o.Case.Kind, exp, got, strings.Join(o.Fired, ", "), o.Result, o.Seconds)
		counts[o.Result]++
	}
	fmt.Fprintf(&b, "\n%d cases: %d OK, %d known misses, %d unexpected.\n",
		len(outcomes), counts[ResultOK], counts[ResultMissKnown],
		len(outcomes)-counts[ResultOK]-counts[ResultMissKnown])

	var misses []Outcome
	for _, o := range outcomes {
		if o.Case.KnownMiss != "" || o.Unexpected() {
			misses = append(misses, o)
		}
	}
	if len(misses) > 0 {
		b.WriteString("\n## Misses\n\n")
		for _, o := range misses {
			why := o.Case.KnownMiss
			if o.ErrorMsg != "" {
				why = "error: " + o.ErrorMsg
			} else if why == "" {
				why = "unexpected; not yet explained"
			}
			fmt.Fprintf(&b, "- **%s** (%s): %s\n", o.Case.Name, o.Result, why)
		}
	}
	b.WriteString("\n## Loop matrix\n\nNot built. The matrix (implementer model by context profile, N runs per cell) is\n")
	b.WriteString("Phase 7's second half; `demo/RUNS.md` lists the recorded loop runs so far.\n")
	return b.String()
}
