package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lioilsources/factory-gate/internal/eval"
	"github.com/lioilsources/factory-gate/internal/gate"
)

// cmdEval runs the static gate evals and writes evals/RESULTS.md. It exits 1 when any case
// ends in an unexpected result; known misses do not fail it.
func cmdEval(args []string) int {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	base := fs.String("base", "main", "base ref every case is applied to")
	casesDir := fs.String("cases", "evals/cases", "directory of cases")
	out := fs.String("out", "evals/RESULTS.md", "results file; empty to skip writing")
	only := fs.String("only", "", "run only the case with this name")
	_ = fs.Parse(args)

	repo, err := repoRoot()
	if err != nil {
		return fail(err)
	}
	cases, err := eval.LoadCases(filepath.Join(repo, *casesDir))
	if err != nil {
		return fail(err)
	}
	if *only != "" {
		var keep []eval.Case
		for _, c := range cases {
			if c.Name == *only {
				keep = append(keep, c)
			}
		}
		if len(keep) == 0 {
			return fail(fmt.Errorf("no case named %q", *only))
		}
		cases = keep
	}

	outcomes := eval.Run(eval.Options{
		RepoDir: repo, Base: *base, Now: time.Now,
		Log: func(f string, a ...any) { fmt.Printf(f, a...) },
	}, cases)

	if *out != "" && *only == "" {
		md := eval.Markdown(outcomes, time.Now().Format("2006-01-02"), gate.Version(), *base)
		if err := os.WriteFile(filepath.Join(repo, *out), []byte(md), 0o644); err != nil {
			return fail(err)
		}
		fmt.Println("wrote", *out)
	}
	code := 0
	for _, o := range outcomes {
		if o.Unexpected() {
			fmt.Fprintf(os.Stderr, "unexpected: %s: %s %s\n", o.Case.Name, o.Result, o.ErrorMsg)
			code = 1
		}
	}
	return code
}
