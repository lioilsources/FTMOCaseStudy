package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

// RunOptions are the inputs of one gate run.
type RunOptions struct {
	RepoDir     string // the main repository; .factory/ lives here
	Base, Head  string
	SpecPath    string // repo-relative, read from base; "" for none
	Mode        string // blocking or advisory
	Attempt     int
	RequestedBy string
	Source      string        // live or replayed
	RunID       string        // "" picks the next free id
	Agents      []AgentRecord // nil reads them from head's commit trailers
	Now         func() time.Time
}

// RunResult is the outcome of a gate run.
type RunResult struct {
	Evidence *Evidence
	Dir      string // .factory/runs/<run-id>
}

// FactoryDir is the runtime output directory of a repository.
func FactoryDir(repo string) string { return filepath.Join(repo, ".factory") }

// Run judges head against the rules of base and writes evidence and an audit record.
func Run(o RunOptions) (*RunResult, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Mode == "" {
		o.Mode = "blocking"
	}
	if o.Source == "" {
		o.Source = "live"
	}
	if o.Attempt <= 0 {
		o.Attempt = 1
	}
	g := Git{Dir: o.RepoDir}
	base, err := g.RevParse(o.Base)
	if err != nil {
		return nil, err
	}
	head, err := g.RevParse(o.Head)
	if err != nil {
		return nil, err
	}

	// Rules come from base.
	raw, err := g.Show(base, "gate.yaml")
	if err != nil {
		return nil, fmt.Errorf("gate.yaml at base: %w", err)
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		return nil, err
	}
	var spec *Spec
	if o.SpecPath != "" {
		data, err := g.Show(base, o.SpecPath)
		if err != nil {
			return nil, fmt.Errorf("spec must exist at base: %w", err)
		}
		if spec, err = ParseSpec(o.SpecPath, data); err != nil {
			return nil, err
		}
	}

	// The change is what head adds since it forked from base, as in a pull request.
	// Commits that landed on base after the fork are not part of it.
	fork, err := g.MergeBase(base, head)
	if err != nil {
		return nil, err
	}
	changes, err := g.Diff(fork, head)
	if err != nil {
		return nil, err
	}
	hunks, err := g.Hunks(fork, head)
	if err != nil {
		return nil, err
	}
	var protected, tier2 []string
	for _, f := range changes {
		if cfg.IsProtected(f.Path) {
			protected = append(protected, f.Path)
		}
		if cfg.TierOf(f.Path) == 2 {
			tier2 = append(tier2, f.Path)
		}
	}
	baseMod, _ := g.Show(fork, "go.mod")
	headMod, _ := g.Show(head, "go.mod")
	newMods := NewModules(baseMod, headMod)

	runID := o.RunID
	if runID == "" {
		if runID, err = nextRunID(o.RepoDir); err != nil {
			return nil, err
		}
	}
	runDir := filepath.Join(FactoryDir(o.RepoDir), "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}

	checks, err := runChecks(g, cfg, spec, base, head, runID, changes)
	if err != nil {
		return nil, err
	}

	verdict, reason := Decide(VerdictInput{
		ProtectedTouched: protected, Tier2Changed: tier2, Spec: spec, NewModules: newMods,
		Checks: checks, Attempt: o.Attempt, MaxAttempts: cfg.MaxAttempts,
	})

	agents := o.Agents
	trailers, _ := g.Trailers(head)
	if agents == nil {
		agents = agentsFromTrailers(trailers)
	}
	ev := &Evidence{
		RunID: runID, Verdict: verdict, Reason: reason, Checks: checks,
		ReadFirst: ReadFirst(cfg, hunks), Files: changes,
		ProtectedTouched: protected, NewModules: newMods, Limits: Limits,
		Provenance: Provenance{
			Base: base, MergeBase: fork, Head: head, RequestedBy: o.RequestedBy, Agents: agents, Trailers: trailers,
			Attempt: o.Attempt, MaxAttempts: cfg.MaxAttempts, GateVersion: Version(),
			Mode: o.Mode, Source: o.Source, SameModel: sameModel(agents),
		},
	}
	if spec != nil {
		ev.Provenance.SpecPath, ev.Provenance.SpecSHA256 = spec.Path, spec.SHA256
	}
	if err := WriteEvidence(runDir, ev); err != nil {
		return nil, err
	}
	if err := AppendAudit(filepath.Join(FactoryDir(o.RepoDir), "audit.jsonl"), AuditFromEvidence(ev, o.Now())); err != nil {
		return nil, err
	}
	return &RunResult{Evidence: ev, Dir: runDir}, nil
}

// runChecks checks head out into a worktree, overlays protected paths from base and runs
// every configured check. Only a failed compile stops the later code checks.
func runChecks(g Git, cfg *Config, spec *Spec, base, head, runID string, changes []FileChange) ([]CheckResult, error) {
	wt, err := WorktreeDir(runID)
	if err != nil {
		return nil, err
	}
	if err := g.AddWorktree(wt, head); err != nil {
		return nil, err
	}
	defer g.RemoveWorktree(wt)
	if err := g.OverlayProtected(cfg, base, head, wt); err != nil {
		return nil, err
	}

	var out []CheckResult
	compiled := true
	if cfg.Checks.Build != nil {
		var r CheckResult
		r, compiled = runBuild(wt, cfg)
		out = append(out, r)
	}
	if c := cfg.Checks.Invariants; c != nil {
		if compiled {
			out = append(out, runInvariants(wt, cfg))
		} else {
			out = append(out, CheckResult{ID: "invariants", Required: c.Required, Blocking: true, Status: StatusNotRun, Key: "build failed"})
		}
	}
	if cfg.Checks.Budget != nil {
		out = append(out, runBudget(cfg, spec, changes))
	}
	return out, nil
}

// WorktreeDir returns a fresh directory for a worktree on the system temp disk. The
// repository may live on a slow volume; checks are far faster on local storage, and the
// worktree is removed after the run.
func WorktreeDir(name string) (string, error) {
	parent, err := os.MkdirTemp("", "factory-gate-")
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, name), nil
}

// nextRunID returns r-NNNN, one above the highest existing run.
func nextRunID(repo string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(FactoryDir(repo), "runs"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	high := 0
	for _, e := range entries {
		var n int
		if _, err := fmt.Sscanf(e.Name(), "r-%04d", &n); err == nil && n > high {
			high = n
		}
	}
	return fmt.Sprintf("r-%04d", high+1), nil
}

// agentsFromTrailers reads the agent named in the commit trailers, if any.
func agentsFromTrailers(t map[string]string) []AgentRecord {
	agent, ok := t["Agent"]
	if !ok {
		return nil
	}
	role, kind, _ := strings.Cut(agent, "/")
	return []AgentRecord{{Role: role, Kind: kind, Model: t["Model"], Endpoint: t["Endpoint"], PromptSHA256: t["Prompt-SHA256"]}}
}

func sameModel(agents []AgentRecord) bool {
	seen := map[string]string{}
	for _, a := range agents {
		if role, ok := seen[a.Model]; ok && role != a.Role && a.Model != "" {
			return true
		}
		seen[a.Model] = a.Role
	}
	return false
}

// Version is the VCS revision the gate binary was built from.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, dirty := "unknown", ""
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision":
			rev = s.Value
		case s.Key == "vcs.modified" && s.Value == "true":
			dirty = "-dirty"
		}
	}
	return rev + dirty
}
