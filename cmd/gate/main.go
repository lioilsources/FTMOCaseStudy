// Command gate judges a proposed change against rules taken from the base revision.
//
//	gate run    --base <ref> --head <ref> [--spec <file>] [--mode blocking|advisory]
//	            [--attempt N] [--requested-by <name>]
//	gate loop   --spec <file> --base <ref> [--max-attempts N] [--context spec-only|spec+invariants]
//	            [--record <dir> | --replay <dir>] [--stream]
//	gate doctor
//
// Exit codes of run: 0 PASS, 1 FAIL, 2 ESCALATE, 3 internal error. Advisory mode exits 0.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lioilsources/factory-gate/internal/gate"
)

const usageText = `usage:
  gate run    --base <ref> --head <ref> [--spec <file>] [--mode blocking|advisory] [--attempt N] [--requested-by <name>]
  gate loop   --spec <file> --base <ref> [--max-attempts N] [--context spec-only|spec+invariants] [--record <dir> | --replay <dir>] [--stream]
  gate doctor
  gate version`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usageText)
		os.Exit(3)
	}
	args := os.Args[2:]
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(args))
	case "loop":
		os.Exit(cmdLoop(args))
	case "doctor":
		os.Exit(cmdDoctor(args))
	case "version":
		fmt.Println(gate.Version())
	default:
		fmt.Fprintln(os.Stderr, usageText)
		os.Exit(3)
	}
}

func gitOut(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// repoRoot returns the top level of the repository and loads its .env, if any.
func repoRoot() (string, error) {
	repo, err := gitOut("rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	if err := loadDotEnv(repo + "/.env"); err != nil {
		return "", err
	}
	return repo, nil
}

// loadDotEnv sets KEY=VALUE lines from path that are not already in the environment.
// Values are never printed.
func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf(".env line %d is not KEY=VALUE", i+1)
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
	return nil
}

func requester(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	name, _ := gitOut("config", "user.name")
	return name
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "gate:", err)
	return 3
}
