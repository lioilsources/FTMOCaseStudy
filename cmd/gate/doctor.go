package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/lioilsources/factory-gate/internal/agent"
	"github.com/lioilsources/factory-gate/internal/gate"
	"github.com/lioilsources/factory-gate/internal/loop"
)

// cmdDoctor checks for each configured role that its endpoint is reachable, that the
// model is listed, and that a one-token request answers. It prints labels, never URLs.
func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	base := fs.String("base", "HEAD", "read gate.yaml from this ref")
	_ = fs.Parse(args)
	repo, err := repoRoot()
	if err != nil {
		return fail(err)
	}
	raw, err := gate.Git{Dir: repo}.Show(*base, "gate.yaml")
	if err != nil {
		return fail(err)
	}
	cfg, err := gate.ParseConfig(raw)
	if err != nil {
		return fail(err)
	}
	names := make([]string, 0, len(cfg.Roles))
	for n := range cfg.Roles {
		names = append(names, n)
	}
	sort.Strings(names)
	bad := 0
	models := map[string]string{}
	for _, name := range names {
		ok := checkRole(cfg, name)
		if !ok {
			bad++
		}
		models[name] = cfg.Roles[name].Model
	}
	if m := models["implementer"]; m != "" && m == models["test_author"] {
		fmt.Println("note: implementer and test_author use the same model: reduced independence")
	}
	if bad > 0 {
		fmt.Printf("%d role(s) not ready; without endpoints use `make demo` (replayed)\n", bad)
		return 1
	}
	return 0
}

func checkRole(cfg *gate.Config, name string) bool {
	role, err := loop.ResolveRole(cfg, name, true, os.Getenv)
	if err != nil {
		fmt.Printf("%-12s FAIL  %v\n", name, err)
		return false
	}
	label := role.Endpoint.Label
	listed, latency, err := listModels(role.Endpoint)
	if err != nil {
		fmt.Printf("%-12s FAIL  endpoint %s unreachable: %v\n", name, label, err)
		return false
	}
	model := role.ModelFor(1)
	note := "model listed"
	if !listed[model] {
		note = "model NOT in /models (the server may still serve it)"
	}
	fmt.Printf("%-12s ok    endpoint %s, /models %d ms, %s: %s\n", name, label, latency.Milliseconds(), note, model)

	r, err := agent.Completion{Endpoint: role.Endpoint}.Call(context.Background(), agent.Request{
		Role: name, Model: model, Timeout: role.Timeout,
		Messages: []agent.Message{{Role: "user", Content: "Reply with the single word: ok"}},
	})
	if err != nil {
		fmt.Printf("%-12s FAIL  chat request: %v\n", name, err)
		return false
	}
	fmt.Printf("%-12s ok    chat answered in %.1f s (%d tokens out)\n", name, r.Duration.Seconds(), r.TokensOut)
	return true
}

func listModels(ep agent.Endpoint) (map[string]bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.BaseURL+"/models", nil)
	if err != nil {
		return nil, 0, fmt.Errorf("bad URL")
	}
	req.Header = ep.Headers.Clone()
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed")
	}
	defer resp.Body.Close()
	latency := time.Since(start)
	if resp.StatusCode != http.StatusOK {
		return nil, latency, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, latency, fmt.Errorf("decode /models")
	}
	out := map[string]bool{}
	for _, m := range body.Data {
		out[m.ID] = true
	}
	return out, latency, nil
}
