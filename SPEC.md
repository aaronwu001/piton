# Piton — Specification

**SKELETON — structure only.** Every section below carries one sentence saying what it will
contain. No section is filled in yet. The owner reviews this structure before any content is
written.

Items marked ⚠ are reconstructions or gaps that need an explicit ruling before they are written.

---

## 0. Status and Authority

States that this document is the single authority on system behaviour, that only the owner may
ratify a change to it, and that `notes/` and `GRILLING_LOG.md` may never be cited as justification
for anything.

## 1. Governing Principle

> The database is the only coordination mechanism. Every in-memory structure is a cache that may
> vanish at any instant without affecting correctness.

States the principle and its three already-decided consequences (attempt deadlines live in a
column; run ownership lives in a column; async results are delivered through a row), and explains
that this sentence is the tie-breaker for any future "should this live in memory?" question.

## 2. Purpose and Scope

### 2.1 What Piton is
One paragraph: a crash-safe orchestration engine for workflows whose next step is decided one step
at a time, with every decision and every result recorded in a plain relational schema.

### 2.2 What Piton is not
Names the things deliberately excluded so they are never mistaken for oversights: no DAG, no
authentication, no UI, no message broker, no distributed transaction across workers.

### 2.3 The audience this document is written for
States that rationale is first-class here — every non-obvious rule carries a `Why:` — because the
project exists partly to demonstrate the design, not only to run it.

---

## 3. Definitions

*Nothing earlier in this document uses a term defined here without a forward reference; nothing
later uses a term that is not defined here.*

### 3.1 Actors
Defines **orchestrator**, **planner**, **worker**, and **operator**, each in one sentence, with the
one thing each is uniquely responsible for.

### 3.2 Entities
Defines **workflow**, **run**, **step**, **attempt**, **StepSpec**, and **dead-letter entry**, and
states the containment relationships between them.

### 3.3 Identity and ordering
Defines `run_id`, `step_id`, `attempt_id`, `orchestrator_id`, and `seq`; states that UUIDs are the
identity and that `step_name` is a non-unique, optional display label a lazy planner may omit.

### 3.4 The two kinds of state
Defines **business state** (run / step / attempt status, outputs, budgets) against **coordination
metadata** (`orchestrators.last_seen_at`, `runs.owner_id`, `runs.claimed_at`), and states that only
the first is governed by the transaction rules in §8.

---

## 4. Architecture

### 4.1 Components and the boundaries between them
One diagram in prose: what crosses each boundary, and which boundaries are HTTP.

### 4.2 The driving loop
The core cycle — ask the planner, persist the decision, dispatch, record the outcome, repeat —
stated as the single loop everything else in this document modifies.

### 4.3 Ownership and liveness
Why a run has an owner, why ownership expires, and why liveness is a property of the orchestrator
process rather than of the run.

### 4.4 Deployment shape
States that the orchestrator is stateless, that multiple replicas must remain possible, and that
multi-replica operation is a constraint on the design rather than a feature being built.

---

## 5. State Model

### 5.1 Run states
`RUNNING`, `DONE`, `DLQ`, `CANCELLED` — each with its entry and exit conditions.

### 5.2 Step states
`RUNNING`, `DONE`, `DLQ`, `CANCELLED` — each with its entry and exit conditions.

### 5.3 Attempt states
`RUNNING`, `DONE`, `FAILED` — plus the failure reasons that are diagnostic labels rather than
distinct mechanisms.

### 5.4 The `last_step` convention
States the convention that collapses the table: *a run with no steps is defined to have
`last_step = DONE`*, and shows the four cells it removes.

### 5.5 The run × last_step combination table
The eight legal combinations L1–L8, what each means, and which of the two recovery actions applies
when the run is claimed. **This table is load-bearing and is placed prominently by design.**

### 5.6 Impossible combinations
The eight impossible combinations, each with the transaction boundary that makes it impossible.

### 5.7 The uniform cancel rule
The single sentence that covers all three cancelled combinations without special cases, including
why `attempt_count` is not incremented on cancellation and why a DLQ verdict is never rewritten.

---

## 6. Data Model

*Column meanings and their conditional logic live here; the reasoning behind them lives in
`notes/`.*

