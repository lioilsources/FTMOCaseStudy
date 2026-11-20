package gate

import (
	"os"
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"gate.yaml", "gate.yaml", true},
		{"gate.yaml", "sub/gate.yaml", false},
		{"sample/orders/invariants/**", "sample/orders/invariants/harness_test.go", true},
		{"sample/orders/invariants/**", "sample/orders/invariants/deep/x.go", true},
		{"sample/orders/invariants/**", "sample/orders/engine.go", false},
		{"sample/orders/invariants/**", "sample/orders/invariants", true},
		{"sample/orders/*.go", "sample/orders/engine.go", true},
		{"sample/orders/*.go", "sample/orders/invariants/harness_test.go", false},
		{"*.md", "README.md", true},
		{"*.md", "docs/a.md", false},
		{"docs/**", "docs/sample-evidence/evidence.md", true},
		{".claude/**", ".claude/settings.json", true},
		{"internal/**", "internalx/a.go", false},
		{"**/secret.txt", "a/b/secret.txt", true},
		{"**/secret.txt", "secret.txt", true},
		{"[", "[", false}, // malformed pattern matches nothing
	}
	for _, tt := range tests {
		if got := Match(tt.pattern, tt.name); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestRepoConfig(t *testing.T) {
	data, err := os.ReadFile("../../gate.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	tiers := map[string]int{
		"sample/orders/engine.go":                     2,
		"sample/orders/README.md":                     2,
		"README.md":                                   0,
		"docs/sample-evidence/evidence.md":            0,
		"specs/partial-fills.md":                      1,
		"internal/gate/run.go":                        1,
		"sample/orders/invariants/invariants_test.go": 2,
	}
	for p, want := range tiers {
		if got := cfg.TierOf(p); got != want {
			t.Errorf("TierOf(%s) = %d, want %d", p, got, want)
		}
	}
	for p, want := range map[string]bool{
		"sample/orders/invariants/invariants_test.go": true,
		"gate.yaml":               true,
		"prompts/implementer.md":  true,
		"internal/gate/run.go":    true,
		"sample/orders/engine.go": false,
		"README.md":               false,
		"specs/partial-fills.md":  false,
	} {
		if got := cfg.IsProtected(p); got != want {
			t.Errorf("IsProtected(%s) = %v, want %v", p, got, want)
		}
	}
}

func TestParseConfigRejectsUnknownFields(t *testing.T) {
	if _, err := ParseConfig([]byte("max_attempts: 3\nbudget: {max_changed_lines: 1, max_files: 1}\nsurprise: true\n")); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestParseSpec(t *testing.T) {
	data, err := os.ReadFile("../../specs/partial-fills.md")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseSpec("specs/partial-fills.md", data)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "partial-fills" || s.Tier != 2 || len(s.AllowedPaths) != 1 || len(s.SHA256) != 64 {
		t.Fatalf("spec = %+v", s)
	}
	if _, err := ParseSpec("x.md", []byte("no front matter")); err == nil {
		t.Fatal("spec without front matter accepted")
	}
}

func TestRunBudget(t *testing.T) {
	cfg := &Config{Budget: Budget{MaxChangedLines: 10, MaxFiles: 2}, Checks: Checks{Budget: &CheckConfig{Required: true}}}
	spec := &Spec{AllowedPaths: []string{"sample/orders/*.go"}}
	tests := []struct {
		name    string
		changes []FileChange
		want    string
	}{
		{"within", []FileChange{{Path: "sample/orders/engine.go", Added: 5, Deleted: 5}}, StatusPass},
		{"test lines do not count", []FileChange{{Path: "sample/orders/engine_test.go", Added: 500}}, StatusPass},
		{"too many lines", []FileChange{{Path: "sample/orders/engine.go", Added: 11}}, StatusFail},
		{"too many files", []FileChange{{Path: "sample/orders/a.go"}, {Path: "sample/orders/b.go"}, {Path: "sample/orders/c.go"}}, StatusFail},
		{"out of scope", []FileChange{{Path: "README.md", Added: 1}}, StatusFail},
	}
	for _, tt := range tests {
		if got := runBudget(cfg, spec, tt.changes); got.Status != tt.want {
			t.Errorf("%s: status %s (%s), want %s", tt.name, got.Status, got.Detail, tt.want)
		}
	}
}

func TestReadFirst(t *testing.T) {
	cfg := &Config{Tiers: []TierRule{{Tier: 2, Paths: []string{"sample/orders/**"}}}, DefaultTier: 1}
	hunks := []Hunk{
		{Path: "README.md", Start: 1, Lines: 50},
		{Path: "sample/orders/engine.go", Start: 10, Lines: 3},
		{Path: "sample/orders/engine.go", Start: 80, Lines: 7},
		{Path: "sample/orders/orders.go", Start: 5, Lines: 1},
		{Path: "sample/orders/orders.go", Start: 9, Lines: 2},
	}
	got := ReadFirst(cfg, hunks)
	if len(got) != 3 {
		t.Fatalf("got %d locations, want 3", len(got))
	}
	if got[0].Start != 80 || got[1].Start != 10 || got[2].Start != 9 {
		t.Fatalf("order = %+v", got)
	}
	got = ReadFirst(cfg, hunks[:2])
	if got[0].Path != "sample/orders/engine.go" || got[1].Path != "README.md" {
		t.Fatalf("largest hunk should follow tier-2 hunks: %+v", got)
	}
}
