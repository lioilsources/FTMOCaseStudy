package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// AgentRecord says which agent made the change under test.
type AgentRecord struct {
	Role         string  `json:"role"`
	Kind         string  `json:"kind"`
	Model        string  `json:"model"`
	Endpoint     string  `json:"endpoint"`
	PromptSHA256 string  `json:"prompt_sha256"`
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	Seconds      float64 `json:"seconds"`
	Source       string  `json:"source,omitempty"` // live or replayed
}

// Provenance is where the change and the verdict come from.
type Provenance struct {
	Base        string            `json:"base"`
	MergeBase   string            `json:"merge_base"`
	Head        string            `json:"head"`
	RequestedBy string            `json:"requested_by"`
	Agents      []AgentRecord     `json:"agents"`
	Trailers    map[string]string `json:"trailers,omitempty"`
	SpecPath    string            `json:"spec_path,omitempty"`
	SpecSHA256  string            `json:"spec_sha256,omitempty"`
	Attempt     int               `json:"attempt"`
	MaxAttempts int               `json:"max_attempts"`
	GateVersion string            `json:"gate_version"`
	Mode        string            `json:"mode"`
	Source      string            `json:"source"`
	SameModel   bool              `json:"same_model,omitempty"`
}

// Location is one place a reviewer should read first.
type Location struct {
	Hunk
	Why string `json:"why"`
}

// Evidence is what a reviewer reads instead of starting from the diff.
type Evidence struct {
	RunID            string        `json:"run_id"`
	Verdict          string        `json:"verdict"`
	Reason           string        `json:"reason"`
	Checks           []CheckResult `json:"checks"`
	ReadFirst        []Location    `json:"read_first"`
	Provenance       Provenance    `json:"provenance"`
	Files            []FileChange  `json:"files"`
	ProtectedTouched []string      `json:"protected_touched,omitempty"`
	NewModules       []string      `json:"new_modules,omitempty"`
	Limits           []string      `json:"limits"`
}

// Limits is the static "what this gate cannot tell you" section.
var Limits = []string{
	"The gate is only as good as its invariants. With none written, it catches nothing that tests would not.",
	"The invariants here are illustrative. Real ones have to come from the people who know the domain.",
	"Passing the gate is not proof of correctness. It is evidence that named properties held under a stated number of generated cases with a recorded seed.",
	"Loop results vary from run to run. One run proves nothing by itself.",
	"The completion implementer rewrites whole files and cannot run tests itself.",
	"The independent test author may share blind spots with the implementer.",
	"Identities are asserted, not authenticated. In production, requester and approver come from the platform.",
	"The audit log is a local file standing in for the platform's record.",
	"With the command adapter the agent has a shell in the worktree and is not sandboxed.",
}

// ReadFirst picks at most three locations by a fixed rule, never by a model:
// hunks in tier-2 paths (largest first), then the largest hunk overall.
// Lines with surviving mutants come first once the mutation check exists.
func ReadFirst(cfg *Config, hunks []Hunk) []Location {
	sorted := append([]Hunk(nil), hunks...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Lines > sorted[j].Lines })
	var out []Location
	seen := map[Hunk]bool{}
	add := func(h Hunk, why string) {
		if len(out) < 3 && !seen[h] {
			seen[h] = true
			out = append(out, Location{h, why})
		}
	}
	for _, h := range sorted {
		if cfg.TierOf(h.Path) == 2 {
			add(h, "tier-2 path")
		}
	}
	if len(sorted) > 0 {
		add(sorted[0], "largest hunk")
	}
	return out
}

// WriteEvidence writes evidence.md, evidence.json, logs/ and, on FAIL, failure_report.md.
func WriteEvidence(dir string, ev *Evidence) error {
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return err
	}
	for _, c := range ev.Checks {
		if c.Log != "" {
			if err := os.WriteFile(filepath.Join(dir, "logs", c.ID+".log"), []byte(c.Log), 0o644); err != nil {
				return err
			}
		}
	}
	js, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence.json"), append(js, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence.md"), []byte(EvidenceMarkdown(ev)), 0o644); err != nil {
		return err
	}
	if ev.Verdict == Fail {
		return os.WriteFile(filepath.Join(dir, "failure_report.md"), []byte(FailureReport(ev)), 0o644)
	}
	return nil
}

