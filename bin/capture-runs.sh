#!/bin/sh
# Five live loop runs, as fixed in demo/CAPTURE_RULE.md. Runs one after another.
set -u
cd "$(dirname "$0")/.."
for n in ${RUNS:-1 2 3 4 5}; do
  dir="demo/recorded/run-$n"
  if [ -e "$dir" ]; then echo "$dir exists, not overwriting"; exit 1; fi
  echo "=== run-$n $(date '+%H:%M:%S')"
  .factory/bin/gate loop --spec specs/partial-fills.md --base main --context spec-only --record "$dir"
  echo "=== run-$n exit $? $(date '+%H:%M:%S')"
done
echo "=== all done"
