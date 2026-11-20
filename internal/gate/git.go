package gate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Git runs git commands in one repository.
type Git struct{ Dir string }

// Run runs git with args and returns stdout.
func (g Git) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// RevParse resolves a ref to a full SHA.
func (g Git) RevParse(ref string) (string, error) {
	out, err := g.Run("rev-parse", "--verify", ref+"^{commit}")
	return strings.TrimSpace(out), err
}

// Show returns the content of path at rev.
func (g Git) Show(rev, path string) ([]byte, error) {
	out, err := g.Run("show", rev+":"+path)
	return []byte(out), err
}

// ListFiles lists every file in the tree of rev.
func (g Git) ListFiles(rev string) ([]string, error) {
	out, err := g.Run("ls-tree", "-r", "--name-only", rev)
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// FileChange is one changed file between base and head.
type FileChange struct {
	Path    string `json:"path"`
	Status  string `json:"status"` // A, M, D, R...
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary,omitempty"`
}

// IsTest reports whether the file is a Go test file.
func (f FileChange) IsTest() bool { return strings.HasSuffix(f.Path, "_test.go") }

// Diff returns the changed files between base and head, from --numstat and --name-status.
// Renames are reported as a delete plus an add, so both paths are judged.
func (g Git) Diff(base, head string) ([]FileChange, error) {
	num, err := g.Run("diff", "--no-renames", "--numstat", base, head)
	if err != nil {
		return nil, err
	}
	stat, err := g.Run("diff", "--no-renames", "--name-status", base, head)
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(stat), "\n") {
		if f := strings.Split(line, "\t"); len(f) == 2 {
			status[f[1]] = f[0]
		}
	}
	var out []FileChange
	for _, line := range strings.Split(strings.TrimSpace(num), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			continue
		}
		c := FileChange{Path: f[2], Status: status[f[2]]}
		if f[0] == "-" {
			c.Binary = true
		} else {
			c.Added, _ = strconv.Atoi(f[0])
			c.Deleted, _ = strconv.Atoi(f[1])
		}
		out = append(out, c)
	}
	return out, nil
}

// Hunk is one hunk of a unified diff, located in the head version of a file.
type Hunk struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	Lines int    `json:"lines"` // added plus deleted lines
}

// Hunks parses `git diff -U0` into hunks.
func (g Git) Hunks(base, head string) ([]Hunk, error) {
	out, err := g.Run("diff", "--no-renames", "-U0", base, head)
	if err != nil {
		return nil, err
	}
	var hunks []Hunk
	var file string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "@@ "):
			// @@ -a,b +c,d @@
			f := strings.Fields(line)
			if len(f) < 3 {
				continue
			}
			_, oldN := parseRange(f[1])
			start, newN := parseRange(f[2])
			hunks = append(hunks, Hunk{Path: file, Start: start, Lines: oldN + newN})
		}
	}
	return hunks, nil
}

func parseRange(s string) (start, n int) {
	s = strings.TrimLeft(s, "+-")
	a, b, found := strings.Cut(s, ",")
	start, _ = strconv.Atoi(a)
	n = 1
	if found {
		n, _ = strconv.Atoi(b)
	}
	return start, n
}

// Trailers returns the trailers of the commit message at rev.
func (g Git) Trailers(rev string) (map[string]string, error) {
	msg, err := g.Run("log", "-1", "--format=%B", rev)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "interpret-trailers", "--parse")
	cmd.Dir = g.Dir
	cmd.Stdin = strings.NewReader(msg)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git interpret-trailers: %w", err)
	}
	t := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(line, ": "); ok {
			t[k] = v
		}
	}
	return t, nil
}

// AddWorktree checks rev out, detached, into dir.
func (g Git) AddWorktree(dir, rev string) error {
	_, err := g.Run("worktree", "add", "--detach", "--force", dir, rev)
	return err
}

// RemoveWorktree removes a worktree created by AddWorktree.
func (g Git) RemoveWorktree(dir string) {
	if _, err := g.Run("worktree", "remove", "--force", dir); err != nil {
		_ = os.RemoveAll(dir)
		_, _ = g.Run("worktree", "prune")
	}
}

// OverlayProtected makes every protected path in the worktree equal to base: files from
// base are written over head's, and protected files that exist only in head are removed.
func (g Git) OverlayProtected(cfg *Config, base, head, dir string) error {
	baseFiles, err := g.ListFiles(base)
	if err != nil {
		return err
	}
	headFiles, err := g.ListFiles(head)
	if err != nil {
		return err
	}
	inBase := map[string]bool{}
	for _, f := range baseFiles {
		if !cfg.IsProtected(f) {
			continue
		}
		inBase[f] = true
		data, err := g.Show(base, f)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	for _, f := range headFiles {
		if cfg.IsProtected(f) && !inBase[f] {
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(f))); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}
