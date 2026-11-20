package gate

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixture is a tiny module in a temporary repository with its own gate.yaml,
// a library under lib/ and a human-owned test under inv/ standing in for the invariants.
var fixture = map[string]string{
	"go.mod": "module example.com/fixture\n\ngo 1.25\n\nrequire pgregory.net/rapid v1.3.0\n",
	"gate.yaml": `version: 1
protected_paths: [inv/**, gate.yaml]
tiers:
  - {tier: 2, paths: [lib/**]}
  - {tier: 0, paths: ["*.md"]}
default_tier: 1
budget: {max_changed_lines: 50, max_files: 4}
max_attempts: 3
checks:
  build: {required: true, timeout_seconds: 120}
  invariants: {required: true, package: ./inv/, checks: 10, seed: 1, timeout_seconds: 60}
  budget: {required: true}
`,
	"spec.md":    "---\nid: fixture\ntier: 2\nallowed_paths: [lib/*.go]\nplan_approved_by: \"reviewer\"\n---\n\nChange lib.\n",
	"README.md":  "# fixture\n",
	"lib/lib.go": "package lib\n\n// Add adds.\nfunc Add(a, b int64) int64 { return a + b }\n",
	"inv/inv_test.go": `package inv

import (
	"testing"

	"example.com/fixture/lib"
	"pgregory.net/rapid"
)

func TestAdd(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.Int64Range(0, 100).Draw(t, "a")
		if lib.Add(a, 1) != a+1 {
			t.Fatal("add broken")
		}
	})
}
`,
}

// rapidSums returns this module's go.sum lines for rapid, so the fixture builds offline.
func rapidSums(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "pgregory.net/rapid ") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n") + "\n"
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=tester@example.invalid",
		"GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=tester@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// branch commits files on a new branch from main and returns to main.
func branch(t *testing.T, dir, name string, files map[string]string) {
	t.Helper()
	git(t, dir, "checkout", "-q", "-b", name, "main")
	write(t, dir, files)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", name)
	git(t, dir, "checkout", "-q", "main")
}

func TestRunEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go build in a temporary repository")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	write(t, dir, fixture)
	write(t, dir, map[string]string{"go.sum": rapidSums(t)})
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "base")

	broken := "package lib\n\n// Add adds.\nfunc Add(a, b int64) int64 { return a - b }\n"
	branch(t, dir, "docs", map[string]string{"README.md": "# fixture\n\nMore docs.\n"})
	branch(t, dir, "bug", map[string]string{"lib/lib.go": broken})
	// The bug plus a weakened test: the change tries to grade itself.
	branch(t, dir, "self-graded", map[string]string{"lib/lib.go": broken,
		"inv/inv_test.go": "package inv\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n"})

	now := func() time.Time { return time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC) }
	tests := []struct {
		head, spec      string
		attempt         int
		want            string
		wantInvariants  string
		wantInEvidence  string
		wantFailureFile bool
	}{
		{"docs", "", 1, Pass, StatusPass, "Every required check passed", false},
		{"bug", "spec.md", 1, Fail, StatusFail, "add broken", true},
		{"bug", "spec.md", 3, Escalate, StatusFail, "attempt 3 of 3", false},
		{"self-graded", "spec.md", 1, Escalate, StatusFail, "inv/inv_test.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.head, func(t *testing.T) {
			res, err := Run(RunOptions{RepoDir: dir, Base: "main", Head: tt.head, SpecPath: tt.spec,
				Attempt: tt.attempt, RequestedBy: "tester", Now: now})
			if err != nil {
				t.Fatal(err)
			}
			ev := res.Evidence
			if ev.Verdict != tt.want {
				t.Fatalf("verdict %s (%s), want %s", ev.Verdict, ev.Reason, tt.want)
			}
			// The original invariants ran, whatever the change did to them.
			for _, c := range ev.Checks {
				if c.ID == "invariants" && c.Status != tt.wantInvariants {
					t.Errorf("invariants %s, want %s", c.Status, tt.wantInvariants)
				}
			}
			md, err := os.ReadFile(filepath.Join(res.Dir, "evidence.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(md), tt.wantInEvidence) {
				t.Errorf("evidence.md lacks %q", tt.wantInEvidence)
			}
			_, err = os.Stat(filepath.Join(res.Dir, "failure_report.md"))
			if got := err == nil; got != tt.wantFailureFile {
				t.Errorf("failure_report.md present = %v, want %v", got, tt.wantFailureFile)
			}
		})
	}

	f, err := os.Open(filepath.Join(dir, ".factory", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines := 0
	for s := bufio.NewScanner(f); s.Scan(); lines++ {
	}
	if lines != len(tests) {
		t.Errorf("audit.jsonl has %d lines, want %d", lines, len(tests))
	}
	if out := git(t, dir, "worktree", "list"); strings.Count(out, "\n") != 0 {
		t.Errorf("worktrees left behind:\n%s", out)
	}
}
