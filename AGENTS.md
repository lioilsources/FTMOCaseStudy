# AGENTS.md

Rules for any agent working in this repository. Plan: `FACTORY_PLAN.md`. State: `PROGRESS.md`.

## Repo map

- `cmd/gate/` CLI entry point. `internal/gate/` diff, worktree, checks, verdict, evidence, audit.
- `internal/agent/` role contract, completion and command adapters, record/replay.
- `sample/orders/` the synthetic service under test. `sample/orders/invariants/` human-owned property tests.
- `specs/` task specs. `prompts/` role prompts. `evals/` gate evals. `demo/` demo script and recordings.
- `.factory/` runtime output, gitignored.

## Commands

- `make build` build and vet. `make test` all tests. `make invariants` property tests, fixed seed.
- `make gate` build the CLI. `make demo` offline demo. `make demo-live` live loop. `make eval` gate evals.

## Ground rules

1. Synthetic data only. No real company names, data or internal details anywhere.
2. No secret-shaped literals, not even fake ones. Tests assemble them at runtime.
3. Endpoints and credentials come from environment variables named in `gate.yaml`. Never log headers.
4. Never weaken a check to make something pass. If a check is wrong, write it in `PROGRESS.md` and stop.
5. Every demo artefact carries `source:` `live`, `recorded`, `replayed` or `seeded`.
6. No cherry-picking without disclosure. Every recorded run is listed in `demo/RUNS.md`.
7. Every number in README, BUILD_LOG or RESULTS comes from a command run in this repo.
8. Reality beats the plan: check `--help` of installed tools; record deviations in `PROGRESS.md`.
9. Dependencies: standard library, `gopkg.in/yaml.v3`, `pgregory.net/rapid`. Anything else needs Olin.
10. Money and quantities are `int64` minor units. No floats in `sample/`.

## Protected paths

You may read these, you may not change them:

- `sample/orders/invariants/**`, `gate.yaml`, `AGENTS.md`, `CLAUDE.md`, `.claude/**`, `prompts/**`
- `cmd/gate/**`, `internal/**`, `evals/**`, `.github/**`

For agents run by the factory, all of these are protected. For the session building this repo,
`cmd/gate`, `internal`, `prompts` and `evals` are delegated work; the human-owned ones are the
invariants, `gate.yaml` and these rule files. `.claude/settings.json` denies the edit tools on
human-owned paths, but it does not stop shell writes: the gate's diff check is the real enforcement.

## Commit trailers for model-made commits

The git author is always the human requester. The tool is named in trailers:

```
Agent: implementer/completion
Model: <model id>
Endpoint: <endpoint label>
Prompt-SHA256: <hash of the prompt file>
Run-ID: <id>
Requested-By: <human>
```

## PROGRESS protocol

- Start of session: read `FACTORY_PLAN.md` and `PROGRESS.md`, append a session row to `BUILD_LOG.md`.
- End of phase or session: update `PROGRESS.md` (status, deviations, open questions, next step), commit.
- At a STOP: commit, update `PROGRESS.md`, summarise in at most 10 lines, then wait for Olin.