### 6.1 `workflows`
### 6.2 `runs`
### 6.3 `steps`
### 6.4 `attempts`
### 6.5 `dead_letter_queue`
### 6.6 `orchestrators`

Each subsection is a column table — name, type, nullability, meaning — followed by that table's
invariants.

### 6.7 Table discipline
States the rule that `runs` / `steps` hold current status as one mutable row each, while
`dead_letter_queue` is append-only history that is never modified and may hold many rows per run.

---

## 7. Storage Requirements

*A backend that does not meet these is not a conforming implementation.*

### 7.1 Interface obligations
States that the storage interface takes and returns opaque `[]byte` for every JSON document and
must not assume JSONB or any backend-specific type.

### 7.2 Required indexes
Lists the indexes the behaviour in this document depends on, including the partial index on running
runs that keeps sweep cost proportional to concurrency rather than to history.

### 7.3 Atomicity and isolation obligations
States the guarantees required from the backend, including that an `UPDATE` blocked by a row lock
must re-evaluate its `WHERE` clause after the lock is released.

---

## 8. Concurrency Control

### 8.1 The primitive
Defines compare-and-set as a write whose effect is conditional on the current state, evaluated
atomically with the write, and states that zero rows affected means the expectation was wrong.

### 8.2 The ownership fence
The fence as a use of §8.1 whose predicate is ownership; why it is one row lock at the top of a
transaction rather than a predicate repeated in every statement; and what a writer does when it
finds it has been superseded.

### 8.3 The attempt CAS
The second predicate — one writer per attempt outcome — and why it also rejects late reports with
no separate mechanism.

### 8.4 The single exception
States that an async callback arriving at a non-owner may write the `attempts` table under §8.3
alone, may write nothing else, and why recording a fact differs from making a decision.

### 8.5 Claiming
The single atomic statement that transfers ownership, and why exactly one sweeper wins each run.

### 8.6 The sweep
States that every orchestrator runs its own sweep, that the sweep only claims and never touches
business state, and what it scans.

### 8.7 Heartbeat and release
The one-row-per-process heartbeat, the liveness predicate, and the three operations that are
permitted to write coordination metadata.

---

## 9. Wire Protocol

### 9.1 Format boundary
Config files on disk are YAML; HTTP bodies are JSON; the database stores JSON — and no file may
have an extension that disagrees with its content.

### 9.2 Orchestrator → planner
The request, field by field, including the history catalogue, the cap of the most recent **100**
steps, and the statement that a planner must not assume the catalogue is complete.

### 9.3 Planner → orchestrator
The three responses — `continue`, `done`, `fail` — and the rule that `StepSpec` is one field of a
`continue` response rather than the response itself.

### 9.4 StepSpec
Every field, its type, whether it is required, and which fields are meaningful in which mode.

### 9.5 Orchestrator → worker
The envelope, and the raw body, stated separately because they share no fields.

### 9.6 Worker → orchestrator
How success and failure are expressed in each mode, and the rule that transport failure is always
failure regardless of body.

### 9.7 Legal mode combinations
The three legal combinations of `connection_mode` × `dispatch_style`, and the mechanical reason the
fourth cannot exist.

### 9.8 Rejected StepSpecs
What makes a StepSpec invalid, and the statement that an invalid StepSpec is a planner failure that
consumes planner budget.

---

## 10. HTTP API

*The read endpoints are shared by the operator and the planner; they are one surface, not two.*

### 10.1 Control endpoints
```
POST /workflows                     create a workflow definition
GET  /workflows                     list workflow definitions
GET  /workflows/{id}                read one workflow definition
POST /workflows/{id}/runs           start a run
POST /runs/{run_id}/replay          replay a run that is currently in DLQ
POST /runs/{run_id}/cancel          cancel a run
```

### 10.2 Read endpoints
```
GET  /runs                          list runs, filterable by status
GET  /runs/{run_id}                 one run, with a summary of its steps
GET  /runs/{run_id}/steps           the step catalogue, with attempt summaries
GET  /runs/{run_id}/dlq             the append-only dead-letter history for this run
GET  /steps/{step_id}               one step in full
GET  /steps/{step_id}/output        that step's stored output bytes, verbatim
```

