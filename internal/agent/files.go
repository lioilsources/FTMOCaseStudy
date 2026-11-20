package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// File is one proposed file, with a path relative to the working directory.
type File struct {
	Path    string
	Content string
}

var (
	reasoning  = regexp.MustCompile(`(?s)<(think|thinking|reasoning)>.*?</(think|thinking|reasoning)>`)
	fileHeader = regexp.MustCompile(`(?m)^=== FILE: (.+?) ===[ \t]*$`)
	fileEnd    = "=== END FILE ==="
)

// StripReasoning removes reasoning blocks, including a block whose opening tag the
// server already swallowed (text up to a lone closing tag).
func StripReasoning(s string) string {
	s = reasoning.ReplaceAllString(s, "")
	for _, tag := range []string{"</think>", "</thinking>", "</reasoning>"} {
		if i := strings.LastIndex(s, tag); i >= 0 {
			s = s[i+len(tag):]
		}
	}
	return s
}

// ParseFiles reads the delimiter protocol:
//
//	=== FILE: <relative path> ===
//	<full content>
//	=== END FILE ===
//
// A reply without a well-formed block, or with an unsafe path, is an agent error.
func ParseFiles(reply string) ([]File, error) {
	s := StripReasoning(reply)
	locs := fileHeader.FindAllStringSubmatchIndex(s, -1)
	if len(locs) == 0 {
		return nil, fmt.Errorf("%w: no === FILE: <path> === block in the reply", ErrAgent)
	}
	var files []File
	seen := map[string]bool{}
	for _, loc := range locs {
		p := strings.TrimSpace(s[loc[2]:loc[3]])
		rest := s[loc[1]:]
		end := strings.Index(rest, fileEnd)
		if end < 0 {
			return nil, fmt.Errorf("%w: file %s has no %s line", ErrAgent, p, fileEnd)
		}
		clean, err := SafePath(p)
		if err != nil {
			return nil, err
		}
		if seen[clean] {
			return nil, fmt.Errorf("%w: file %s appears twice", ErrAgent, clean)
		}
		seen[clean] = true
		files = append(files, File{Path: clean, Content: unfence(rest[:end])})
	}
	return files, nil
}

// SafePath rejects absolute paths and paths that leave the working directory.
func SafePath(p string) (string, error) {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || filepath.IsAbs(p) || strings.Contains(p, ":") {
		return "", fmt.Errorf("%w: unsafe path %q", ErrAgent, p)
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, ".git/") || clean == ".git" {
		return "", fmt.Errorf("%w: unsafe path %q", ErrAgent, p)
	}
	return clean, nil
}

// unfence trims the blank lines around a file and a Markdown code fence a model may add.
func unfence(s string) string {
	s = strings.Trim(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[1 : len(lines)-1]
	}
	return strings.Join(lines, "\n") + "\n"
}

// WriteFiles writes the files under dir and runs gofmt on the Go ones. A gofmt failure is
// not an error here: the build check reports broken code.
func WriteFiles(dir string, files []File) error {
	var goFiles []string
	for _, f := range files {
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(f.Content), 0o644); err != nil {
			return err
		}
		if strings.HasSuffix(f.Path, ".go") {
			goFiles = append(goFiles, dst)
		}
	}
	if len(goFiles) > 0 {
		_ = exec.Command("gofmt", append([]string{"-w"}, goFiles...)...).Run()
	}
	return nil
}
