package main

import (
	"flag"
	"fmt"

	"github.com/lioilsources/factory-gate/internal/gate"
)

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	base := fs.String("base", "main", "base ref: the rules come from here")
	head := fs.String("head", "HEAD", "head ref: the change under test")
	spec := fs.String("spec", "", "spec file, read from base")
	mode := fs.String("mode", "blocking", "blocking or advisory")
	attempt := fs.Int("attempt", 1, "attempt number within a loop")
	requestedBy := fs.String("requested-by", "", "requester (default: git config user.name)")
	_ = fs.Parse(args)
	if *mode != "blocking" && *mode != "advisory" {
		return fail(fmt.Errorf("--mode must be blocking or advisory"))
	}
	repo, err := repoRoot()
	if err != nil {
		return fail(err)
	}
	res, err := gate.Run(gate.RunOptions{
		RepoDir: repo, Base: *base, Head: *head, SpecPath: *spec, Mode: *mode,
		Attempt: *attempt, RequestedBy: requester(*requestedBy),
	})
	if err != nil {
		return fail(err)
	}
	ev := res.Evidence
	fmt.Printf("%s %s: %s\n", ev.RunID, ev.Verdict, ev.Reason)
	for _, c := range ev.Checks {
		fmt.Printf("  %-10s %-12s %s\n", c.ID, c.Status, c.Key)
	}
	fmt.Printf("evidence: %s/evidence.md\n", res.Dir)
	if *mode == "advisory" {
		return 0
	}
	return gate.ExitCode(ev.Verdict)
}