### 10.3 Callback endpoint
```
POST /callbacks/{attempt_id}        an async worker reports its result
```

### 10.4 Operational endpoints
```
GET  /healthz                       liveness, including whether storage is reachable
```

### 10.5 Error responses
The error body shape, and the rule that a rejection states the actual current state rather than
only that the request was refused.

⚠ **Open:** whether `GET /runs/{run_id}/dlq` is the right shape, or whether the DLQ history belongs
inside `GET /runs/{run_id}`. Not previously discussed.

---

## 11. Configuration

### 11.1 Workflow-level fields
The five `step_*` / `planner_*` knobs with their defaults, and the explicit statement that
`step_max_attempts` is a **total** attempt count and not a retry count.

### 11.2 Layering
Workflow-level now; run-level and step-level fields designed in but rejected until milestone η, and
the rule that a rejection is a 400 rather than silence.

---

## 12. Failure, Retry and the Dead-Letter Queue

### 12.1 What counts as a failure
### 12.2 Budget and the transition to DLQ
### 12.3 Worker-side versus planner-side DLQ
### 12.4 What a dead-letter entry records

Each subsection states the rule; §12.2 states why an unbounded retry is structurally impossible.

---

## 13. Recovery

### 13.1 What recovery handles
The six situations recovery is guaranteed to survive.

### 13.2 What recovery does not handle
The published non-guarantees, including duplicate worker execution, committed side effects, planner
non-determinism across a crash, loss of the database itself, and the rule that recovery never
auto-replays a DLQ'd run.

### 13.3 Timeouts are lower bounds
States that a timeout is a lower bound on when failure is declared rather than an exact instant, and
which mechanism is authoritative when the two disagree.

---

## 14. Replay

States that replay targets the run rather than a dead-letter entry, that the idempotency gate is
"is this run in DLQ right now", that `run_id` never changes across rounds, and the accepted
limitation that a replay always resumes at the furthest step reached.

---

## 15. Cancellation

States which runs may be cancelled, the state transitions it produces, and the published
non-guarantee that cancellation stops the orchestrator from advancing the run but does not abort
work already handed to a worker.

---

## 16. Validation

States the governing principle — better to be too strict and be told we do not support something,
than too lax and let a user fail silently — and lists what must be decidable at submission time
rather than deferred to run time.

---

## 17. Observability for the Operator

States the hard requirement that the owner must be able to run the system by hand and see inside it
from a terminal, that database truth is the interface, and that automated tests exist to guarantee
that what he sees by hand is the expected behaviour.

---

## 18. Milestones

*Milestones are demo scenarios, not layers. Each one is a user-visible capability that can be shown
end to end.*

⚠ **Reconstruction — needs confirmation.** `GRILLING_LOG` R5-f fixes the order and names ζ, θ, ε, η
and ι explicitly, but never spells out α, β, γ or δ. The four below are inferred from R1-7 (the
owner's own ordering) and R3-11 (DLQ before crash recovery). Confirm or correct before this section
is written.

| Order | Milestone | Capability demonstrated |
|---|---|---|
| 1 | α | Basic happy-path run: static planner, sync envelope worker, run reaches DONE |
| 2 | γ | ⚠ Retries and the dead-letter queue: kill a worker, watch attempts burn, watch it land in DLQ |
| 3 | β | ⚠ Crash recovery: kill the orchestrator mid-run, watch it resume; DLQ'd content is untouched |
| 4 | δ | ⚠ Replay in its variants |
| 5 | ζ | Custom HTTP / LLM planner, and switching between planners |
| 6 | θ | Sync raw body — an unmodifiable HTTP endpoint works as a worker |
| 7 | ε | Async envelope |
| 8 | η | Per-step and per-run overrides |
| 9 | ι | Cancellation and submission-time validation |

Each milestone section states its demo script: what the operator does by hand, and what he must see
in the database afterwards.

---

## 19. Open Assumptions

Everything not yet settled or not yet guaranteed, in one place, so that no reader has to consult a
second registry to find out whether something is true.

---

## Appendix A. Glossary index

An alphabetical pointer from every defined term to the section that defines it.
