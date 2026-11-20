// Command gate judges a proposed change against rules taken from the base revision.
//
//	gate run --base <ref> --head <ref> [--spec <file>] [--mode blocking|advisory]
//	         [--attempt N] [--requested-by <name>]
//
// Exit codes: 0 PASS, 1 FAIL, 2 ESCALATE, 3 internal error. Advisory mode always exits 0.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lioilsources/factory-gate/internal/gate"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "version":
		fmt.Println(gate.Version())
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gate run --base <ref> --head <ref> [--spec <file>] [--mode blocking|advisory] [--attempt N] [--requested-by <name>]")
	os.Exit(3)
}

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
		fmt.Fprintln(os.Stderr, "gate: --mode must be blocking or advisory")
		return 3
	}
	repo, err := gitOut("rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gate:", err)
		return 3
	}
	if *requestedBy == "" {
		*requestedBy, _ = gitOut("config", "user.name")
	}
	res, err := gate.Run(gate.RunOptions{
		RepoDir: repo, Base: *base, Head: *head, SpecPath: *spec, Mode: *mode,
		Attempt: *attempt, RequestedBy: *requestedBy,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "gate:", err)
		return 3
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

func gitOut(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}
