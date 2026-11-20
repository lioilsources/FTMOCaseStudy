package loop

import (
	"fmt"
	"time"

	"github.com/lioilsources/factory-gate/internal/agent"
	"github.com/lioilsources/factory-gate/internal/gate"
)

// ResolveRole builds a role from gate.yaml. With live set, the endpoint URL and headers are
// read from the environment; without it (replay) only the endpoint label is needed.
func ResolveRole(cfg *gate.Config, name string, live bool, getenv func(string) string) (agent.RoleConfig, error) {
	r, ok := cfg.Roles[name]
	if !ok {
		return agent.RoleConfig{}, fmt.Errorf("gate.yaml has no role %q", name)
	}
	timeout := time.Duration(r.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	rc := agent.RoleConfig{Name: name, Kind: r.Kind, Model: r.Model, Ladder: r.Ladder, Context: r.Context,
		Timeout: timeout, Endpoint: agent.Endpoint{Label: r.Endpoint}}
	if rc.Kind == "" {
		rc.Kind = "completion"
	}
	if !live {
		return rc, nil
	}
	if rc.Kind != "completion" {
		return rc, fmt.Errorf("role %s: kind %q is not built (only completion)", name, rc.Kind)
	}
	if rc.Model == "" && len(rc.Ladder) == 0 {
		return rc, fmt.Errorf("role %s: no model set in gate.yaml", name)
	}
	ep, ok := cfg.Endpoints[r.Endpoint]
	if !ok {
		return rc, fmt.Errorf("role %s: unknown endpoint %q", name, r.Endpoint)
	}
	resolved, err := agent.ResolveEndpoint(r.Endpoint, ep.BaseURLEnv, ep.HeadersEnv, getenv)
	if err != nil {
		return rc, err
	}
	rc.Endpoint = resolved
	return rc, nil
}
