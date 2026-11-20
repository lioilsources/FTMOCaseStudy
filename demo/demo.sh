#!/bin/sh
# make demo: the factory loop, offline.
#
# source: replayed. Model output comes from the default recording (demo/DEFAULT); the gate
# runs live on every step. The shortcut case is source: seeded (demo/attempts/).
# Set DEMO_NOPAUSE=1 to run without pauses.
set -eu
cd "$(dirname "$0")/.."

GATE=.factory/demo/gate
REC="demo/recorded/$(cat demo/DEFAULT)"
SPEC=specs/partial-fills.md
STAMP=$(date +%Y%m%d%H%M%S)
mkdir -p .factory/demo

pause() {
	if [ -z "${DEMO_NOPAUSE:-}" ]; then
		printf '\n(press enter) '
		read -r _
	fi
}
step() { printf '\n\033[1m== %s\033[0m\n' "$1"; }
show() { printf '\033[2m$ %s\033[0m\n' "$*"; "$@"; }

step "1. The task, as a product owner wrote it (no word about duplicate events)"
show sed -n '1,30p' "$SPEC"
pause

step "2. Build the gate"
show go build -o "$GATE" ./cmd/gate
printf 'recording: %s (source: recorded live on the implementer model; replayed now)\n' "$REC"
pause

step "3. Replay the recorded loop. The model's replies are replayed; the gate runs live"
set +e
show "$GATE" loop --spec "$SPEC" --base main --replay "$REC" | tee .factory/demo/loop.out
set -e
LOOP=$(sed -n 's/^loop \(l-[0-9]*\) finished.*/\1/p' .factory/demo/loop.out)
RUNS=$(sed -n 's/^  attempt [0-9]*  *[A-Za-z_]*  *\(r-[0-9]*\).*/\1/p' .factory/demo/loop.out)
pause

N=0
for R in $RUNS; do
	N=$((N + 1))
	step "4.$N Attempt $N: what the model changed"
	show git --no-pager show --stat --format='%h %s%n%n%(trailers)' "loop/$LOOP/attempt-$N"
	if [ -f ".factory/runs/$R/failure_report.md" ]; then
		step "4.$N Attempt $N: the failure report sent back to the model (head)"
		show sed -n '1,25p' ".factory/runs/$R/failure_report.md"
	fi
	pause
done

LAST=$(echo "$RUNS" | tail -n 1)
step "5. Evidence of the final attempt: what a reviewer reads instead of the diff"
show sed -n '1,/^## Counterexample/p' ".factory/runs/$LAST/evidence.md"
show sed -n '/^## Read these first/,/^## What this gate/p' ".factory/runs/$LAST/evidence.md"
pause

step "6. The shortcut (source: seeded): fake the fix by weakening the invariant"
BR="demo/seeded-$STAMP"
show git worktree add -q -b "$BR" ".factory/demo/seeded-$STAMP" main
(cd ".factory/demo/seeded-$STAMP" &&
	git apply ../../../demo/attempts/weakened-invariant.patch &&
	git -c user.name=demo -c user.email=demo@example.invalid commit -q -am "seeded: weakened invariant")
git worktree remove --force ".factory/demo/seeded-$STAMP"
set +e
show "$GATE" run --base main --head "$BR" --spec "$SPEC" --source seeded
set -e
pause

step "7. The audit trail"
show tail -n 4 .factory/audit.jsonl
printf '\nDone. Every run is listed in demo/RUNS.md; the capture rule is demo/CAPTURE_RULE.md.\n'
