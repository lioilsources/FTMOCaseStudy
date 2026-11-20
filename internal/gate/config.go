// Package gate computes a verdict for a change: diff, worktree, checks, evidence and audit.
//
// The one rule that matters: the gate takes its rules from base, never from the change
// under test. Config, spec and protected paths are read from the base revision.
package gate

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config is gate.yaml.
type Config struct {
	Version        int                 `yaml:"version"`
	ProtectedPaths []string            `yaml:"protected_paths"`
	Tiers          []TierRule          `yaml:"tiers"`
	DefaultTier    int                 `yaml:"default_tier"`
	Budget         Budget              `yaml:"budget"`
	MaxAttempts    int                 `yaml:"max_attempts"`
	Checks         Checks              `yaml:"checks"`
	Endpoints      map[string]Endpoint `yaml:"endpoints"`
	Roles          map[string]Role     `yaml:"roles"`
}

// TierRule assigns a tier to paths.
type TierRule struct {
	Tier  int      `yaml:"tier"`
	Paths []string `yaml:"paths"`
}

// Budget limits the size of a change.
type Budget struct {
	MaxChangedLines int `yaml:"max_changed_lines"`
	MaxFiles        int `yaml:"max_files"`
}

// Checks configures each check. A nil entry means the check does not exist yet.
type Checks struct {
	Build      *CheckConfig `yaml:"build"`
	Invariants *CheckConfig `yaml:"invariants"`
	Budget     *CheckConfig `yaml:"budget"`
}

// CheckConfig is the configuration of one check.
type CheckConfig struct {
	Required       bool   `yaml:"required"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	Package        string `yaml:"package"`
	Checks         int    `yaml:"checks"`
	Seed           uint64 `yaml:"seed"`
}

// Endpoint names the environment variables that hold a model endpoint.
type Endpoint struct {
	BaseURLEnv string `yaml:"base_url_env"`
	HeadersEnv string `yaml:"headers_env"`
}

// Role binds an agent role to a model.
type Role struct {
	Kind           string   `yaml:"kind"`
	Endpoint       string   `yaml:"endpoint"`
	Model          string   `yaml:"model"`
	Ladder         []string `yaml:"ladder"`
	Context        string   `yaml:"context"`
	TimeoutSeconds int      `yaml:"timeout_seconds"`
	Command        []string `yaml:"command"`
}

// ParseConfig parses gate.yaml and rejects unknown fields and missing essentials.
func ParseConfig(data []byte) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("gate.yaml: %w", err)
	}
	if c.MaxAttempts <= 0 {
		return nil, fmt.Errorf("gate.yaml: max_attempts must be positive")
	}
	if c.Budget.MaxChangedLines <= 0 || c.Budget.MaxFiles <= 0 {
		return nil, fmt.Errorf("gate.yaml: budget limits must be positive")
	}
	return &c, nil
}

// TierOf returns the tier of a path: the first matching rule, else the default.
func (c *Config) TierOf(path string) int {
	for _, r := range c.Tiers {
		if MatchAny(r.Paths, path) {
			return r.Tier
		}
	}
	return c.DefaultTier
}

// IsProtected reports whether a path is protected.
func (c *Config) IsProtected(path string) bool { return MatchAny(c.ProtectedPaths, path) }
