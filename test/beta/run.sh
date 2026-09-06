#!/usr/bin/env bash
#
# Milestone beta - the automated suite.
#
# CLAUDE.md 4 step 6: this suite job is to guarantee that what the owner saw by
# hand stays true. It does not replace step 5, and a green run the owner has
# never looked behind is not evidence the milestone landed.
#
#   ./test/beta/run.sh              run every group
#   ./test/beta/run.sh -run Crash   pass any flag straight through to go test
#
# Everything executes inside WSL (CLAUDE.md 8). The suite needs docker and the
# Go toolchain; it needs no psql, because it reaches the database through
# `docker compose exec postgres psql`, exactly as demo.sh does.
#
# WHY -p 1
#   CLAUDE.md 5.5.3 is not advice here, it is the rule this suite exists under:
#   beta manipulates global coordination state - runs.owner_id and the
#   orchestrators table - and it KILLS THE ORCHESTRATOR. A test running beside
#   it would not merely see a database no single test put into that condition;
#   it would lose the process it was talking to. -p 1 is what makes the group
#   discipline real rather than intended.
#
# WHY -count=1
#   A cached PASS would report that a milestone still holds without having
#   started a container at all.

set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

for tool in docker go; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "run.sh: required tool not on PATH: $tool" >&2; exit 2; }
done

echo "Milestone beta - automated suite"
echo "  groups run one at a time, each against its own clean environment"
echo "  environment: demos/beta/docker-compose.yml (CLAUDE.md 5.5.4 - the same"
echo "  file the owner hand-run demo uses; the suite defines none of its own)"
echo

exec go test -p 1 -count=1 -timeout 40m -v ./test/beta/... "$@"
