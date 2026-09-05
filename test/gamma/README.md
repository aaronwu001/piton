# Milestone γ — the automated suite

This directory holds the automated suite for milestone γ. It is **not** the demo script; the two are
different steps of `CLAUDE.md § 4` with different jobs — see `test/alpha/README.md`, which states
that split once and is not repeated here.

Both read the same two interfaces: the HTTP API of `SPEC.md § 10`, and database truth
(`SPEC.md § 17.1`). Both run against `demos/gamma/docker-compose.yml` — `CLAUDE.md § 5.5.4` forbids
the suite from defining an environment of its own, so that the owner's hand-run demo and the suite
cannot diverge.

## How to run it

```bash
./test/gamma/run.sh              # every group
./test/gamma/run.sh -run Retry   # any go test flag passes straight through
go test -short ./...             # skips the group; needs no docker
```

## The one group

`CLAUDE.md § 5.5.1` brings up one docker-compose environment **per test file or per milestone
scenario**, and `SPEC.md § 18.2` is one scenario: four runs against one environment, which is what
its `demo.sh` row requires of the demo too. So γ has a single group, `workerfailure`, and its
`TestMain` runs all four legs before any test asserts anything.

| Leg | Workflow | How the worker fails | `failure_reason` (`§ 5.3`) | Where the run ends |
|---|---|---|---|---|
| 1 | `workflow-retry.json` | fails twice, then succeeds | — on the first two | `DONE`, `attempt_count = 3` |
| 2 | `workflow-worker-error.json` | replies 200 with a failure envelope | `worker_error` | `DLQ` (`§ 12.3` **L4**) |
| 3 | `workflow-http-500.json` | replies HTTP 500 | `transport_error` | `DLQ` (**L4**) |
| 4 | `workflow-unreachable.json` | nothing is listening | `transport_error` | `DLQ` (**L4**) |

**Legs 3 and 4 share a `failure_reason` on purpose.** `§ 5.3` defines `transport_error` as *"the HTTP
exchange did not produce a usable reply — non-2xx, connection refused, DNS failure, connection
reset"*. A suite that exercised only one of them would leave half of that definition unasserted, and
the two are the cases an implementation is most likely to collapse into one.

**`§ 5.5.3` does not apply here.** That rule makes α's `ownership` group stand alone because it
asserts *global* coordination state. Nothing in γ manipulates `orchestrators` or another run's
`owner_id`; the two tests that do speak about every run at once —
`TestImpossibleCombinationNeverPersisted` and `TestPlannerBudgetUntouched` — assert invariants that
must hold whichever runs exist.

## Why each DLQ leg declares a second static step it never reaches

`SPEC.md § 18.2` requires *"no step created after it"*. A workflow whose `planner_static_steps` ended
at the failing step could not show that — there would be nothing the planner could have created even
if it had been asked. The second element makes the absence evidence rather than a tautology, and
`TestNoStepIsCreatedAfterTheFailingOne` fails loudly if a workflow file ever loses it.

## What this group asserts beyond the five queries of § 18.2

Each of these comes from a ratified section other than `§ 18.2`, and is here because γ is the first
milestone in which it is observable at all:

| Test | Rule |
|---|---|
| `TestRetryStepIsDoneOnTheLastAttemptTheBudgetAllows` | `§ 11.1` — `step_max_attempts` is a **total** attempt count, not a retry count |
| `TestBudgetIsBurnedAtDispatch` | `§ 4.2` — the counter moves at dispatch, so within a round with no cancel and no replay it equals `count(attempts)` (`§ 6.3`) |
| `TestUnreachableIsTransportErrorNotTimeout` | `§ 5.3` — `timeout` is decided by the clock, never by the shape of the error |
| `TestImpossibleCombinationNeverPersisted` | `§ 5.6`, `§ 12.2` — the step, the run and the dead-letter entry are one transaction |
| `TestPlannerBudgetUntouched` | `§ 12.1` — the static planner cannot fail at run time |
| `TestEveryAttemptNamesItsOrchestrator` | `§ 6.4` — `dispatched_by` |
| `TestWhatGammaDoesNotDemonstrate` | `§ 14`, `§ 15`, `§ 5.3` — replay, cancellation and `orphaned` are other milestones, and their absence is asserted rather than assumed |

## Where the assertions come from

`CLAUDE.md § 5.1` permits exactly one source: `SPEC.md`. Every assertion names the section it came
from, and a failure reports that section rather than only the query that returned false. Nothing was
derived by reading `internal/`.

If an assertion here and `SPEC.md` disagree, `SPEC.md` wins (`§ 5.2`). If you believe `SPEC.md` is
wrong, say so and stop — the owner rules, `SPEC.md` changes first, and the assertion changes second.
Never adjust an assertion to match the code.

## What this suite does not cover

The **planner side** of `§ 12.3`. `§ 18.2` states why: the static planner cannot fail at run time
(`§ 12.1`), so the only way to reach **L5** today is an unbuilt planner reporting that it is unbuilt
— which would demonstrate the milestone's absence, not the mechanism. It belongs to **ζ**. What γ
does assert is that no planner-side entry appeared by accident: every dead-letter row names a step.

Also absent: crash recovery (β), replay (δ), cancellation (ι), `raw` dispatch (θ), `async` (ε), and
overrides (η).

## Why `test/gamma/harness` duplicates `test/alpha/harness`

The two differ in one thing that matters — which demo directory they point at — so a shared package
was possible. It was not done. α's suite is the guard on a milestone the owner verified by hand, and
refactoring it to serve γ would put that guard at risk for a later milestone's convenience.
`SPEC.md § 4.4` gives each milestone its own directory for the same reason. If a third milestone
wants the same plumbing, that is the point at which extracting it is worth the risk, and it is in
`BACKLOG.md` rather than assumed.
