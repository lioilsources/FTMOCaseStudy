package loop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lioilsources/factory-gate/internal/agent"
	"github.com/lioilsources/factory-gate/internal/gate"
)

const (
	buggyLib = "package lib\n\n// Add adds.\nfunc Add(a, b int64) int64 { return a - b }\n"
	fixedLib = "package lib\n\n// Add adds.\nfunc Add(a, b int64) int64 { return b + a }\n"
)

func reply(content string) string {
	return "=== FILE: lib/lib.go ===\n" + content + "=== END FILE ===\n"
}

// fixtureRepo creates a temporary repository: a library, a property test standing in for
// the invariants, a spec with an approved plan, a prompt and a gate.yaml.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sums, err := os.ReadFile("../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	var rapidSums []string
	for _, l := range strings.Split(string(sums), "\n") {
		if strings.HasPrefix(l, "pgregory.net/rapid ") {
			rapidSums = append(rapidSums, l)
		}
	}
	files := map[string]string{
		"go.mod": "module example.com/fixture\n\ngo 1.25\n\nrequire pgregory.net/rapid v1.3.0\n",
		"go.sum": strings.Join(rapidSums, "\n") + "\n",
		"gate.yaml": `version: 1
protected_paths: [inv/**, gate.yaml, prompts/**]
tiers:
  - {tier: 2, paths: [lib/**]}
default_tier: 1
budget: {max_changed_lines: 50, max_files: 4}
max_attempts: 3
checks:
  build: {required: true, timeout_seconds: 120}
  invariants: {required: true, package: ./inv/, checks: 20, seed: 1, timeout_seconds: 60}
  budget: {required: true}
endpoints:
  fake: {base_url_env: FIXTURE_URL, headers_env: FIXTURE_HEADERS}
roles:
  implementer: {kind: completion, endpoint: fake, model: fake-coder, context: spec-only, timeout_seconds: 10}
`,
		"specs/add.md":           "---\nid: add\ntier: 2\nallowed_paths: [lib/*.go]\nplan_approved_by: \"reviewer\"\n---\n\nMake Add add.\n",
		"prompts/implementer.md": "You are the implementer. Reply with === FILE: <path> === blocks.\n",
		"lib/lib.go":             "package lib\n\n// Add adds.\nfunc Add(a, b int64) int64 { return 0 }\n",
		"inv/inv_test.go": `package inv

import (
	"testing"

	"example.com/fixture/lib"
	"pgregory.net/rapid"
)

func TestAdd(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.Int64Range(0, 100).Draw(t, "a")
		b := rapid.Int64Range(0, 100).Draw(t, "b")
		if lib.Add(a, b) != a+b {
			t.Fatalf("Add(%d, %d) = %d", a, b, lib.Add(a, b))
		}
	})
}
`,
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func gitEnv() []string {
	return append(os.Environ(), "GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=tester@example.invalid",
		"GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=tester@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null")
}

// scriptedServer replies with the given contents in order.
func scriptedServer(t *testing.T, contents ...string) *httptest.Server {
	t.Helper()
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if i >= len(contents) {
			http.Error(w, "no more scripted replies", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "fake-coder",
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": contents[i]}}},
			"usage":   map[string]int{"prompt_tokens": 100 + i, "completion_tokens": 10 + i},
		})
		i++
	}))
	t.Cleanup(srv.Close)
	return srv
}

func options(t *testing.T, dir string, caller agent.Caller, source string) Options {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "gate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := gate.ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	role, err := ResolveRole(cfg, "implementer", false, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	return Options{RepoDir: dir, Base: "main", SpecPath: "specs/add.md", Role: role, Caller: caller, Source: source,
		RequestedBy: "tester", AuthorEmail: "tester@example.invalid",
		Now: func() time.Time { return time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC) }}
}

func outcomes(r *Result) string {
	var s []string
	for _, a := range r.Attempts {
		s = append(s, a.Outcome)
	}
	return strings.Join(s, ",")
}

func TestLoopFailThenPassAndReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the gate in a temporary repository")
	}
	dir := fixtureRepo(t)
	srv := scriptedServer(t, reply(buggyLib), reply(fixedLib))
	recDir := filepath.Join(t.TempDir(), "recording")
	live := agent.Completion{Endpoint: agent.Endpoint{Label: "fake", BaseURL: srv.URL}}
	rec := &agent.Recorder{Next: live, Dir: recDir, Endpoint: "fake"}

	res, err := Run(context.Background(), options(t, dir, rec, "live"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomes(res); got != "FAIL,PASS" {
		t.Fatalf("live outcomes = %s, want FAIL,PASS", got)
	}

	// The second prompt carried the gate's failure report, with the counterexample.
	data, err := os.ReadFile(filepath.Join(recDir, "call-002.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "The gate rejected your previous attempt") || !strings.Contains(string(data), "Add(") {
		t.Fatal("attempt 2 prompt lacks the failure report")
	}

	// The commit names the agent in trailers and keeps the human as author.
	g := gate.Git{Dir: dir}
	author, _ := g.Run("log", "-1", "--format=%an", res.Attempts[1].Head)
	trailers, _ := g.Trailers(res.Attempts[1].Head)
	if strings.TrimSpace(author) != "tester" || trailers["Agent"] != "implementer/completion" || trailers["Model"] != "fake-coder" {
		t.Fatalf("author %q, trailers %v", author, trailers)
	}

	// Replaying the recording gives the same verdict sequence, with the gate running live.
	rep, err := agent.LoadReplayer(recDir)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := Run(context.Background(), options(t, dir, rep, "replayed"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomes(res2); got != "FAIL,PASS" {
		t.Fatalf("replayed outcomes = %s, want FAIL,PASS", got)
	}
	if rep.Remaining() != 0 {
		t.Fatalf("%d recorded calls unused", rep.Remaining())
	}
}

func TestLoopAgentErrorIsRetriedOnceAndCountedApart(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the gate in a temporary repository")
	}
	dir := fixtureRepo(t)
	srv := scriptedServer(t, "I fixed it.", "Still prose.", reply(fixedLib))
	caller := agent.Completion{Endpoint: agent.Endpoint{Label: "fake", BaseURL: srv.URL}}
	res, err := Run(context.Background(), options(t, dir, caller, "live"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomes(res); got != AgentError+",PASS" {
		t.Fatalf("outcomes = %s", got)
	}
	if res.Attempts[0].Calls != 2 || res.Attempts[0].RunID != "" {
		t.Fatalf("attempt 1 = %+v, want two calls and no gate run", res.Attempts[0])
	}
}

func TestLoopProtectedPathEscalates(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the gate in a temporary repository")
	}
	dir := fixtureRepo(t)
	weakened := "=== FILE: inv/inv_test.go ===\npackage inv\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n=== END FILE ===\n"
	srv := scriptedServer(t, reply(buggyLib)+weakened)
	caller := agent.Completion{Endpoint: agent.Endpoint{Label: "fake", BaseURL: srv.URL}}
	res, err := Run(context.Background(), options(t, dir, caller, "live"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomes(res); got != "ESCALATE" {
		t.Fatalf("outcomes = %s, want ESCALATE", got)
	}
}
