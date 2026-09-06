# Milestone β — the automated suite

This directory holds the automated suite for milestone β: crash recovery. It is **not** the demo
script; `test/alpha/README.md` states that split once and it is not repeated here.

Both run against `demos/beta/docker-compose.yml` — `CLAUDE.md § 5.5.4` forbids the suite from
defining an environment of its own.

```bash
./test/beta/run.sh              # the group
./test/beta/run.sh -run Crash   # any go test flag passes straight through
go test -short ./...            # skips it; needs no docker
```

## Why this group must stand alone

`CLAUDE.md § 5.5.3` makes it mandatory rather than tidy: β manipulates **global** coordination state
— `runs.owner_id` and the whole `orchestrators` table — and it **kills the orchestrator**. A test
running beside it would not merely see a database no single test put into that condition; it would
lose the process it was talking to. `run.sh` passes `-p 1`.

## The five legs

| Leg | What happens | Rule |
|---|---|---|
| 1 | `SIGKILL` while a step is in flight; restart; the run resumes | `§ 13.1.1` |
| 2 | A run already in `DLQ` before the crash is untouched by it | `§ 13.2.7` |
| 3 | A clean `SIGTERM` releases ownership at once | `§ 8.7` |
| 4 | The orchestrator is killed on every attempt; the run converges to `DLQ` | `§ 13.1.4`, `§ 12.2` |
| 5 | An orchestrator that cannot reach storage exits non-zero | `§ 13.1.5` |

**Legs 1 and 2 share one crash on purpose.** The DLQ'd run is created *before* the kill, so the same
restart that resumes one run must leave the other alone. This is the half of β that could not be
shown until γ existed, and it is why `SPEC.md § 18` orders γ before β.

**Legs 1 and 3 are a matched pair.** After `SIGKILL`, `runs.owner_id` still holds the dead process's
id — `§ 8.7` names four writers of coordination metadata and all four live inside an orchestrator
process, so a killed process releases nothing. After `SIGTERM` the column is already `NULL` before
anything else runs. `§ 8.7` calls release *"an optimisation that makes failover immediate rather than
`lease_ttl` later; correctness does not depend on it"* — these two legs are what that sentence looks
like from a terminal.

## Why the fixture is a script and the tests only assert

Most of what β demonstrates is visible **only while it happens**: that a run still has an `owner_id`
in the instant after a `SIGKILL`; that a clean shutdown has already cleared it. By the time Go's test
functions run, the system has moved on. So `TestMain` drives the legs and records what it observed,
with real queries at the moment they were true, and each test asserts one rule against that record
plus the final state of the database.

Every wait is on a **state**, never on a sleep of a computed length. `§ 13.3` makes a timeout a lower
bound — *"somewhere in `[deadline, deadline + sweep_interval]`"* when the owner is dead — so a test
that slept and then asserted would be testing a schedule `SPEC.md` deliberately does not promise. A
slow machine makes this suite slower, not red.

## Why `demos/beta/piton.yaml` does not use the default timings

α and γ run at `§ 8.6`/`§ 8.7`'s defaults (sweep 5 s, heartbeat 10 s, lease TTL 30 s). β runs at
2 s / 2 s / 6 s, and the reason is latency and nothing else: `§ 8.5`'s claim cannot take a run from an
orchestrator that is still live, so after every kill **nothing can happen** until the dead process's
lease expires. At the default TTL that is a 30-second pause in the middle of five legs, for a
property the numbers have nothing to do with.

The values change how *long* failover takes, never what recovery *does*. `§ 4.4` makes all three
configuration precisely so a deployment may choose, and the `§ 8.7` relationship is preserved — the
lease still tolerates two missed heartbeats (the same 3× ratio as 10 s / 30 s).

## Why `restart: "no"` is the key line of the compose file

Docker restarting the orchestrator by itself would take the demonstration away from the operator: he
would never see the state the crash left behind, because something else would have repaired it before
he looked. The recovery under test is Piton's (`§ 13`), never the container runtime's.

## What this suite does not cover

Two of `§ 13.1`'s six situations, and the reason is the same for both — neither can be staged from
outside the process:

| Not covered | Why |
|---|---|
| `§ 13.1.2` — a single run's driver dies while the process lives | There is no external handle on one driver. Staging it would mean adding a test-only hook to the orchestrator, which is a change to the product for the benefit of its test |
| `§ 13.1.6` — storage unreachable at runtime | Stopping Postgres mid-run and restarting it is possible, but the assertion — *"runs orphan intact and are reclaimed when storage returns"* — is the same state leg 1 already asserts, reached by a slower and far more timing-dependent path |

`§ 13.1.3` — a crash **during** recovery — is exercised without being isolated: leg 4 kills the
orchestrator three times in a row, and each kill lands on a process that has just recovered the
previous one.

Also absent, as in every milestone before δ: replay, cancellation, `raw` dispatch, `async`, an HTTP
planner, and overrides. Where their absence is observable, `TestWhatBetaDoesNotDemonstrate` asserts
it rather than leaving it unsaid.

## Where the assertions come from

`CLAUDE.md § 5.1` permits exactly one source: `SPEC.md`. Every assertion names the ratified section it
came from — `§ 13.1`, `§ 13.2`, `§ 8.5`, `§ 8.6`, `§ 8.7`, `§ 5.3`, `§ 5.5`, `§ 6.2`, `§ 6.4`,
`§ 12.2`, `§ 12.3` — and nothing was derived by reading `internal/`.

One assertion is deliberately looser than the others. `§ 13.1.5` requires *"an error message that
names storage as the cause"* and fixes no wording, so `TestStorageUnreachableAtStartupFailsFast`
checks the exit code exactly and accepts any of `storage`, `postgres`, `database` or `dsn`. A fixed
string would be inventing a contract no ruling covers (`CLAUDE.md § 9`).

## Duplication with the other harnesses

See `BACKLOG.md` **B18**. β's harness has diverged furthest — killing a process, restarting it, and
waiting for one orchestrator to take a run away from another are all new here — but the docker and
psql plumbing beneath is still a third copy.
