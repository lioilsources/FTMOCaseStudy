package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lioilsources/factory-gate/internal/agent"
	"github.com/lioilsources/factory-gate/internal/gate"
	"github.com/lioilsources/factory-gate/internal/loop"
)

func cmdLoop(args []string) int {
	fs := flag.NewFlagSet("loop", flag.ExitOnError)
	spec := fs.String("spec", "", "spec file, read from base (required)")
	base := fs.String("base", "main", "base ref")
	maxAttempts := fs.Int("max-attempts", 0, "attempt limit (default: gate.yaml max_attempts)")
	contextProfile := fs.String("context", "", "spec-only or spec+invariants (default: the role's)")
	record := fs.String("record", "", "record every model call into this directory")
	replay := fs.String("replay", "", "serve model calls from this recording; the gate still runs live")
	stream := fs.Bool("stream", false, "print model output as it arrives")
	requestedBy := fs.String("requested-by", "", "requester (default: git config user.name)")
	_ = fs.Parse(args)
	if *spec == "" {
		return fail(fmt.Errorf("--spec is required"))
	}
	if *record != "" && *replay != "" {
		return fail(fmt.Errorf("--record and --replay exclude each other"))
	}
	repo, err := repoRoot()
	if err != nil {
		return fail(err)
	}
	g := gate.Git{Dir: repo}
	raw, err := g.Show(*base, "gate.yaml")
	if err != nil {
		return fail(err)
	}
	cfg, err := gate.ParseConfig(raw)
	if err != nil {
		return fail(err)
	}
	live := *replay == ""
	role, err := loop.ResolveRole(cfg, "implementer", live, os.Getenv)
	if err != nil {
		return fail(err)
	}

	var caller agent.Caller
	source := "live"
	if live {
		caller = agent.Completion{Endpoint: role.Endpoint}
		if *record != "" {
			caller = &agent.Recorder{Next: caller, Dir: *record, Endpoint: role.Endpoint.Label}
		}
	} else {
		rep, err := agent.LoadReplayer(*replay)
		if err != nil {
			return fail(err)
		}
		caller, source = rep, "replayed"
	}
	var streamTo io.Writer
	if *stream {
		streamTo = os.Stdout
	}
	email, _ := gitOut("config", "user.email")
	res, err := loop.Run(context.Background(), loop.Options{
		RepoDir: repo, Base: *base, SpecPath: *spec, MaxAttempts: *maxAttempts, Context: *contextProfile,
		Role: role, Caller: caller, Source: source, RequestedBy: requester(*requestedBy), AuthorEmail: email,
		Stream: streamTo, Out: os.Stdout,
	})
	if err != nil {
		return fail(err)
	}
	if *record != "" {
		if err := loop.WriteSummaryTo(*record, res); err != nil {
			return fail(err)
		}
	}
	fmt.Printf("loop %s finished: %s\n", res.LoopID, res.Final)
	for _, a := range res.Attempts {
		fmt.Printf("  attempt %d  %-11s %-8s model %s, %d+%d tokens, %.1f s\n", a.N, a.Outcome, a.RunID, a.Model, a.TokensIn, a.TokensOut, a.Seconds)
	}
	if res.Final == gate.Pass {
		return 0
	}
	return 1
}
