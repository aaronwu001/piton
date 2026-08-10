# StateFlow v2 — Grilling Log

Purpose: capture the owner's rulings that will produce `CLAUDE.md` + the new minimal spec.
Format: one round per section. Question → owner's ruling → consequence.
Nothing here is spec. Once a ruling lands in `CLAUDE.md` or the spec, it stays here only as rationale.

Started: 2026-08-08

---

## Round 0 — What I read (evidence, not claims)

Inventory of `./docs` (3,800 lines, 10 files):

| File | Lines | Self-declared status | Reality |
|---|---|---|---|
| `StateFlow_Whitepaper_v1_0.md` | 610 | "Authoritative design as of v1.0" | Lags implementation in ~12 places |
| `WHITEPAPER_V1_1_PATCHES.md` | 249 | Patch list to be merged into the above | Never merged; CN-language |
| `BEHAVIOR_MATRIX.md` | 398 | "Authoritative behavior spec, wins all conflicts" | ~150 assertion rows, CN-language, references `spec/` path that does not exist here |
| `StateFlow_Rules_Consolidation_v3_EN.md` | 334 | "Reference, not authority" (was authority in v3) | Stale; 5 ratified changes missing |
| `CLAUDE-old.md` | 344 | Development discipline | Contradicts BEHAVIOR_MATRIX on `retry_limit` location |
| `BACKLOG.md` | 43 | "Not yet true" | 16 whitepaper repairs owed + 9 future items |
| `OPERATIONAL_FACTS.md` | 826 | How to run it; no behavior | Environment-specific, dated 2026-08-03 |
| `USER_MANUAL.md` | 549 | User-facing | Not read in full yet |
| `README.md` | 289 | Public README | Contains a 4th copy of the API table |
| `HANDOFF_23AB.md` | 158 | Session handoff | Template with `<<< >>>` placeholders unfilled |

**Structural finding:** there are four separate registries for "things not settled" —
whitepaper §18 (Temporary Design Registry), BEHAVIOR_MATRIX §K (explicit non-guarantees),
BEHAVIOR_MATRIX §M (裁定紀錄 / rulings), and `BACKLOG.md`. Plus §21 (Roadmap).
Any single question ("is X guaranteed?") can be answered differently by four documents.
This — not the technical content — is the actual defect.

**Second structural finding:** the API surface is stated in full in 4 places
(whitepaper §12–13, CLAUDE-old, README, USER_MANUAL) with drift between them.

**Third finding:** `/home/aaronwu/stateflowv2` contains **only** `docs/`. It is not a git
repo. There is no code and no tests here. The referenced implementation lives elsewhere
(`~/Projects/StateFlow` per OPERATIONAL_FACTS §0), and `spec/BEHAVIOR_MATRIX.md` — the
path every doc calls authoritative — does not exist at that path in this tree.

---

## Round 1 — Rulings (2026-08-08)

| # | Question | Ruling | Consequence |
|---|---|---|---|
| 1 | Rewrite vs. continue old repo | **Full rewrite.** Reason given: "so that I can understand it" | New repo at `/home/aaronwu/stateflowv2`. No code is ported by copy-paste |
| 2 | Old repo access | **Readable at `~/Projects/StateFlow`, reference only.** Explicit warning: do not let it hallucinate us into its assumptions — **especially when writing tests**; tests must be derived from SPEC, never from reading old test files | Reading old impl for "how they did X in Go" is allowed; reading old tests to write new tests is not |
| 3 | Primary audience | **Learning + demonstrating the ability to independently design and build under distributed-systems assumptions.** Rationale is therefore first-class, not decoration | Every non-obvious rule in SPEC carries a `Why:` |
| 4 | File set | **`SPEC.md` + `CLAUDE.md`, nothing else.** Open problem raised by owner: the grilling log must not become a third source of confusion — needs a hard separation mechanism | Answered in Round 2 Q1 |
| 5 | Old BEHAVIOR_MATRIX's ~150 rows | **Do not inherit.** Re-derive a minimal set. Conflicts with other docs are surfaced and ruled one by one. **Design principle: start from the simplest thing that works; expand only when a concrete need appears — do not treat every old feature as a parallel requirement.** Shipping something with small known bugs earlier beats shipping something complete later | Feature scope is subtractive by default. The burden of proof is on including something |
| 6 | Where assumptions live | **`SPEC.md § Open Assumptions`** for unsettled/unguaranteed things; **`CLAUDE.md`** for future work | Four old registries (§18, §K, §M, BACKLOG) collapse into two locations |
| 7 | Unit of "one feature" | **Not TX — a user-visible capability, demoable end to end.** Owner's own ordering: (1) basic happy path runs; (2) kill a worker → see retries → see it land in DLQ; (3) replay works in its variants; (4) static vs. custom planner configuration and switching; … Complexity like input-field validation is pushed to the back deliberately | Milestones = demo scenarios, not layers. See Round 2 Q3 |
| 8 | Old ceremony (read-only spec, sha256 freeze, blind-test sessions, handoff docs) | **All cut.** What matters is features landing. Test discipline reduces to: tests must not be written by reading the code. **New hard requirement: the owner must be able to run the system by hand and see inside it — database state and, ideally, other runtime state — not merely watch an automated suite pass.** Automated tests exist to guarantee that what he sees by hand is the expected behavior | Observability-for-the-operator becomes a real requirement, not a Phase-2 nicety |
| 9 | Who may edit SPEC | **Owner only, on an explicit ruling.** Claude proposes; Claude never edits | The one piece of ceremony that survives |
| 10 | Tech stack | **Unchanged: Go + Postgres + Docker Compose + golang-migrate** | Not a source of the problem |

---

## Round 2 — Rulings (2026-08-08)

| # | Question | Ruling |
|---|---|---|
| 1 | Separation mechanism for the three files | **Split by voice, not by topic.** `SPEC.md` = "the system MUST…" + `Why:`. `CLAUDE.md` = "you (Claude) MUST…" + milestones + future work. `notes/` = "what we discussed and decided". **Amendment by owner: do not bother deleting superseded log entries — the only rule that matters is that `notes/` may never be cited as justification.** Deleting is maintenance cost for no benefit |
| 2 | Old `./docs` | **Move to `legacy/` + `legacy/README.md` marking it VOID.** Reference only: read it for "how they thought", never for "so we do the same" |
| 3 | Milestone list | **Not settled.** Owner's list was an example, not an inventory. Requires a full feature brainstorm first (Round 3). **Explicit owner request: the run × last_step combination table and its recovery rules — his own design, which he likes — must be reviewed and settled BEFORE any implementation starts, because it is where the edge cases live. If it audits clean, adopt it wholesale** |
| 4 | Async priority | **Async is not deferrable in design, only in implementation.** It determines the practicality of the product and is the source of most edge cases. Sync-first is acceptable **only if** every message carries an explicit `connection_mode` field and the spec's field formats are defined per mode from day one — otherwise sync assumptions bake in and later async work forces a format break |
| 5 | Operator observability | **Terminal access to database truth. No UI.** A web page adds complexity for no gain here |
| 6 | `step_id` | **UUID is the identity. `step_name` is a non-unique, optional display label** — a lazy planner may omit it entirely |
| 7 | Message shapes / who assembles worker input | **Open — the hardest design question in this project.** Owner's position, in his words: StateFlow stores everything; the planner decides what context flows onward; no LLM should see more than it needs; StateFlow itself should not be in the business of shaping outputs. Unresolved sub-question he raised himself: what does the planner receive after a worker finishes? Requires a concrete proposal (Round 3) |
| 8a | `output_field` | Depends on Q7 |
| 8b | history size limits | **Required eventually, low priority** |
| 8c | retry mechanics | **Implement later, but put the columns and structures in the schema from the start.** Early milestones demo with `retry_limit = 0` (no retry) rather than with a missing field |
| 8d | per-step `retry_limit` / `timeout` override | **A selling point — mid priority.** After core recovery, not at the very end |
| 8e | in-process sweeper | **Deferred pending explanation** — owner does not remember how it differs from recovery; re-derive from the old docs and re-decide |
| 8f | late-result salvage | **Permanently out.** Once retried or DLQ'd, the old reply is rejected by CAS on `step_id` + `attempt_id` |
| 8g | multi-replica | **Must be possible later — a selling point**, because the orchestrator is stateless: partition workflows across orchestrators, all writing to one DB. Do not design anything that forecloses it |
| 8h | **NEW: storage abstraction layer** | **Required from the start.** One relational backend today (Postgres), but the interface must not assume JSONB or any Postgres-specific type. A future implementer on another relational DB must be able to choose their own encoding |
| 8i | auth | Out of scope |
| 9 | Config layering | Workflow-level config set at creation. **Run-level override: designed-in now (the run API accepts parameters), unused initially.** Step-level override: same pattern, low priority. **The planner gets its own retry limit and timeout, configured at the workflow level under clearer names** — these determine when a run goes to DLQ because of the planner rather than a worker |
| 10 | Project name | **Rename.** The old StateFlow repo gets a deprecation notice pointing at the new one. Need an unused name meaning: lightweight, general-purpose orchestration engine, quick for teams to adopt, and it remembers every step in a plain relational schema |

---

## Round 3 — Rulings (2026-08-09)

