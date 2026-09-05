# Milestone α — the automated suite

This directory holds the automated suite for milestone α. It is **not** the demo script; the two
are different steps of `CLAUDE.md § 4` with different jobs.

| | `demos/alpha/demo.sh` | this suite |
|---|---|---|
| Which step | 2 — written before the code | 3 to write, 6 to guard |
| Job | fix what the operator must see, and let him see it | guarantee that what he saw **stays** true |
| Acceptance evidence | **yes** (`§ 4` step 5) | no — a green run the owner has never looked behind is not evidence a milestone landed |

Both read the same two interfaces: the HTTP API of `SPEC.md § 10`, and database truth
(`SPEC.md § 17.1`). Both run against `demos/alpha/docker-compose.yml` — `CLAUDE.md § 5.5.4` forbids
the suite from defining an environment of its own, so that the owner's hand-run demo and the suite
cannot diverge.

## How to run it

```bash
./test/alpha/run.sh              # every group
./test/alpha/run.sh -run Envelope   # any go test flag passes straight through
go test -short ./...             # skips both groups; needs no docker
```

Everything executes inside WSL (`CLAUDE.md § 8`). No `psql` is needed on the host: the suite reaches
the database through `docker compose exec postgres psql`, the same path `SPEC.md § 17.1` gives the
operator, because `demos/alpha/docker-compose.yml` deliberately publishes no host port for Postgres.

## The groups

`CLAUDE.md § 5.5` defines a group as one docker-compose environment, brought up for a set of tests
and torn down — volume wipe included — before the next group starts. A Go package is the unit that
can own a `TestMain`, so **one package is one group**, and `run.sh` passes `-p 1` so that Go does not
run two of them at once.

| Package | Group | Contains |
|---|---|---|
| `happypath` | the α scenario | the run reaches `DONE`; steps, attempts and the dead-letter count are what `§ 18.1` requires; the dispatch envelope matches `§ 9.5`; `input_from` resolves as `§ 9.4` says; and everything α does **not** demonstrate is asserted absent |
| `ownership` | coordination state | exactly one orchestrator row, live by `§ 4.3`'s definition; the heartbeat actually advances `last_seen_at` (`§ 8.7`); a run that has left `RUNNING` holds neither `owner_id` nor `claimed_at` (`§ 6.2`, `§ 8.7`); every attempt names the orchestrator that dispatched it (`§ 6.4`) |
| `validation` | what is refused at submission | all six rules of `§ 16` at `POST /workflows`; all six of `§ 9.8` against every element of `planner_static_steps`; and `§ 16`'s run-creation rules — a non-empty `overrides` refused, `{}` / `null` / omission all accepted, a missing `input` refused |

`ownership` is separate for the reason `§ 5.5.3` gives: it asserts **global** coordination state —
the whole `orchestrators` table, and `runs.owner_id` across every run — so a test running beside it
would see a database that no single test put into that condition. "Exactly one orchestrator row" is
only meaningful when nothing else may start a second one.

Each group creates its own fixture in `TestMain`: one workflow, one run, walked to a terminal state.
`SPEC.md § 18.1`'s scenario **is** a single run, so the tests assert different rules about the state
that one run left behind rather than each starting a run of their own.

## Where the assertions come from

`CLAUDE.md § 5.1` permits exactly one source: `SPEC.md`. Every assertion names the section it came
from, and a failure reports that section rather than only the query that returned false. Nothing was
derived by reading an implementation — at the time this was written there is none, which is the
point of `§ 4` step 3 and R20-a.

**Until milestone α is implemented, this suite fails at `TestMain`**, because the orchestrator
container has nothing to serve. That is the expected state, not a defect.

If an assertion here and `SPEC.md` disagree, `SPEC.md` wins (`§ 5.2`). If you believe `SPEC.md` is
wrong, say so and stop — the owner rules, `SPEC.md` changes first, and the assertion changes second.
Never adjust an assertion to match the code.

## Why validation is asserted here and not at milestone ι

`SPEC.md § 18`'s table gives *"cancellation and submission-time validation"* to milestone **ι**, so
the `validation` group needs a word of explanation. `§ 18.1` now states it directly: **α implements
all of `§ 16`; ι demonstrates validation, it does not introduce it** — milestones are demo
scenarios, not layers (`§ 18`).

The part that could never have waited is `§ 16` rule 3, and `§ 6.1` gives the reason as correctness
rather than tidiness: a malformed StepSpec discovered at run time leaves a run that can neither
progress nor fail — it is not the planner refusing, and no attempt exists to burn budget — so it
would sit `RUNNING` forever, reclaimed and re-failed by every sweep. Validating at submission makes
that state unreachable. α uses the static planner, so α needs the whole StepSpec validator; the rest
of `§ 16` is then a few checks on the same parse.

## What this suite does not cover

Everything α does not implement: retries, DLQ, crash recovery, replay, cancellation, raw dispatch,
async, an HTTP planner, and overrides — each has its own milestone (`SPEC.md § 18`). Where their
absence is observable, `happypath` asserts it rather than leaving it unsaid.

One thing is deliberately observed but not asserted anywhere: whether the run passed through
`RUNNING` before reaching `DONE`. The echo pipeline finishes in milliseconds, so catching it is a
race; `SPEC.md § 5.1` makes `RUNNING` the state a run is created in, and the end state is what
`§ 18.1` requires.
