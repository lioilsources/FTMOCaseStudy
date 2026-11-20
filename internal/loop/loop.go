// Package loop runs the factory loop: the implementer proposes a change, the gate judges
// it, a failure report goes back to the implementer, and only a PASS reaches a person.
package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lioilsources/factory-gate/internal/agent"
	"github.com/lioilsources/factory-gate/internal/gate"
)

// AgentError is the attempt outcome when the model's reply could not be used.
// It is kept apart from the gate's verdicts in every statistic.
const AgentError = "agent_error"

// Options are the inputs of one loop.
type Options struct {
	RepoDir     string
	Base        string
	SpecPath    string
	MaxAttempts int    // 0 means gate.yaml's max_attempts
	Context     string // spec-only or spec+invariants; "" means the role's
	Role        agent.RoleConfig
	Caller      agent.Caller
	Source      string // live or replayed
	RequestedBy string
	AuthorEmail string
	Stream      io.Writer // model output as it arrives; nil for none
	Out         io.Writer // progress
	Now         func() time.Time
}

// Attempt is one turn of the loop.
type Attempt struct {
	N         int     `json:"n"`
	Model     string  `json:"model"`
	Outcome   string  `json:"outcome"` // PASS, FAIL, ESCALATE or agent_error
	Reason    string  `json:"reason,omitempty"`
	RunID     string  `json:"run_id,omitempty"`
	Branch    string  `json:"branch,omitempty"`
	Head      string  `json:"head,omitempty"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	Seconds   float64 `json:"seconds"`
	Calls     int     `json:"calls"`
}

// Result is the outcome of a loop.
type Result struct {
	LoopID   string    `json:"loop_id"`
	Spec     string    `json:"spec"`
	Context  string    `json:"context"`
	Source   string    `json:"source"`
	Endpoint string    `json:"endpoint"`
	Base     string    `json:"base"`
	Attempts []Attempt `json:"attempts"`
	Final    string    `json:"final"`
}

// Run runs attempts until PASS, ESCALATE or the attempt limit.
func Run(ctx context.Context, o Options) (*Result, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	g := gate.Git{Dir: o.RepoDir}
	base, err := g.RevParse(o.Base)
	if err != nil {
		return nil, err
	}
	cfg, spec, promptText, err := loadFromBase(g, base, o.SpecPath)
	if err != nil {
		return nil, err
	}
	maxAttempts := o.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = cfg.MaxAttempts
	}
	contextProfile := o.Context
	if contextProfile == "" {
		contextProfile = o.Role.Context
	}
	if contextProfile == "" {
		contextProfile = "spec-only"
	}
	var invariants []agent.File
	if contextProfile == "spec+invariants" {
		if invariants, err = invariantSources(g, cfg, base); err != nil {
			return nil, err
		}
	} else if contextProfile != "spec-only" {
		return nil, fmt.Errorf("unknown context profile %q", contextProfile)
	}
	promptSum := sha256.Sum256([]byte(promptText))
	promptSHA := hex.EncodeToString(promptSum[:])

	loopID, err := nextLoopID(o.RepoDir)
	if err != nil {
		return nil, err
	}
	res := &Result{LoopID: loopID, Spec: o.SpecPath, Context: contextProfile, Source: o.Source,
		Endpoint: o.Role.Endpoint.Label, Base: base}
	fmt.Fprintf(o.Out, "loop %s: spec %s, context %s, up to %d attempts, %s\n", loopID, o.SpecPath, contextProfile, maxAttempts, o.Source)

	from := base // attempt 1 starts from base, later ones from the previous attempt
	feedback := ""
	for n := 1; n <= maxAttempts; n++ {
		a, head, report, err := attempt(ctx, o, g, cfg, spec, promptText, promptSHA, invariants, loopID, base, from, feedback, n)
		if err != nil {
			return res, err
		}
		res.Attempts = append(res.Attempts, a)
		res.Final = a.Outcome
		fmt.Fprintf(o.Out, "attempt %d: %s %s\n", n, a.Outcome, a.Reason)
		if a.Outcome == gate.Pass || a.Outcome == gate.Escalate {
			break
		}
		if head != "" {
			from = head
		}
		feedback = report
	}
	return res, writeSummary(o.RepoDir, res)
}

// attempt runs one turn. It returns the new head ("" when nothing was committed) and the
// feedback for the next turn.
func attempt(ctx context.Context, o Options, g gate.Git, cfg *gate.Config, spec *gate.Spec, promptText, promptSHA string,
	invariants []agent.File, loopID, base, from, feedback string, n int) (Attempt, string, string, error) {
	model := o.Role.ModelFor(n)
	a := Attempt{N: n, Model: model}
	branch := fmt.Sprintf("loop/%s/attempt-%d", loopID, n)
	wt, err := gate.WorktreeDir(fmt.Sprintf("%s-a%d", loopID, n))
	if err != nil {
		return a, "", "", err
	}
	if _, err := g.Run("worktree", "add", "-q", "-b", branch, wt, from); err != nil {
		return a, "", "", err
	}
	defer g.RemoveWorktree(wt)

	current, err := currentFiles(wt, spec)
	if err != nil {
		return a, "", "", err
	}
	req := agent.Request{Role: o.Role.Name, Model: model, Timeout: o.Role.Timeout, Stream: o.Stream,
		Messages: []agent.Message{
			{Role: "system", Content: promptText},
			{Role: "user", Content: userPrompt(spec, current, invariants, feedback)},
		}}
	fmt.Fprintf(o.Out, "attempt %d: asking %s on %s\n", n, model, o.Role.Endpoint.Label)

	// A reply that does not parse is retried once, then counts as an agent error.
	var files []agent.File
	var parseErr error
	for try := 0; try < 2; try++ {
		reply, err := o.Caller.Call(ctx, req)
		a.Calls++
		a.TokensIn += reply.TokensIn
		a.TokensOut += reply.TokensOut
		a.Seconds += reply.Duration.Seconds()
		if errors.Is(err, agent.ErrStale) {
			return a, "", "", err
		}
		if err != nil {
			parseErr = fmt.Errorf("%w: %v", agent.ErrAgent, err)
			continue
		}
		if o.Stream != nil {
			fmt.Fprintln(o.Stream)
		}
		if files, parseErr = agent.ParseFiles(reply.Content); parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		a.Outcome, a.Reason = AgentError, parseErr.Error()
		return a, "", agentFeedback(n, parseErr), nil
	}
	if err := agent.WriteFiles(wt, files); err != nil {
		return a, "", "", err
	}
	wg := gate.Git{Dir: wt}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	if _, err := wg.Run(append([]string{"add", "--"}, paths...)...); err != nil {
		return a, "", "", err
	}
	if out, _ := wg.Run("diff", "--cached", "--name-only"); strings.TrimSpace(out) == "" {
		a.Outcome, a.Reason = AgentError, "the reply changed nothing"
		return a, "", agentFeedback(n, errors.New("your files were identical to the current ones; nothing changed")), nil
	}
	stat, _ := wg.Run("diff", "--cached", "--stat")
	fmt.Fprint(o.Out, stat)

	msg := commitMessage(o, model, promptSHA, loopID, n)
	if _, err := wg.Run("-c", "user.name="+o.RequestedBy, "-c", "user.email="+o.AuthorEmail, "commit", "-q", "-m", msg); err != nil {
		return a, "", "", err
	}
	head, err := wg.RevParse("HEAD")
	if err != nil {
		return a, "", "", err
	}
	a.Branch, a.Head = branch, head

	run, err := gate.Run(gate.RunOptions{
		RepoDir: o.RepoDir, Base: base, Head: head, SpecPath: o.SpecPath, Attempt: n,
		RequestedBy: o.RequestedBy, Source: o.Source, Now: o.Now,
		Agents: []gate.AgentRecord{{Role: o.Role.Name, Kind: o.Role.Kind, Model: model, Endpoint: o.Role.Endpoint.Label,
			PromptSHA256: promptSHA, TokensIn: a.TokensIn, TokensOut: a.TokensOut, Seconds: a.Seconds, Source: o.Source}},
	})
	if err != nil {
		return a, "", "", err
	}
	a.RunID, a.Outcome, a.Reason = run.Evidence.RunID, run.Evidence.Verdict, run.Evidence.Reason
	report := ""
	if a.Outcome == gate.Fail {
		data, err := os.ReadFile(filepath.Join(run.Dir, "failure_report.md"))
		if err != nil {
			return a, "", "", err
		}
		report = string(data)
	}
	return a, head, report, nil
}

func agentFeedback(n int, err error) string {
	return fmt.Sprintf("# Failure report (attempt %d)\n\nYour reply could not be used: %v\nReply with complete files in the === FILE: <path> === format.\n", n, err)
}

func commitMessage(o Options, model, promptSHA, loopID string, n int) string {
	kind := o.Role.Kind
	if kind == "" {
		kind = "completion"
	}
	return fmt.Sprintf("%s: attempt %d for %s\n\nAgent: %s/%s\nModel: %s\nEndpoint: %s\nPrompt-SHA256: %s\nRun-ID: %s/attempt-%d\nRequested-By: %s\n",
		loopID, n, o.SpecPath, o.Role.Name, kind, model, o.Role.Endpoint.Label, promptSHA, loopID, n, o.RequestedBy)
}

// loadFromBase reads gate.yaml, the spec and the implementer prompt from base.
func loadFromBase(g gate.Git, base, specPath string) (*gate.Config, *gate.Spec, string, error) {
	raw, err := g.Show(base, "gate.yaml")
	if err != nil {
		return nil, nil, "", fmt.Errorf("gate.yaml at base: %w", err)
	}
	cfg, err := gate.ParseConfig(raw)
	if err != nil {
		return nil, nil, "", err
	}
	data, err := g.Show(base, specPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("spec at base: %w", err)
	}
	spec, err := gate.ParseSpec(specPath, data)
	if err != nil {
		return nil, nil, "", err
	}
	prompt, err := g.Show(base, "prompts/implementer.md")
	if err != nil {
		return nil, nil, "", fmt.Errorf("prompts/implementer.md at base: %w", err)
	}
	return cfg, spec, string(prompt), nil
}

func nextLoopID(repo string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(gate.FactoryDir(repo), "loops"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	high := 0
	for _, e := range entries {
		var k int
		if _, err := fmt.Sscanf(e.Name(), "l-%04d", &k); err == nil && k > high {
			high = k
		}
	}
	id := fmt.Sprintf("l-%04d", high+1)
	return id, os.MkdirAll(filepath.Join(gate.FactoryDir(repo), "loops", id), 0o755)
}

func writeSummary(repo string, res *Result) error {
	return WriteSummaryTo(filepath.Join(gate.FactoryDir(repo), "loops", res.LoopID), res)
}

// WriteSummaryTo writes summary.json into dir, for example next to a recording.
func WriteSummaryTo(dir string, res *Result) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.json"), append(data, '\n'), 0o644)
}