// EvidenceMarkdown renders the evidence in the fixed section order.
func EvidenceMarkdown(ev *Evidence) string {
	var b strings.Builder
	p := ev.Provenance
	fmt.Fprintf(&b, "# Evidence %s\n\nsource: %s\n\n## Verdict: %s\n\n%s.\n\n", ev.RunID, p.Source, ev.Verdict, capitalize(ev.Reason))

	b.WriteString("## Checks\n\n| Check | Status | Required | ms | Key |\n| --- | --- | --- | --- | --- |\n")
	for _, c := range ev.Checks {
		fmt.Fprintf(&b, "| %s | %s | %v | %d | %s |\n", c.ID, c.Status, c.Required, c.Ms, c.Key)
	}

	b.WriteString("\n## Counterexample and findings\n\n")
	found := false
	for _, c := range ev.Checks {
		if c.Detail != "" {
			found = true
			fmt.Fprintf(&b, "### %s\n\n```\n%s\n```\n\n", c.ID, c.Detail)
		}
	}
	if len(ev.ProtectedTouched) > 0 {
		found = true
		fmt.Fprintf(&b, "### protected paths touched\n\n- %s\n\n", strings.Join(ev.ProtectedTouched, "\n- "))
	}
	if len(ev.NewModules) > 0 {
		found = true
		fmt.Fprintf(&b, "### new modules in go.mod\n\n- %s\n\n", strings.Join(ev.NewModules, "\n- "))
	}
	if !found {
		b.WriteString("None.\n\n")
	}

	b.WriteString("## Read these first\n\n")
	if len(ev.ReadFirst) == 0 {
		b.WriteString("No hunks.\n")
	}
	for _, l := range ev.ReadFirst {
		fmt.Fprintf(&b, "- `%s:%d` (%d changed lines, %s)\n", l.Path, l.Start, l.Lines, l.Why)
	}

	b.WriteString("\n## Provenance\n\n")
	fmt.Fprintf(&b, "- base: `%s` (rules)\n- merge base: `%s` (diff from here)\n- head: `%s`\n- requested by: %s\n", p.Base, p.MergeBase, p.Head, p.RequestedBy)
	if len(p.Agents) == 0 {
		b.WriteString("- agent: none named in the commit trailers\n")
	}
	for _, a := range p.Agents {
		fmt.Fprintf(&b, "- agent: %s/%s, model `%s`, endpoint %s, prompt sha256 `%s`, tokens %d in / %d out, %.1f s, %s\n",
			a.Role, a.Kind, a.Model, a.Endpoint, a.PromptSHA256, a.TokensIn, a.TokensOut, a.Seconds, orUnknown(a.Source))
	}
	if p.SameModel {
		b.WriteString("- same model: reduced independence\n")
	}
	if p.SpecPath != "" {
		fmt.Fprintf(&b, "- spec: `%s` sha256 `%s`\n", p.SpecPath, p.SpecSHA256)
	} else {
		b.WriteString("- spec: none\n")
	}
	fmt.Fprintf(&b, "- attempt: %d of %d\n- gate version: `%s`\n- mode: %s\n- source: %s\n", p.Attempt, p.MaxAttempts, p.GateVersion, p.Mode, p.Source)

	b.WriteString("\n## What this gate cannot tell you\n\n")
	for _, l := range ev.Limits {
		fmt.Fprintf(&b, "- %s\n", l)
	}
	return b.String()
}

// FailureReport is what goes back to the model: the failing checks and their evidence,
// nothing else, and no suggested fix. It is deterministic for the same code (no run id,
// timings or worktree paths), so a replayed loop sends the model the same prompt.
func FailureReport(ev *Evidence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Failure report (attempt %d)\n\n", ev.Provenance.Attempt)
	for _, c := range ev.Checks {
		if c.Status != StatusFail {
			continue
		}
		fmt.Fprintf(&b, "## %s: %s\n\n", c.ID, c.Key)
		if c.Detail != "" {
			fmt.Fprintf(&b, "```\n%s\n```\n\n", Normalize(c.Detail))
		}
	}
	return b.String()
}

var (
	testTimings   = regexp.MustCompile(`\(\d+(\.\d+)?m?s\)`)
	pkgTimings    = regexp.MustCompile(`(?m)\t\d+(\.\d+)?m?s$`)
	worktreePaths = regexp.MustCompile(`\S*/\.factory/worktrees/[^/\s]+/`)
)

// Normalize removes timings and worktree paths from tool output.
func Normalize(s string) string {
	s = testTimings.ReplaceAllString(s, "")
	s = pkgTimings.ReplaceAllString(s, "")
	return worktreePaths.ReplaceAllString(s, "")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func orUnknown(s string) string {
	if s == "" {
		return "source unknown"
	}
	return s
}
