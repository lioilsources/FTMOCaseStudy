package gate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Spec is a task spec: YAML front matter between "---" lines, then Markdown.
type Spec struct {
	Path           string   `yaml:"-"`
	SHA256         string   `yaml:"-"`
	Body           string   `yaml:"-"`
	ID             string   `yaml:"id"`
	Tier           int      `yaml:"tier"`
	AllowedPaths   []string `yaml:"allowed_paths"`
	PlanApprovedBy string   `yaml:"plan_approved_by"`
}

// ParseSpec parses a spec file.
func ParseSpec(specPath string, data []byte) (*Spec, error) {
	const fence = "---\n"
	if !bytes.HasPrefix(data, []byte(fence)) {
		return nil, fmt.Errorf("spec %s: missing front matter", specPath)
	}
	rest := data[len(fence):]
	end := bytes.Index(rest, []byte("\n"+fence))
	if end < 0 {
		return nil, fmt.Errorf("spec %s: unterminated front matter", specPath)
	}
	var s Spec
	if err := yaml.Unmarshal(rest[:end+1], &s); err != nil {
		return nil, fmt.Errorf("spec %s: %w", specPath, err)
	}
	if s.ID == "" || len(s.AllowedPaths) == 0 {
		return nil, fmt.Errorf("spec %s: id and allowed_paths are required", specPath)
	}
	sum := sha256.Sum256(data)
	s.Path, s.SHA256, s.Body = specPath, hex.EncodeToString(sum[:]), string(rest[end+1+len(fence):])
	return &s, nil
}
