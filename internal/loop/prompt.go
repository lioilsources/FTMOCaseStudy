package loop

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lioilsources/factory-gate/internal/agent"
	"github.com/lioilsources/factory-gate/internal/gate"
)

// currentFiles returns the tracked files in the worktree that match the spec's allowed paths.
func currentFiles(wt string, spec *gate.Spec) ([]agent.File, error) {
	out, err := gate.Git{Dir: wt}.Run("ls-files")
	if err != nil {
		return nil, err
	}
	var files []agent.File
	for _, p := range strings.Fields(out) {
		if !gate.MatchAny(spec.AllowedPaths, p) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(wt, filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		files = append(files, agent.File{Path: p, Content: string(data)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// invariantSources returns the invariant test files from base, read-only context for the
// spec+invariants profile.
func invariantSources(g gate.Git, cfg *gate.Config, base string) ([]agent.File, error) {
	if cfg.Checks.Invariants == nil {
		return nil, nil
	}
	dir := strings.TrimPrefix(path.Clean(cfg.Checks.Invariants.Package), "./")
	all, err := g.ListFiles(base)
	if err != nil {
		return nil, err
	}
	var files []agent.File
	for _, p := range all {
		if strings.HasPrefix(p, dir+"/") && strings.HasSuffix(p, ".go") {
			data, err := g.Show(base, p)
			if err != nil {
				return nil, err
			}
			files = append(files, agent.File{Path: p, Content: string(data)})
		}
	}
	return files, nil
}

// userPrompt is the spec, the current files, optionally the invariants, and the last
// failure report. Nothing else is sent.
func userPrompt(spec *gate.Spec, current, invariants []agent.File, feedback string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Task spec: %s\n\nAllowed paths: %s\n\n%s\n", spec.ID, strings.Join(spec.AllowedPaths, ", "), strings.TrimSpace(spec.Body))
	b.WriteString("\n# Current files\n\n")
	for _, f := range current {
		fmt.Fprintf(&b, "=== FILE: %s ===\n%s=== END FILE ===\n\n", f.Path, f.Content)
	}
	if len(invariants) > 0 {
		b.WriteString("# Property tests the gate will run (read only, you cannot change them)\n\n")
		for _, f := range invariants {
			fmt.Fprintf(&b, "--- %s ---\n%s\n", f.Path, f.Content)
		}
	}
	if feedback != "" {
		fmt.Fprintf(&b, "# The gate rejected your previous attempt\n\n%s\n", strings.TrimSpace(feedback))
	}
	b.WriteString("\nReturn the complete files you change, in the === FILE: <path> === format.\n")
	return b.String()
}