### Settled

| # | Topic | Ruling |
|---|---|---|
| R3-1 | Combination-table dependencies 2 & 3 (seq assigned by one writer; every state write is exactly one transaction) | **Accepted as load-bearing.** They must appear prominently in SPEC, not buried |
| R3-2 | Reserved lease columns on `runs` | **Accepted** — add now, implement later. Follow-up question answered in Round 4: nullable, not required |
| R3-3 | Sync worker receiving a **raw** (un-enveloped) body | **NOT cut.** Owner considers "any unmodifiable HTTP endpoint is a valid worker" a major selling point. It becomes an add-on with relatively high priority, behind a per-step `dispatch_style` field that is designed in from day one |
| R3-4 | Exponential backoff | Later, low priority. Competes with worker-requested retry delay for the same slot |
| R3-5 | Named worker registry | Very low priority |
| R3-6 | `GET /ui` | **Not cut, only deprioritized.** May still be useful for demos. Terminal-first is the requirement, not terminal-only |
| R3-7 | Structured logging | **Probably unnecessary** — the database already holds the state. (Round 4 refines: keep error *text* in the DB, keep stdout minimal) |
| R3-8 | Prometheus metrics | **Dropped.** Owner: out of the blue and ambiguous. Claude over-reached |
| R3-9 | Run cancellation | **In.** Introduce `CANCELLED` and nothing else. Kill the driver. If last step was RUNNING it becomes CANCELLED; if it was DONE it stays DONE. Result: `run=CANCELLED, last_step=CANCELLED|DONE`. **Only a RUNNING run may be cancelled** |
| R3-10 | Sweeper now + reserved columns | **Agreed** (superseded by Round 4's stronger proposal — see R4-1) |
| R3-11 | Milestone order | **DLQ before crash recovery.** Reason: the crash-recovery demo's point is partly that DLQ'd content is *not* touched by recovery — it must wait for an explicit replay. You cannot show that until DLQ exists |
| R3-12 | Planner response format | The old project's biggest time sink was arguing about syntactic/semantic validation of planner responses. **Mitigation: write the expected format explicitly and completely up front**, so validation becomes mechanical rather than a design discussion |
| R3-13 | Project name | `Cairn` is the current front-runner but the search continues. Target: as immediately understandable as "breadcrumb", but more explicitly about **workflows — especially non-deterministic / AI ones — that are crash-safe**, and readable as an orchestration engine. Two words acceptable |

### Owner's questions carried into Round 4

| # | Question as asked | Where answered |
|---|---|---|
| R3-Q1 | Is there an implementation that does the sweeper's job now but supports multi-replica later? Should we adopt it now, or must we do the naive sweeper first? | R4-1 |
| R3-Q2 | Should we maintain a stored `last_step` field for faster reads? Want the trade-off before ruling | R4-2 |
| R3-Q3 | "Fan-out is very hard under this data structure; DAG would need a whole new one." Is that assumption wrong? What are the design options? Market use may need fan-out | R4-3 |
| R3-Q4 | Besides a claiming algorithm or a load balancer in front, what other solutions exist for two orchestrators claiming the same run? | R4-1 |
| R3-Q5 | Should `runs.owner_id` be required? Required is complex; not required means a later schema change | R4-1 |
| R3-Q6 | Planner context: two-phase HTTP conversation (planner holds state) vs. planner calling back to an orchestrator read endpoint. Owner prefers the latter. What is actually *in* the orchestrator→planner message? Trade-off between "planner assembles the whole input" and "orchestrator assembles from the planner's references"? | R4-4 |
| R3-Q7 | What is `retry_after_seconds` (B5), and isn't it the same thing as per-step retry overrides (B6)? | R4-5 |
| R3-Q8 | Recovery (C1/C2/C4) must state explicitly what it **can** and **cannot** handle | R4-6 |
| R3-Q9 | Replay: if a replay exhausts the budget again and returns to DLQ, is it the same `run_id`? Can it be replayed again? Where exactly does replay idempotency bite? | R4-7 |
| R3-Q10 | Manual step re-run / output rewrite — same `run_id` or not? | R4-8 |
| R3-Q11 | Cancellation — how is it stored without adding states or breaking existing logic? | R4-9 |
| R3-Q12 | What are G2 (submission-time validation) and G4 (YAML/JSON boundary)? | R4-10 |
| R3-Q13 | Timeout / heartbeat / backoff / configurable limits — give several combinations and let the owner order them | R4-11 |

---

## Round 4 — Claude's analysis (input to SPEC; not itself spec)

### R4-1 · Lease-based claiming replaces both the sweeper and startup recovery

**Finding: the multi-replica-safe design is *simpler* than the single-replica one, not harder.**

Startup recovery and the sweeper are the same operation — "find runs that are RUNNING but not being driven, and drive them." Under one process you can approximate "not being driven" locally. Under N processes you cannot. The standard fix removes the approximation entirely:

```
runs.owner_id         TEXT        NULL   -- which orchestrator instance drives this run
runs.lease_expires_at TIMESTAMPTZ NULL
```

Claiming is one atomic statement, correct for N=1 and N=many alike:

```sql
UPDATE runs SET owner_id = :me, lease_expires_at = now() + :lease
WHERE status = 'RUNNING' AND (owner_id IS NULL OR lease_expires_at < now())
RETURNING run_id
```

Consequences:
- **Startup recovery stops being a separate code path** — it is just the first sweep.
- **No load balancer is needed.** Claiming *is* the load balancer (work-stealing): whoever has capacity claims. A balancer in front would be strictly worse — it needs liveness knowledge, and a dead replica's runs stay stranded until membership converges.
- **Fencing:** every state-writing transaction carries `AND runs.owner_id = :me`. Zero rows affected ⇒ this goroutine lost the lease ⇒ it stops. This is the same mechanism cancellation needs (R4-9) — one primitive, two uses.
- **Renewal is one UPDATE per interval for all owned runs**, not per run.
- `owner_id` is **nullable**: NULL means unclaimed. The "required vs. not" worry dissolves.
- Portable: a conditional UPDATE exists in every relational database, so it does not violate the storage-abstraction rule.

Alternatives considered and rejected: Postgres advisory locks (backend-specific — violates the abstraction rule); consistent hashing / balancer (needs membership, strands work on death); external queue (second source of truth); leader election (no horizontal scale).

### R4-2 · `last_step`: derive it, do not store it

- Derived: `SELECT status FROM steps WHERE run_id=? ORDER BY seq DESC LIMIT 1` on index `(run_id, seq)` — an index seek, sub-millisecond, and read once per sweep rather than per request.
- Stored pointer: saves microseconds, and introduces the possibility that the pointer and reality disagree. **The combination table is the entire recovery design; a stale pointer turns it into a lie.**
- It is a pure function of immutable data, so if profiling ever justifies it, it can be materialized later and backfilled safely. Nothing is foreclosed.

**Recommendation: derive.**

### R4-3 · Fan-out — the owner's assumption is half right

Correct half: a **full DAG** does need a new structure. `seq` as a total order becomes meaningless, the recovery branch key (`last_step`) becomes undefined, and the planner contract must grow dependency edges. That is a rewrite of the loop, not an addition.

Wrong half: **fan-out in general does not.** What actually blocks it is one sentence in the design — "the recovery unit is *the step* at max seq." Change it to "the recovery unit is *the set of steps* at max seq" and today's serial system is simply the case where every set has exactly one member.

| Option | Shape | Cost now | Cost later | Ceiling |
|---|---|---|---|---|
| **1. Serial forever** | as today | 0 | — | No cross-step parallelism ever |
| **2. Parallel group at one `seq`** | steps carry `seq` + `group_index`; the frontier advances when *all* members at max seq are DONE | **0 if the recovery unit is defined as a set from day one**; the schema gains one nullable column | Moderate: dispatch N, wait for all, per-member attempts already work unchanged | No cross-group dependencies; a group is atomic in the frontier |
| **3. Full DAG** | `steps.depends_on[]`, no `seq`, frontier = steps whose deps are DONE | High | High | None |

**Recommendation: adopt Option 2's *definition* now (recovery unit = the max-`seq` set), implement it later, and treat Option 3 as a different product.** This costs nothing today and buys the fan-out option outright. It covers the realistic cases — map over N items, call 3 models in parallel — which is most of what "fan-out" means in AI pipelines.

### R4-4 · Planner context and who assembles the worker's input

**Two-phase conversation vs. planner-fetches: the owner's preference (planner fetches) is right, and for a stronger reason than stated.** A two-phase HTTP conversation forces the planner to be *stateful* — it must know whether it is answering "here is the catalog" or "here is the next step". Every stateful planner then needs its own crash story. The fetch model keeps the planner a pure function: one request in, one decision out; anything it wants, it pulls.

It is also not really "another port" — it is the same read API the operator uses.

Costs to state plainly:
- The planner needs network access **back** to the orchestrator. Trivial for the built-in static planner; a real deployment constraint for a third-party planner behind a firewall.
- A planner failing because *the read API* was unreachable is indistinguishable, from the orchestrator's side, from a planner that is simply broken. Both consume planner budget. Diagnosis comes from the recorded per-attempt error text.
- Optional later mitigation: inline outputs below some size into the catalog so simple planners never need to fetch. This is the old 2 KB rule reborn — but as an optimization, never a requirement.

**Message shapes.**

Orchestrator → planner:
```json
{
  "run_id": "...",
  "workflow_input": { },
  "history": [
    {"step_id":"...", "name":"ocr", "seq":1, "status":"DONE",
     "output_bytes":14203, "completed_at":"..."}
  ],
  "fetch_base_url": "http://orchestrator:8080"
}
```

Planner → orchestrator: `{"status":"continue","step":{…}}` | `{"status":"done"}` | `{"status":"fail","reason":"…"}`

**Who assembles the worker's input — the trade-off the owner asked for:**

| | Planner assembles (literal payload in the StepSpec) | Orchestrator assembles (planner supplies references) |
|---|---|---|
| Planner must fetch past outputs | Always — to copy them forward | Only when it wants to *reason* about them |
| Planner→orchestrator message size | Large; carries the whole payload | Small; carries references |
| Duplication in the database | The same bytes stored again inside `steps.decision` | References only |
| Can transform data between steps | Yes, fully | No — the worker receives prior outputs as they were |
| LLM planner token cost | High | Low |
| Who is at fault for wrong worker input | The planner | Split: planner chose the references, orchestrator assembled |

**Recommendation: both, because they compose and neither is expensive.** `params` = the planner's literal values (small, config-like). `input_from` = references for bulk data, defaulting to *all completed steps*. A planner that genuinely needs to reshape bulk data fetches it and inlines it into `params` — so no capability is missing, and the cheap path is the default path.

### R4-5 · `retry_after_seconds` (B5) is not the same as per-step overrides (B6)

Three distinct knobs that the old documents kept conflating — this is exactly the "unclear responsibility split" the owner diagnosed:

| Knob | Question it answers | Who sets it | When |
|---|---|---|---|
| **retry limit (X)** | How many failures before DLQ | User (workflow, later step/run) | At config time |
| **attempt timeout** | How long may one attempt live before it is declared failed | User (workflow, later step/run) | At config time |
| **retry delay** | How long to wait after a failure verdict before the next attempt | User default (+ `retry_after_seconds`: the *worker* asks at runtime for a longer wait, e.g. it was rate-limited; the system takes the larger of the two, never the smaller) | Per failure |

B6 is the first two rows; B5 is the third row's runtime override. Different axes, different actors.

### R4-6 · What recovery can and cannot do (must be stated explicitly in SPEC)

**Handles:**
1. Orchestrator killed at any instant — in-flight runs resume; completed steps never re-run.
2. A single run's driver dying (storage blip, panic) while the process lives — reclaimed by the next sweep.
3. A crash *during* recovery — re-entrant, no double-counting (an already-claimed attempt is no longer RUNNING).
4. A crash loop — each pass burns one unit of retry budget, so every in-flight step converges monotonically to DLQ. Unbounded retry is structurally impossible.
5. Storage unreachable at startup — fail fast, non-zero exit, an error message that names storage as the cause.
6. Storage unreachable at runtime — runs orphan intact, reclaimed when storage returns.

**Does not handle — these are non-guarantees, and must be published as such:**
1. **Duplicate worker execution.** A re-dispatch may re-run a step that actually completed but whose result was never persisted. The worker must be idempotent on `step_id`. Worst case, up to X concurrent duplicates of one step.
2. **Work lost in flight.** A result computed but not persisted is gone and will be recomputed.
3. **Side effects already committed by the worker** (mail sent, payment made) — never rolled back.
4. **Planner non-determinism across a crash.** If the crash preceded persistence of the planner's answer, the planner is asked again and an LLM may answer differently. Legal by design: the "asked exactly once" guarantee covers *persisted* decisions only.
5. **Loss or corruption of the database itself.** Zero recovery — the DB is deliberately both the single source of truth and the single point of failure.
6. **Two orchestrators claiming the same run** — until leases are implemented.
7. **A worker that hangs with an effectively infinite timeout.** Nothing ever declares failure, so nothing ever recovers. This is why the default timeout must be finite.

### R4-7 · Replay idempotency — the real rule

The guard is **not** "has this run been replayed before". It is **"is this run in DLQ right now"**.

- Run currently in DLQ → replay proceeds; the transaction that takes it out of DLQ is itself the idempotency gate, so a double-click has exactly one winner.
- Run not in DLQ (RUNNING / DONE / CANCELLED) → 409, and the message states the actual current status.
- Therefore the owner's scenario resolves cleanly: replay → budget exhausted again → back to DLQ (a **second** DLQ row, **same `run_id`**) → **replay is allowed again**, as many times as it lands there. `run_id` never changes; `dead_letter_queue` accumulates history rows.

**Design correction to the old system:** it exposed `POST /dlq/{entry_id}/replay`, then needed a patch rule ("derive the worker-side/planner-side branch from current state, not from the entry") because an old entry and current reality diverge after multiple rounds. **Replay should target the run: `POST /runs/{run_id}/replay`.** The DLQ entry is history; the run is the thing you act on. This deletes the whole class of confusion.

### R4-8 · Manual step re-run / output rewrite

Same `run_id` — a run is the unit of history, and forking it would duplicate step history and break "one run, one story".

The hard part is not identity, it is **truncating the frontier**: to re-run step 3 of 6 you must make steps 4–6 stop counting.

| Option | Consequence |
|---|---|
| Delete the later steps | Destroys the audit trail. Rejected |
| Add `steps.superseded_at`, exclude superseded steps from history and from `last_step` | Works; adds one concept |
| Do not support it in v2 core — support only "replay the DLQ'd step", which needs no truncation because the DLQ'd step *is* the last step | Free |

**Recommendation: the third now, the second later if a real need appears.** Manual output rewrite (an UPDATE on a DONE step's output) is a separate, easier feature and also low priority.

### R4-9 · Cancellation, stored

Following the owner's ruling: `CANCELLED` is added to run and step. New legal combinations: `run=CANCELLED, last_step=CANCELLED` and `run=CANCELLED, last_step=DONE` — both untouched by recovery.

The open sub-question is the **attempt** underneath a cancelled step. Proposal: `attempt → FAILED(reason='cancelled')`, and **`attempt_count` is not incremented** — cancellation is not the worker's failure, and the step is terminal anyway, so the budget is irrelevant.

Implementation note: cancel races the driver goroutine. It resolves with the fencing predicate from R4-1 — cancel is one transaction conditioned on `run.status='RUNNING'`; the driver's next state write finds zero rows affected and exits. **One mechanism (fencing), three uses: lease hand-off, cancellation, and stale-writer suppression.**

Open: should a run in DLQ also be cancellable, giving DLQ an exit other than replay? Owner said RUNNING only; worth re-checking.

### R4-10 · What G2 and G4 were

**G2 — submission-time validation.** Reject a bad workflow definition at `POST /workflows` with a 400, instead of discovering it mid-run. The failures it catches: `planner_type: "htp"` (typo); `planner_type=http` with no URL; an unknown key like `retrylimit` silently ignored so the user believes a setting took effect; `retry_limit: "3"` as a string silently coerced to 3. The old project's governing principle, worth keeping: **better to be too strict and have users complain we do not support something, than too lax and let them fail silently.** Anything decidable at submission time must not be deferred to run time, where it costs budget, produces DLQ entries, and wastes triage.

**G4 — file format vs. wire format.** Config *files* on disk are YAML (comments allowed — worth a lot for demos and for a portfolio reader); the HTTP API body is JSON; the database stores JSON. The old repo shipped `.yaml` files whose contents were actually JSON — an extension lying about its content. The rule is one line: no file whose extension disagrees with its content.

### R4-11 · Timeout / liveness — combinations to choose from

Mechanisms available:

- **(a)** Finite default timeout + user-configured timeout per workflow (later per step/run). The user estimates how long the work takes.
- **(b)** Worker **heartbeat** extends the deadline: the worker pings `POST /tasks/heartbeat {step_id, attempt_id}` every N seconds; the effective deadline becomes `last_heartbeat + grace`.
- **(c)** Worker-requested retry delay (`retry_after_seconds`) — for rate limits.
- **(d)** Exponential backoff on the retry delay.
- **(e)** Progress payload on the heartbeat (`{"progress":0.4,"note":"page 12/30"}`), stored for observability.

| Combination | Contents | Time to detect a dead worker | Cost |
|---|---|---|---|
| **Punch 1** | (a) only | Up to the full configured timeout — for a 3-hour job, 3 hours | zero |
| **Punch 2** | (a) + (b) | ~grace (e.g. 90s), **independent of task length** | small: one endpoint, one column, dynamic deadline |
| **Punch 3** | Punch 2 + (e) | same, plus visible progress | small increment |
| **Punch 4** | Punch 2 + (c) + (d) | same, plus rate-limit friendliness | medium |

**Structural finding that outranks all of the above — persist `attempts.deadline_at`.**

The old design kept each attempt's timer **in process memory only**. That single choice is what forced the `orphaned` failure reason to exist: when the process died, the timer died with it, so nothing in the world would ever pronounce that attempt failed, and recovery had to claim it by hand.

Store the deadline as a column instead, and:
- **Timeout and orphan-claiming become one mechanism**: any live orchestrator's sweep expires any attempt whose `deadline_at` has passed. `orphaned` survives only as a diagnostic label ("was its owner alive?"), never as a separate mechanism.
- Timeouts are enforced correctly across **multiple replicas** — impossible with an in-memory timer.
- Heartbeats become an `UPDATE attempts SET deadline_at = now() + grace` — nothing else changes.
- The frontier's liveness stops depending on any process staying alive.

Refinement worth taking: on claiming a run, an orchestrator may expire a **sync**-mode attempt immediately (its HTTP connection died with the previous owner, so no report can ever arrive) but should let an **async**-mode attempt run to its deadline (its callback can still arrive and be honoured by the new owner). `connection_mode` is already in the persisted decision, so the rule costs nothing — and it recovers work the old design threw away.

---

## Round 4 — Rulings (2026-08-09)

| # | Topic | Ruling |
|---|---|---|
| R4-a | Persist `attempts.deadline_at`; timeout and orphan-claiming become one mechanism | **Adopted** |
| R4-b | Lease-based claiming | **Adopted in principle**, conditional on understanding lease semantics, fencing, and how cancellation uses it (explained in Round 5) |
| R4-c | Timeout is no longer exact | **Accepted trade-off — must be written down.** A timeout is a *lower bound* on when failure is declared, not an exact instant |
| R4-d | Parallel-group / fan-out pre-definition | **Leaning against pre-committing.** Owner's objection, which is correct: a same-`seq` group only buys "parallel single steps at one level", not a real DAG, and the model assumes branch depth is exactly one worker step. Decision: **write the definitions on the non-fan-out architecture**, with an explicit note that DAG may come later and its design is unexplored. No `group_index` column |
| R4-e | DAG vs. LLM planners | Owner's read — a real DAG suits a static planner, not an LLM one — is essentially right (see R5-3) |
| R4-f | Replay targets the run, not the DLQ entry | **Adopted, with its limitation written down:** if round 1 failed at step 1 and round 2 failed at step 2, replay resumes from step 2 and never revisits step 1. Accepted because it is unambiguous. **Requirement: every replay round must leave an inspectable record** |
| R4-g | Two-table discipline | `runs` / `steps` hold **current status**, one row each, mutated in place. `dead_letter_queue` is **append-only history**, never modified, many rows per run |
| R4-h | Manual step re-run / output rewrite | **v2 core supports only "replay the step that is in DLQ"** — i.e. the run resumes from its furthest successful point. Because a replay always resumes at the latest reached step, "round 2 fails earlier than round 1" cannot arise. This is a convenience-driven scope cut, not a principle; revisit if a real need appears |
| R4-i | Cancelling a step whose attempt already finished | If the attempt is DONE, the step is DONE — leave both alone. Only `run` becomes CANCELLED |
| R4-j | G2 submission-time validation / G4 format boundary | **Both accepted.** Disk config = YAML; HTTP body = JSON; DB = JSON. No file whose extension disagrees with its content |
| R4-k | Recovery non-guarantees | Accepted, **plus one more to state explicitly: recovery never auto-replays a DLQ'd run.** Reaching DLQ means a human decides |
| R4-l | Cancelling a run that is in DLQ | **Allowed.** Exact state combination to be settled — see R5-5 |
| R4-m | Heartbeat (punch 2) | **Deferred.** Not strictly necessary; progress reporting and exponential backoff likewise go to the later list |
| R4-n | Raw (un-enveloped) worker body | **Both `sync` and `async` must eventually support it**, so that a worker already written in its own style needs no modification at all. They may land at different times. **`sync` raw is the priority** — async raw would force the orchestrator to conform to the worker's own callback shape, which is complex; the alternative is that the user writes a small adapter. Record both paths now |
| R4-o | Project name | **Piton.** New repository |

---

## Round 5 — Claude's analysis (input to SPEC; not itself spec)

### R5-1 · What the sweep now does, and why timeout stops being exact

The sweep carries **two** jobs that used to be separate: claiming unowned runs, and expiring attempts whose `deadline_at` has passed.

**Consequence to publish:** a timeout is a *lower bound*, not an exact instant. An attempt with a 60s timeout is declared failed somewhere in `[deadline, deadline + sweep_interval]`.

But this only applies when nobody is actively driving the run. Refinement worth keeping:

- **Owner alive** → the driver goroutine is already blocked waiting for the response, so it enforces its own in-memory timer and declares failure **precisely** at the deadline.
- **Owner dead / never claimed** → the sweep is the backstop, bounded by the sweep interval.

So the in-memory timer is a *latency optimisation* and the persisted `deadline_at` is the *correctness backstop*. Two mechanisms, clearly ranked, neither load-bearing for the other. State it that way, because "which one is authoritative?" is exactly the kind of ambiguity that produced the old documents' mess: **the database is authoritative; the in-memory timer only makes the common case fast.**

### R5-2 · Why a lease, and what fencing actually is

**Why ownership must expire.** If `owner_id` were set with no expiry, a dead orchestrator would own its runs forever and no one could take them — you would then need something to detect its death and clear the field, which is the original problem again. A lease is **ownership that decays unless renewed**, so renewal *is* the liveness signal. (Note the symmetry: a lease renewal is the orchestrator's heartbeat, exactly as a worker heartbeat would be the worker's.)

Length is a straight trade-off: shorter lease = faster failover, more renewal writes. 30s lease renewed every 10s is a reasonable default. Renewal is **one** UPDATE covering every run this instance owns, not one per run.

**Fencing** = every state-writing transaction asserts, in the same statement, that the writer still holds the run:

```sql
UPDATE steps SET status='DONE', output=:out
WHERE step_id=:sid
  AND EXISTS (SELECT 1 FROM runs
              WHERE run_id=:rid AND status='RUNNING'
                AND owner_id=:me AND lease_expires_at > now());
```

Zero rows affected ⇒ I no longer have the right to write ⇒ abort the transaction, stop the goroutine, dispatch nothing further. No inter-process coordination, no signalling, no killing threads from outside: **a writer discovers it has been superseded at its next write.**

**Cancellation is the same primitive.** Cancel is one transaction:

```sql
UPDATE runs SET status='CANCELLED', owner_id=NULL
WHERE run_id=:rid AND status IN ('RUNNING','DLQ');
-- plus: last step RUNNING → CANCELLED, its RUNNING attempt → FAILED('cancelled')
```

Once it commits, the old driver's next write hits the fence and the goroutine exits on its own. Cancelling the driver's Go `context` as well is a *speed* optimisation, never the correctness mechanism.

**Non-guarantee this creates, and it must be published:** cancellation stops the orchestrator from advancing the run; it does **not** abort work already handed to a worker. A worker dispatched moments before the cancel keeps running to completion, and its report is simply refused. "Cancelled" means "this run will make no further progress", not "everything stopped instantly".

### R5-3 · Fan-out and DAG — answering the owner's objection

**How `group_index` would have worked:** steps carry `(seq, group_index)`; all steps sharing a `seq` are dispatched together; the frontier advances — i.e. the planner is asked again — only once **every** member at max `seq` is DONE. Recovery's branch key becomes "the set of steps at max `seq`": any member RUNNING ⇒ the claim path; all DONE ⇒ ask the planner; any DLQ ⇒ the run is in DLQ.

**The owner's objection is correct.** That model buys a fan-out/fan-in **barrier**, not a DAG. Every branch is exactly one worker step deep, and branches cannot depend on one another.

**On DAG vs. LLM planners** — the owner's instinct is right, and the reason is worth recording: a real DAG is a *graph declared up front*, which is inherently static-planner territory. Asking an LLM to emit a full dependency graph contradicts this system's entire thesis, which is deciding **one step at a time** with full knowledge of everything that already happened. What an LLM *can* naturally emit is "these three things can run now" — which is the group model, not the DAG model. So if fan-out ever lands, the group model is the LLM-compatible shape and a DAG is a different product.

**Decision taken:** no `group_index`, no pre-commitment. One free hedge is still on the table — phrasing the recovery rule as "the frontier is the set of steps at the maximum `seq`, and in v1 that set always has exactly one member." That is wording only: no schema, no code, no test. Owner to rule.

### R5-4 · Replay rounds must be inspectable

The owner requires that every replay round leave a record a human can check. Two ways:

1. **Reconstruct** — bucket attempts by `created_at` against the timestamps of the `dead_letter_queue` rows. Free, and what the old design chose. Awkward to do by eye in a terminal.
2. **`runs.replay_count`**, one integer, incremented inside the replay transaction. Trivially inspectable; makes "which round was this?" a column instead of a puzzle. Cost: one column, one increment, no new concept.

Recommendation: **take (2)**. It directly serves the owner's requirement that he can sit at a terminal and see what happened.

### R5-5 · The complete run × last_step table (this belongs in SPEC, prominently)

**Convention that collapses the table:** *a run with no steps is defined to have `last_step = DONE`.* It behaves identically to a completed last step everywhere — the planner is asked next — so it needs no separate row. This single sentence removes four cells.

**Two cells the old design never listed, found in this audit:**
- `run=DONE, last_step=DONE(none)` — the planner answers `done` on its very first call. An empty pipeline is legal.
- `run=DLQ, last_step=DONE(none)` — the planner answers `fail`, or exhausts its budget, before any step exists.

Both are already covered once "no steps ⇒ last_step=DONE" is adopted. Without that convention they are holes.

**Legal combinations — 8, and recovery only ever performs one of two actions:**

| # | run | last_step | Meaning | Action when claimed |
|---|---|---|---|---|
| L1 | RUNNING | DONE | Waiting on a planner decision (includes the no-steps case) | **Ask the planner** |
| L2 | RUNNING | RUNNING | Waiting on a worker, or its owner died | **The claim path:** expire the overdue/orphaned attempt → budget check → re-dispatch, or it already went to DLQ inside the claim |
| L3 | DONE | DONE | Cleanly finished | Never scanned |
| L4 | DLQ | DLQ | Worker-side DLQ | Never scanned. Exit: replay, or cancel |
| L5 | DLQ | DONE | Planner-side DLQ | Never scanned. Exit: replay, or cancel |
| L6 | CANCELLED | CANCELLED | Cancelled while a step was in flight | Never scanned |
| L7 | CANCELLED | DONE | Cancelled while waiting on the planner, or cancelled out of a planner-side DLQ | Never scanned |
| L8 | CANCELLED | DLQ | Cancelled out of a worker-side DLQ; the step keeps its DLQ verdict as a historical fact | Never scanned |

**Impossible combinations, and why each is impossible:**

| run | last_step | Why it cannot exist |
|---|---|---|
| RUNNING | DLQ | `step→DLQ` and `run→DLQ` are one transaction; no instant separates them |
| RUNNING | CANCELLED | `step→CANCELLED` occurs only inside the cancel transaction, which sets `run→CANCELLED` in the same commit |
| DONE | RUNNING | `run→DONE` is produced only by the planner answering `done`, whose precondition is `last_step=DONE` |
| DONE | DLQ | same |
| DONE | CANCELLED | same |
| DLQ | RUNNING | Worker-side DLQ sets both in one transaction; planner-side DLQ requires `last_step=DONE` |
| DLQ | CANCELLED | Cancel always sets `run=CANCELLED`, never `run=DLQ` |
| CANCELLED | RUNNING | The cancel transaction terminates a RUNNING last step in the same commit |

**The uniform cancel rule** (one sentence, covers all three cancelled combinations without special cases):

> `run → CANCELLED`. If `last_step = RUNNING`, then `last_step → CANCELLED` and its RUNNING attempt → `FAILED(cancelled)` with `attempt_count` **unchanged**. Otherwise the last step keeps the terminal state it already had (DONE or DLQ). A DLQ verdict is a historical fact and is never rewritten.

### R5-6 · Heartbeat, correcting a misunderstanding

The heartbeat proposed is **worker → orchestrator push**: the worker calls `POST /tasks/heartbeat {step_id, attempt_id}`, and the orchestrator does `UPDATE attempts SET deadline_at = now() + grace`. **The orchestrator never polls workers**, and it is not a database poll either.

The alternative — the orchestrator polling a status endpoint on each worker — requires every worker to expose one and costs one request per in-flight attempt per interval. Push is cheaper and puts the obligation on the party that actually knows whether it is still working. Either way it requires worker cooperation, which is why it is deferred.

### R5-7 · Where the backlog lives

Options: a section in `CLAUDE.md`, a separate `BACKLOG.md`, or the final milestone.

**It must not be the final milestone** — a milestone is scheduled work, and backlog items are by definition unscheduled. Labelling them as a milestone quietly promises they will be built.

Recommendation: **`CLAUDE.md § Backlog (unscheduled — not specification)`**, keeping the two-file rule intact. `CLAUDE.md` is already the "what to do next" document, so it is the natural home. Split it into its own file only if it outgrows roughly fifty items.
*(Superseded by R5 ruling: milestones live in `SPEC.md`, unscheduled work lives in `BACKLOG.md`, `CLAUDE.md` holds neither.)*

---

## Round 5 — Rulings (2026-08-09)

| # | Topic | Ruling |
|---|---|---|
| R5-a | Fan-out definition | **Do not touch the current definition at all**, not even in wording. The "frontier is a set" phrasing moves into the backlog as part of the fan-out item |
| R5-b | Fan-out vs. DAG are two separate backlog items | **Fan-out** = an extension of the current model. **DAG** = a new, independent static mode. Both provisional and revisable |
| R5-c | Worker heartbeat | **Dropped entirely**, not merely deferred |
| R5-d | `runs.replay_count` | **Adopted** |
| R5-e | Where milestones and backlog live | **Milestones live in `SPEC.md`** as the planned sequence. **Unscheduled work lives in `BACKLOG.md`.** `CLAUDE.md` holds neither |
| R5-f | Milestone order (final) | α → γ → β → δ → **ζ (custom HTTP/LLM planner, pulled forward to sit right after replay)** → θ (sync raw body) → ε (async envelope) → η (per-step/run overrides) → ι (cancel + submission-time validation) |
| R5-g | Project name | **Piton**, confirmed after an occupancy check: no collision inside workflow orchestration; collisions exist outside it (OpenPiton at Princeton — hardware; a dormant `piton` package on PyPI; Piton Labs — a dev shop) |
| R5-h | Retention / purge | Not required for sweep correctness (see R6-3). Offered as a housekeeping backlog item only |

### Owner's questions carried into Round 6

| # | Question | Where answered |
|---|---|---|
| R5-Q1 | How exactly does the sweep find an unowned run or a timed-out attempt — is it scanning the DB rather than some in-process registry? And what actually causes a goroutine to go missing? | R6-1 |
| R5-Q2 | If the lease is not refreshed on every state write, a long run's lease expires, a different orchestrator claims it — but the planner's and worker's traffic still goes to the old one. Doesn't that make any run longer than the lease effectively an orphan? Would the new owner re-dispatch work that was already done? | R6-2 |
| R5-Q3 | Since DONE/CANCELLED/DLQ runs stay in the same table, won't the sweep scan more and more rows until it explodes? Is a user-run purge the answer? | R6-3 |

---

## Round 6 — Claude's analysis (input to SPEC; not itself spec)

### R6-1 · What the sweep scans, and how a driver goes missing

The sweep reads **the database only**. It has no in-process registry of live goroutines, and deliberately so: an in-process registry cannot see other replicas, and it is lost on restart. `owner_id` + `lease_expires_at` **are** the database-visible representation of "is anyone driving this", which is why they must exist as columns rather than as memory.

A driver goroutine disappears without the run reaching a terminal state whenever any of these happen:

- a storage write fails and the driver returns the error (the old design's explicit rule was that the driver dies here);
- a panic anywhere in the driver — a bug parsing a planner response, a nil dereference;
- the process is killed: deploy, rolling restart, OOM kill, `docker compose down`, machine reboot;
- a network partition to the database that outlasts the driver's patience.

None of these are exotic. "The goroutine mysteriously vanished" is really "any error path that returns before the run is terminal", and there are always more of those than anyone enumerates — which is precisely the argument for a database-level backstop rather than careful goroutine bookkeeping.

### R6-2 · Lease renewal — answering the owner's strongest objection so far

**The premise in the question is wrong, and the correction matters: the lease is NOT renewed as a side effect of writing run state. It is renewed by a dedicated ticker, unconditionally, whether or not the run is making progress.**

```sql
-- every ~10s, one statement covering every run this instance owns
UPDATE runs SET lease_expires_at = now() + interval '30 s'
WHERE owner_id = :me AND status = 'RUNNING';
```

Renewal is a **liveness signal, not a progress signal**. A run sitting on a single three-hour worker call keeps its lease the entire time, because the process holding it is alive and ticking. So "any run longer than the lease becomes an orphan" does not happen. This distinction — liveness versus progress — is the whole reason the lease is a separate concept from the state writes.

**What happens in the case the owner was really worried about** — the owner process is alive but stalled past its lease (long GC pause, VM suspend, database partition):

1. Another orchestrator claims the run.
2. The stalled owner wakes and tries to write. Its transaction carries `AND owner_id=:me AND lease_expires_at > now()`, so it affects zero rows. It aborts and stops. **Correctness holds without any communication between the two processes.**
3. The new owner re-dispatches. **Yes — the worker may perform the same work twice.** That is the published at-least-once contract, absorbed by worker idempotency keyed on `step_id`. It is the same duplication a crash already causes; a stalled owner is simply a crash that came back.

**The real problem hiding inside the question, which the owner half-found: where do async callbacks go?**

In async mode the worker POSTs back to a callback URL. With several replicas behind a load balancer, that callback can land on a replica that does not own the run. The old design made this fatal by construction: the callback handler pushed the result into an **in-memory channel** owned by the driver goroutine. On the wrong replica there is no channel, so the result is lost.

The fix follows from what has already been decided. Once `deadline_at` is a column and every write is CAS-guarded and fenced, the database — not the process layout — arbitrates who wins. So:

- **the callback handler applies the report directly, under CAS + fencing.** Any replica can accept any callback.
- **the driver awaiting an async result polls the attempt row** (say once a second) instead of blocking on an in-memory channel.

This deletes, all at once: the channel registry, the rule that the timer must scrub the registry when it fires, the "single writer" rule's process-level enforcement, and the wrong-replica callback loss. Polling costs one cheap indexed query per second per in-flight async attempt, which for this system's workload is nothing. An in-process fast path can be added later as an optimisation — never as the mechanism.

**The unifying principle this produces, and it should open the SPEC:**

> **The database is the only coordination mechanism. Every in-memory structure is a cache that may vanish at any instant without affecting correctness.**

Three consequences, all already decided independently, now visibly the same idea:
- the attempt deadline lives in a column; the in-memory timer is only a latency optimisation;
- run ownership lives in a column with an expiry; process identity means nothing on its own;
- an async result is delivered through a row; a channel is only a latency optimisation.

### R6-3 · The sweep does not degrade as history accumulates

The sweep predicate is `status='RUNNING' AND (owner_id IS NULL OR lease_expires_at < now())`. With a **partial index on `runs(status) WHERE status='RUNNING'`**, terminal runs are not in the index at all — a DONE run from six months ago costs literally nothing. Cost is proportional to the number of **currently running** runs, which is bounded by concurrency, not by history.

For a backend without partial indexes, a plain index on `status` still seeks straight to the RUNNING range; the cost is the same order. So this does not violate the storage-abstraction rule.

**Therefore no purge is needed for correctness or for sweep performance.** Retention remains worth offering eventually for disk usage, and belongs in the backlog as housekeeping — not as something a user must do to keep the system healthy.

One rule the sweep must state explicitly: it filters on `status='RUNNING'`, so CANCELLED, DONE and DLQ runs are never claimed. Cancellation's `owner_id = NULL` is therefore belt-and-braces, not the mechanism.

---

## Round 7 — Claude's analysis (input to SPEC; not itself spec)

### R7-1 · The lease, specified properly (owner was right that it was never defined)

The earlier sketch put `lease_expires_at` on `runs` and renewed it with a ticking UPDATE across every owned run. That is workable but bad: it rewrites every RUNNING run row every ten seconds, producing continuous dead-tuple churn on the busiest table in the system, and it blurs "run state" with "coordination metadata".

**Better design — liveness belongs to the orchestrator, not to the run.**

```sql
orchestrators (
  orchestrator_id  TEXT PRIMARY KEY,      -- generated once at process boot
  started_at       TIMESTAMPTZ NOT NULL,
  last_seen_at     TIMESTAMPTZ NOT NULL   -- the only column a heartbeat touches
)

runs.owner_id   TEXT NULL REFERENCES orchestrators(orchestrator_id)
runs.claimed_at TIMESTAMPTZ NULL
```

- The heartbeat is **one row per process**, not one per run: `UPDATE orchestrators SET last_seen_at = now() WHERE orchestrator_id = :me`, every ~10s. O(1) regardless of load.
- An orchestrator is **live** iff `last_seen_at > now() - lease_ttl` (default 30s).
- `runs.owner_id` changes only on **claim** and **release** — meaningful, rare events, never a ticking write.
- Claim (one atomic statement, so two sweeps cannot both win):

```sql
UPDATE runs r SET owner_id = :me, claimed_at = now()
WHERE r.status = 'RUNNING'
  AND (r.owner_id IS NULL
       OR NOT EXISTS (SELECT 1 FROM orchestrators o
                      WHERE o.orchestrator_id = r.owner_id
                        AND o.last_seen_at > now() - :ttl))
RETURNING r.run_id;
```

**The fence becomes simpler than the earlier sketch.** Every business transaction opens with:

```sql
SELECT 1 FROM runs WHERE run_id = :rid AND owner_id = :me FOR UPDATE;   -- 0 rows ⇒ abort
```

No expiry check is needed here: if this process stalled and someone else claimed the run, `owner_id` is no longer `:me` and the fence fires. If nobody claimed it, this process is still the rightful owner and proceeding is correct. `FOR UPDATE` takes the row lock, which serialises the fence against a concurrent claim — without it, a claim committing mid-transaction could leave two drivers briefly believing they own the run.

**Carve-out that must be stated, or it becomes the next ambiguity:** the closed transaction ledger governs **business state** — run/step/attempt status, outputs, budgets. The coordination columns (`orchestrators.last_seen_at`, `runs.owner_id`, `runs.claimed_at`) are **not** business state and are written only by claim, heartbeat, and release. Without this sentence, "the heartbeat is a state write outside the ledger" reads as a violation.

### R7-2 · Async callback: who continues the run?

Scenario: orchestrator A dispatched the attempt; the worker's callback lands on orchestrator B.

**Ownership does not move. B applies the result; A continues driving.**

Rejected alternative — B steals ownership on callback — because it would fence A off mid-run for no reason, make ownership ping-pong with traffic routing, and hand runs to instances that never asked for them. Ownership must change only when the owner is absent or dead.

This requires one explicit exception to the fencing rule, and the distinction is principled:

| Kind of write | Requires ownership? | Guarded by |
|---|---|---|
| **Driving** — asking the planner, creating a step, dispatching, replaying, cancelling | **Yes** | the ownership fence |
| **Report application** — turning an arriving success/failure into attempt+step state | **No** | CAS on the attempt (`attempt_id = current_attempt_id AND status = 'RUNNING'`) |

The justification: a report is a **fact about work that happened**, not a **decision about what the run should do next**. Facts may be recorded by whoever receives them; decisions belong to the owner. CAS already guarantees exactly one writer per attempt outcome, so a non-owner cannot corrupt anything — and A learns about it on its next poll of the attempt row.

**This is also the answer to "how does A find out?"** — it is exactly why the driver polls the attempt row instead of waiting on an in-memory channel.

### R7-3 · Sweepers, plurality and atomicity

Yes — **every orchestrator instance runs its own sweep**; there is no designated sweeper. The owner's instinct is right that concurrent sweeps must not corrupt each other, and the claim is a single atomic `UPDATE … RETURNING` taking a row lock, so exactly one instance wins each run.

One simplification worth taking: **the sweep only claims. It never touches business state.** Expiring overdue attempts, checking budgets and re-dispatching are done afterwards by the driver that now owns the run, through the normal fenced path. This keeps "business writes require ownership" true with only the single report-application exception above, instead of scattering exceptions.

Consequently: an overdue attempt on a run owned by a *live* orchestrator is expired by that owner (its own timer or poll); `attempts.deadline_at` is the backstop consulted by a **new** owner at claim time.

### R7-4 · Required indexes are part of the storage contract

Because the storage layer is an abstraction with potentially several backends, "`runs.status` is indexed" is not an implementation footnote — a backend without it is not a conforming implementation. It belongs in a short **Storage requirements** section of SPEC (required indexes and the atomicity obligations), not scattered through the behavioural rules where it would be noise.

---

## Round 7 — Rulings (2026-08-10)

Context: the Round 7 exchange was lost and reconstructed from `./temp` (a truncated terminal
capture) plus this log. The analysis in R7-1…R7-4 above was intact; only the rulings were missing.

| # | Topic | Ruling |
|---|---|---|
| R7-a | Lease design | **The `orchestrators` liveness table (R7-1), not `runs.lease_expires_at`.** Heartbeat is O(1) — one row per process, not one per run. `runs` is not polluted by ticking writes. The fence simplifies to `SELECT 1 … FOR UPDATE` with no expiry check |
| R7-b | Storage JSON handling | **The storage interface takes and returns `[]byte`** — an opaque JSON document. The Postgres implementation stores `jsonb` internally; an implementer on another relational DB picks their own encoding (TEXT / BLOB / native JSON). Cost accepted: the Go layer gets no JSONB query capability (unused today) |
| R7-c | Repository landing | **SPEC first.** `git init`, moving `docs/` into `legacy/`, and `go.mod` all wait until SPEC is settled. Physical location is already `/home/aaronwu/Projects/piton` (the owner moved it; `/home/aaronwu/stateflowv2` no longer exists). Proposed Go module path `github.com/aaronwu001/piton` — not yet ruled |
| R7-d | Fencing exception for report application | **Narrow version adopted.** The exception exists **only** for an async callback landing on a non-owner, and it may write **only the `attempts` table** (that attempt's terminal state). All writes to `steps` and `runs` require ownership. The CAS predicate is `attempt_id = :aid AND status = 'RUNNING'`; no comparison against `steps.current_attempt_id` is needed, because a superseded attempt has already been moved out of `RUNNING` |
| R7-d′ | Priority of the multi-replica story | **Multi-replica orchestration is not a milestone.** It is the "must not be foreclosed" constraint of R2-8g, not something to build. R7-d's *rule* goes into SPEC now for exactly one reason: it determines the async delivery mechanism (the driver polls the attempt row rather than blocking on an in-memory channel), which would be awkward to retrofit. With a single replica the exception is never exercised |
| R7-e | Config field naming and defaults | **Adopted** (see table below). Additional requirement: `step_max_attempts` is a **total attempt count**, not a retry count — SPEC must say so explicitly |
| R7-f | Run-creation API | **Keep the `overrides` sub-object**; any non-empty value is a 400 until milestone η |
| R7-g | Planner message shapes / who assembles worker input / `output_field` | **All adopted.** (a) planner fetches from the read API rather than a two-phase conversation; (b) `params` *and* `input_from` coexist; (c) omitted `input_from` ⇒ the previous step only; (d) **`output_field` is cut** — the whole worker response body is stored verbatim as the step output |
| R7-h | run × last_step combination table | **Adopted wholesale** — L1–L8, the uniform cancel rule, and the "a run with no steps has `last_step = DONE`" convention. Goes into SPEC prominently. This discharges the R2-Q3 requirement that the table be settled before implementation starts |
| R7-i | `orchestrators` lifecycle | **No foreign key** — `runs.owner_id` is a plain string. **`orchestrator_id` is a UUID generated at process boot.** Dead-row cleanup is **backlog housekeeping**, not a correctness requirement |
| R7-j | SPEC opening principle | **Adopted:** *The database is the only coordination mechanism. Every in-memory structure is a cache that may vanish at any instant without affecting correctness.* |
| R7-k | Writing discipline for SPEC | **Definitions and architecture come first, written concisely and clearly**, so that later rules have unambiguous referents. A term that carries a definition (e.g. "StepSpec") may not appear before that definition |
| R7-l | Scope discipline | SPEC is written in full so nothing is forgotten and it can be picked up later — **but the focus stays on the early part: whatever is written must be demoable, testable, and runnable by hand.** Operative test for admitting anything now: **would adding it later be awkward?** If not, it goes to BACKLOG |

**Config fields adopted under R7-e:**

| Level | Field | Default | Meaning |
|---|---|---|---|
| workflow | `step_timeout_seconds` | 300 | Upper bound on one attempt |
| workflow | `step_max_attempts` | 3 | Total attempts before the step goes to DLQ (0 = no execution at all is *not* the meaning; SPEC must define the edge) |
| workflow | `step_retry_delay_seconds` | 0 | Wait between a failure verdict and the next attempt |
| workflow | `planner_timeout_seconds` | 30 | Upper bound on one planner call |
| workflow | `planner_max_attempts` | 3 | Planner failures before the run goes to DLQ |
| StepSpec | `timeout_seconds` / `max_attempts` | null | Override the two step-level values (fields present now, implemented in η) |

**Why two symmetric prefixes:** `step_*` and `planner_*` make it visible at a glance which knob
governs which actor. The old system's `retry_limit` / `RETRY_MAX_ATTEMPTS` / a hard-coded 30s was
precisely the "you cannot tell what governs what" failure.

---

## Round 8 — Claude's analysis (input to SPEC; not itself spec)

### R8-1 · Fencing and CAS are the same primitive (the owner asked; the earlier wording was bad)

The owner's objection — *"shouldn't it just be: check whether the run is still mine, and walk away
if it isn't?"* — is exactly right, and describes the actual behaviour. The earlier phrasing
("every transaction **asserts** it still holds the run") read as *claiming* possession rather than
*testing* it.

**Why it cannot be two separate steps.** A check followed by a write has a gap:

```sql
SELECT owner_id FROM runs WHERE run_id = :rid;   -- (1) still mine, good
                                                 -- (2) ← someone else claims it here
UPDATE steps SET status = 'DONE' WHERE ...;      -- (3) no longer entitled, but the write lands
```

That is TOCTOU (time-of-check to time-of-use). The requirement is therefore not "check" but
**"check and write, with no window in which the answer can change."** Two ways to get it:

- **(A) Put the condition inside the writing statement** — the original R4-1 / R5-2 shape:
  `UPDATE steps … WHERE step_id=:sid AND EXISTS (SELECT 1 FROM runs WHERE run_id=:rid AND owner_id=:me)`.
  No gap, but a business transaction touches `steps`, `attempts` and `runs`, so the predicate must
  be repeated in every statement — noisy, and **one forgotten statement breaks it**.
- **(B) Take a row lock once at the top of the transaction** — the R7-1 shape, and what we adopted:
  `SELECT 1 FROM runs WHERE run_id=:rid AND owner_id=:me FOR UPDATE`. `FOR UPDATE` makes a
  concurrent claim block on that row until we commit or abort, so **one statement protects the whole
  transaction** instead of relying on the author remembering.

**Definitions, stated once so they stop being confusing:**

- **CAS (compare-and-set)** is the general primitive: a write whose effect is conditional on the
  current state matching an expectation, evaluated atomically with the write. In SQL that is
  `UPDATE … WHERE <expected state>`; zero rows affected means the expectation was wrong.
- **Fencing** is *a specific use* of that primitive, where the condition is about **ownership**.
  The name comes from the distributed-systems "fencing token": the point of a fence is to keep a
  superseded process out.

**So: fencing IS CAS — the same mechanism with a different predicate.** The system has two
predicates, and the distinction is worth stating in SPEC exactly this way:

| | **Ownership fence** | **Attempt CAS** |
|---|---|---|
| Question the condition asks | "Am I still this run's owner?" | "Has this attempt's outcome not been written yet?" |
| Row it tests | the `runs` row | the `attempts` row |
| SQL | `WHERE run_id=:rid AND owner_id=:me` | `WHERE attempt_id=:aid AND status='RUNNING'` |
| What it prevents | two drivers making **decisions** about one run | one attempt's outcome being written twice (including a late report) |
| Meaning of zero rows | I am out — abort, stop the goroutine, dispatch nothing | someone already recorded this — discard the report, reply 409 |
| Who must satisfy it | every write that constitutes a **decision** | every write of an attempt outcome, owner or not |

R7-d's exception, restated in these terms: **an async callback landing on a non-owner checks only
the second predicate, not the first.** That is safe because the only row it can touch is one that
already admits exactly one winner.

**Storage-contract obligation this creates** (for the SPEC's Storage Requirements section): a
conforming backend must re-evaluate a blocked `UPDATE`'s `WHERE` clause after the lock is released.
Postgres does this under READ COMMITTED. Without it, a claim would decide using a stale snapshot.

### R8-2 · `dispatch_style` — the two modes, which had never been spelled out

| | `envelope` | `raw` |
|---|---|---|
| What the worker receives | Piton's own JSON envelope (`run_id`, `step_id`, `attempt_id`, `connection_mode`, `params`, `inputs`, and `callback_url` when async) | **only the payload — no Piton fields at all** |
| Must the worker be written for Piton? | Yes — it must parse the envelope | **No** |
| Reason it exists | Normal mode; full functionality | R3-3: **any unmodifiable HTTP endpoint is a valid worker** (an existing internal API, a third-party API) — you cannot add fields to its request body |
| How bulk data reaches it | `inputs`, assembled by the orchestrator from `input_from` | Only by the planner fetching it and inlining it into `params` — the body has no second field |
| Milestone | α onward | θ |

Mechanical consequence: **`async` + `raw` cannot work** — a raw body has nowhere to put
`callback_url`, and R4-n already noted that async raw would force the orchestrator to conform to the
worker's own callback shape. The user's alternative is a small adapter. → BACKLOG.

So there are **three** legal `connection_mode` × `dispatch_style` combinations, not four.

### R8-3 · The complete message catalogue

Planner calls are **always synchronous** — the planner is a pure function: one request in, one
decision out.

**M1 · orchestrator → planner**

```json
POST {planner_url}
{
  "run_id": "018f...",
  "workflow_input": { "doc_url": "s3://..." },
  "history": [
    { "step_id": "018f...", "step_name": "ocr", "seq": 1, "status": "DONE",
      "output_bytes": 14203, "attempt_count": 2, "completed_at": "2026-08-10T12:00:00Z" }
  ],
  "fetch_base_url": "http://orchestrator:8080"
}
```

`history` is a **catalogue only** — it never carries outputs; that is what `output_bytes` is for.
Because the planner is asked only in state L1 (`last_step = DONE`), every step in `history` is DONE.
That is not a coincidence; it follows from the combination table.
*(The presence of `history` at all is reopened in Round 9 — see R9-1.)*

**M2 · planner → orchestrator** — three responses. Note that `StepSpec` is **one field of the
response**, not the response itself:

```json
{ "status": "continue", "step": { ...StepSpec... } }
{ "status": "done" }
{ "status": "fail", "reason": "cannot determine the next step from the document" }
```

`StepSpec`:

```jsonc
{
  "step_name": "ocr",                     // optional display label, non-unique (R2-6)
  "worker_url": "https://...",
  "connection_mode": "sync" | "async",    // R2-4: every message carries it explicitly
  "dispatch_style": "envelope" | "raw",   // R3-3 / R4-n
  "params": { },                          // the planner's literal, config-like values
  "input_from": ["<step_id>", ...],       // omitted ⇒ the previous step; [] ⇒ nothing
  "timeout_seconds": null,                // override, implemented in η
  "max_attempts": null
}
```

**M3 · planner → orchestrator (fetch)** — the same read API the operator uses; see R8-b below.

**M4 · orchestrator → worker** — the three legal combinations:

```json
// M4a · sync + envelope (α)
POST {worker_url}
{ "run_id": "018f...", "step_id": "018f...", "attempt_id": "018f...",
  "connection_mode": "sync",
  "params": { "lang": "zh-TW" },
  "inputs": { "018f...": { "text": "..." } } }

// M4b · async + envelope (ε)
POST {worker_url}
{ "run_id": "018f...", "step_id": "018f...", "attempt_id": "018f...",
  "connection_mode": "async",
  "params": { }, "inputs": { },
  "callback_url": "http://orchestrator:8080/callbacks/018f..." }
```

```
// M4c · sync + raw (θ)
POST {worker_url}
{ "lang": "zh-TW" }        ← params, verbatim. No Piton fields whatsoever.
```

**M5 · worker → orchestrator (result)**

- **M5a · sync + envelope** — the HTTP response body:
  `{"status":"success","output":{ }}` or `{"status":"failure","error":"…"}`.
  A transport-level failure (non-2xx, timeout, connection refused) is **always** a failure
  regardless of body. A business-level failure and a transport-level failure **burn one attempt
  alike**.
- **M5b · sync + raw** — no envelope is available, so only HTTP speaks: 2xx ⇒ success and the
  **entire response body verbatim is the output**; non-2xx ⇒ failure, with the (truncated) body
  stored as error text.
- **M5c · async + envelope** — `POST /callbacks/{attempt_id}` with
  `{"attempt_id":"…","status":"success","output":{ }}`. The immediate HTTP response to the dispatch
  (M4b) means only "accepted": 2xx ⇒ the attempt enters RUNNING and awaits the callback; non-2xx ⇒
  **the attempt fails immediately** (the worker refused the job).

---

## Round 8 — Rulings (2026-08-10)

| # | Topic | Ruling |
|---|---|---|
| R8-a | `callback_url` in sync mode | **Omit the field entirely.** Not an empty string — an empty string is a URL-shaped slot that invites a worker to use it, forcing us to define what happens then. Unambiguous because R2-4 already requires every message to carry an explicit `connection_mode`: `sync` ⇒ no `callback_url`, reply over the HTTP response; `async` ⇒ always present |
| R8-b | Read API surface | **Adopted, conditional on R9-1.** `GET /runs/{run_id}` · `GET /runs/{run_id}/steps` · `GET /steps/{step_id}` · `GET /steps/{step_id}/output`. The two-layer split is deliberate: the catalogue is cheap and fetchable whole, outputs may be large and are fetched individually, so a planner can pull only what it needs. `/output` returns the stored bytes verbatim, per R7-b. **If R9-1 removes `history` from M1, this surface must be revisited** |
| R8-c | `history` row fields | **Adopted:** `step_id`, `step_name`, `seq`, `status`, `output_bytes`, `completed_at`, **`attempt_count`**. `attempt_count` is included because it is the planner's *only* window onto failure history (attempts are not in the catalogue), letting it react to a flaky worker, at the cost of one already-existing integer |
| R8-d | `async` + `raw` | **Not supported in v2 core** — mechanically impossible (nowhere to put `callback_url`). → BACKLOG |
| R8-e | `retry_after_seconds` (R4-5's third knob) | **Not in v2 core → BACKLOG.** Judged by R7-l: it is an optional field in a response body; adding it later breaks no existing worker, touches no schema (`step_retry_delay_seconds` already exists) and touches no state machine. Contrast with R8-d / R9-Q1, which are *semantics* and would be breaking |
| R8-f | Which `dispatch_style` the core builds first | **`envelope` first.** It is the easier one to develop and the one with full functionality; `raw` stays at milestone θ as already ordered in R5-f |
| R8-g | `input_from` under `raw` — **Claude's proposal overruled** | Claude proposed treating a non-empty `input_from` in raw mode as a planner failure (invoking G2). **The owner rejected this: in raw mode we must faithfully accept what the planner sent and must not assume it is an error.** A warning was floated and dismissed as unnecessary extra machinery. Exact reading pending — see R9-Q1 |

### Standing instructions issued by the owner (2026-08-10)

| # | Instruction |
|---|---|
| S-1 | **This log is updated after every exchange, without exception.** All documents are written **in English**. If anything is uncertain, ask rather than assume |
| S-2 | The log records **what was discussed, what was eliminated, what was chosen, and the reasoning** — written plainly enough that ideas cannot contaminate one another later |
| S-3 | Every message field and every DB schema column must eventually be documented with its meaning and its conditional logic (e.g. which fields exist in which mode). **Where this lives is undecided** — `notes/` is the leading candidate, because SPEC should be the must-follow document and carry no noise, only necessary information. Open |

---

## Round 9 — Claude's analysis (input to SPEC; not itself spec)

### R9-1 · Should `history` be removed from the orchestrator → planner message?

Owner's proposal: send only `run_id` (plus `fetch_base_url`); the planner first calls the read API
for the history, then decides which step outputs it wants. Benefit claimed: small messages, no huge
payload ever pushed. Cost: one extra round trip per planner call.

**Size, measured.** `history` is a **catalogue only** — it never carries outputs; that is precisely
what `output_bytes` is for. One row is roughly 180 bytes of JSON, so it grows with the **number of
steps**, not with data volume: ~1.8 KB at 10 steps, ~9 KB at 50, ~36 KB at 200. The "avoid huge
messages" concern was already largely solved when the catalogue was designed.

Worth naming: the genuinely unbounded field in M1 is **not** `history`, it is `workflow_input` —
user-supplied JSON of arbitrary size, resent on **every** planner call.

**The decisive argument: fetching saves nothing for either kind of planner.**
- An **LLM planner** needs the history to decide, and will put it in its prompt. Moving it to a
  fetch removes zero bytes and zero tokens — the same data arrives by a longer path.
- A planner that needs **no** history (a static planner keyed on `seq`) is the built-in one, which
  is likely in-process and makes no HTTP call at all.

So the benefit column is, in this design, close to empty.

**The cost column has three real entries.**
1. **Reliability regression.** Today only planner calls that need outputs depend on the read API.
   Removing `history` makes **every** call depend on it. R4-4 already recorded that a planner
   failing because the read API was unreachable is indistinguishable from a broken planner — both
   burn `planner_max_attempts` and push the run to DLQ. This change promotes that failure mode from
   occasional to universal.
2. **Loss of self-containment — this hits R1-3 and R1-8 directly.** A captured request carrying
   `history` is a reproducible, inspectable record of what the planner saw. `{"run_id":"018f…"}` is
   a *pointer*: without a live orchestrator in the same state you cannot reconstruct the decision.
   The planner request is the single most valuable message in the system to be able to look at — it
   is the snapshot of what the system believed at that moment.
3. **Round trips double** on every planner call.

**The owner's instinct is right on one axis — unbounded growth — but the fix is a cap, not
removal.** That axis was already ruled in R2-8b ("history size limits: required eventually").

**Considered and eliminated:** a workflow-level knob `planner_history: inline | fetch`. Rejected —
it is a setting nobody asked for that forces two code paths to be maintained forever, and R1-5 puts
the burden of proof on inclusion.

---

## Round 9 — Rulings (2026-08-10)

| # | Topic | Ruling |
|---|---|---|
| R9-a | `history` in M1 | **Kept, and no knob is offered.** The message carries the catalogue as originally designed. **A cap is added now: the most recent N steps, with N = 100**, stated explicitly in the documentation. Making N configurable is a BACKLOG item |
| R9-b | Truncation contract | **Option (c): one sentence in the planner contract, no `history_truncated` field** — *"history MAY be truncated; a planner MUST NOT assume it is complete; use the read API for the full list."* A boolean field is the kind of extra machinery already rejected in R8-g; a sentence achieves the same contract expectation at zero cost. Since R9-a implements the cap now, this sentence describes actual behaviour rather than only future-proofing |
| R9-b′ | Consequence spotted by the owner | Because a planner must be able to fetch what the catalogue omits, **the orchestrator's read API is required early, not late.** SPEC must therefore state the **complete orchestrator API surface** clearly and up front |
| R9-c | `input_from` under `raw` — **R8-g reversed** | Two positions were distinguished and the owner ruled on both. **(i) A StepSpec-level `input_from` together with `dispatch_style: "raw"` is an error** — "not expected in raw form" — treated as a planner failure. This reverses R8-g and restores Claude's original G2-based proposal, after the two possible readings were laid out. **(ii) A key literally named `input_from` *inside* `params` is ordinary data** and is sent through verbatim, becoming a top-level key of the raw body; `raw` does not interpret payload content. **(iii)** The orchestrator → worker envelope has no `input_from` field at all, so that leg needs no rule |
| R9-d | Where field/schema documentation lives (discharges S-3) | **Split by voice, per R2-1.** **SPEC** carries the *normative* part — which fields exist, their types, whether they are required, and **which fields exist in which mode**. This is necessary information, not noise: without it a planner author cannot conform and SPEC is not implementable. **`notes/`** carries the *narrative* part — why a field exists, what was eliminated, worked examples, walkthroughs. Test: a field table is a MUST (SPEC); a walkthrough that aids understanding is notes. R1-3 already permits a short `Why:` on any non-obvious SPEC rule, so brief rationale stays; long derivations move out |
| R9-e | Repository and module path | **`git init` done now** at `/home/aaronwu/Projects/piton`; `docs/` moved to `legacy/`. Go module path **`github.com/aaronwu001/piton`** (matches the configured git user). No `go.mod` written yet — **Go is not installed on this machine**, and guessing a toolchain version is worse than writing nothing |
| R9-f | Delivery order for the document set | **SPEC skeleton first** (section headings + one sentence each) → owner reviews the architecture → content is filled in → an external spec review pass → only then does development start |

### Standing instruction added

| # | Instruction |
|---|---|
| S-4 | Documents must be written so that a newcomer can use them as reference material and understand them: complete architecture, clear narration, English throughout |


