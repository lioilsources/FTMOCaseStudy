// Package agent defines the role contract and the adapters that call models for a role.
//
// The contract: given a role, a prompt and a working directory, an adapter leaves the
// proposed files in that directory and reports model id, token counts and duration.
// The model never acts by itself; every action is taken by the gate.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Endpoint is a resolved model endpoint. Headers are never logged, recorded or printed.
type Endpoint struct {
	Label   string
	BaseURL string
	Headers http.Header
}

// RoleConfig binds a role to a model on an endpoint.
type RoleConfig struct {
	Name     string // implementer, test_author
	Kind     string // completion or command
	Model    string
	Ladder   []string // optional: model id per attempt
	Context  string   // spec-only or spec+invariants
	Timeout  time.Duration
	Endpoint Endpoint
}

// ModelFor returns the model of the given attempt: the ladder entry when set, else Model.
func (r RoleConfig) ModelFor(attempt int) string {
	if attempt >= 1 && attempt <= len(r.Ladder) {
		return r.Ladder[attempt-1]
	}
	if len(r.Ladder) > 0 {
		return r.Ladder[len(r.Ladder)-1]
	}
	return r.Model
}

// ResolveEndpoint reads an endpoint from the environment variables named in gate.yaml.
func ResolveEndpoint(label, urlEnv, headersEnv string, getenv func(string) string) (Endpoint, error) {
	base := strings.TrimRight(strings.TrimSpace(getenv(urlEnv)), "/")
	if base == "" {
		return Endpoint{}, fmt.Errorf("endpoint %s: %s is not set", label, urlEnv)
	}
	h, err := ParseHeaders(getenv(headersEnv))
	if err != nil {
		return Endpoint{}, fmt.Errorf("endpoint %s: %s: %w", label, headersEnv, err)
	}
	return Endpoint{Label: label, BaseURL: base, Headers: h}, nil
}

// ParseHeaders parses "Name: value" pairs separated by newlines or a literal "\n".
// The error never contains a value.
func ParseHeaders(s string) (http.Header, error) {
	h := http.Header{}
	s = strings.ReplaceAll(s, `\n`, "\n")
	for i, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("header %d is not of the form Name: value", i+1)
		}
		h.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	return h, nil
}

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is one model call.
type Request struct {
	Role     string
	Model    string
	Messages []Message
	Timeout  time.Duration
	Stream   io.Writer // when set, reply text is written here as it arrives
}

// PromptHash is the sha256 of the messages of a request. It identifies a call in a recording.
func (r Request) PromptHash() string {
	data, _ := json.Marshal(r.Messages)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Reply is what a model returned, with its cost.
type Reply struct {
	Model     string        `json:"model"`
	Content   string        `json:"content"`
	TokensIn  int           `json:"tokens_in"`
	TokensOut int           `json:"tokens_out"`
	Duration  time.Duration `json:"duration_ns"`
	Source    string        `json:"source"` // live or replayed
}

// Caller is the model-call boundary. Record and replay wrap it.
type Caller interface {
	Call(ctx context.Context, req Request) (Reply, error)
}

// ErrAgent marks a reply that could not be used: kept apart from gate FAIL in statistics.
var ErrAgent = errors.New("agent_error")
