-- Piton, initial schema.
--
-- Every table, column and type below is SPEC.md 6. Every index is SPEC.md 7.2,
-- which calls them "part of the contract, not implementation footnotes".
--
-- Several of SPEC.md's invariants are written here as CHECK constraints rather
-- than left to the orchestrator. SPEC.md 6.4 asks for exactly that for the
-- attempts table — "invariants, enforced by the backend rather than by the
-- caller" — and gives the reason: "both are cheap to make impossible and
-- expensive to notice". The same reasoning is applied to the runs and steps
-- invariants that SPEC.md 6.2 and 6.3 state.

BEGIN;

-- ---------------------------------------------------------------- workflows
-- SPEC.md 6.1. A definition; a template, never executed itself.
CREATE TABLE workflows (
    workflow_id              UUID        PRIMARY KEY,
    name                     TEXT        NOT NULL,
    planner_type             TEXT        NOT NULL,
    planner_url              TEXT,
    planner_static_steps     JSONB,
    step_timeout_seconds     INT         NOT NULL,
    step_max_attempts        INT         NOT NULL,
    step_retry_delay_seconds INT         NOT NULL,
    planner_timeout_seconds  INT         NOT NULL,
    planner_max_attempts     INT         NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT workflows_planner_type
        CHECK (planner_type IN ('static', 'http')),

    -- SPEC.md 6.1: "exactly one of planner_url / planner_static_steps is
    -- present, determined by planner_type".
    CONSTRAINT workflows_planner_shape CHECK (
        (planner_type = 'http'
             AND planner_url IS NOT NULL AND planner_static_steps IS NULL)
     OR (planner_type = 'static'
             AND planner_url IS NULL AND planner_static_steps IS NOT NULL)
    ),

    -- SPEC.md 6.1 and 11.1: "all five numeric configuration columns are >= 1
    -- except step_retry_delay_seconds, which is >= 0".
    CONSTRAINT workflows_limits CHECK (
        step_timeout_seconds     >= 1 AND
        step_max_attempts        >= 1 AND
        step_retry_delay_seconds >= 0 AND
        planner_timeout_seconds  >= 1 AND
        planner_max_attempts     >= 1
    )
);

-- --------------------------------------------------------------------- runs
-- SPEC.md 6.2. The unit of history and the unit of ownership.
CREATE TABLE runs (
    run_id                UUID        PRIMARY KEY,
    workflow_id           UUID        NOT NULL REFERENCES workflows (workflow_id),
    status                TEXT        NOT NULL,
    input                 JSONB       NOT NULL,
    planner_attempt_count INT         NOT NULL DEFAULT 0,
    replay_count          INT         NOT NULL DEFAULT 0,
    last_planner_error    TEXT,
    owner_id              TEXT,
    claimed_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT runs_status CHECK (status IN ('RUNNING', 'DONE', 'DLQ', 'CANCELLED')),

    CONSTRAINT runs_counters CHECK (planner_attempt_count >= 0 AND replay_count >= 0),

    -- SPEC.md 6.2: owner_id and claimed_at "are always written and cleared as
    -- a pair".
    CONSTRAINT runs_coordination_pair
        CHECK ((owner_id IS NULL) = (claimed_at IS NULL)),

    -- SPEC.md 6.2: they are "non-NULL only while status = 'RUNNING'". SPEC.md
    -- 8.7's fourth writer is what makes this true rather than merely asserted;
    -- this constraint is what makes a forgotten fourth writer fail loudly
    -- instead of silently lying.
    CONSTRAINT runs_coordination_only_while_running
        CHECK (status = 'RUNNING' OR owner_id IS NULL)
);

-- SPEC.md 7.2: the sweep (SPEC.md 8.6). Partial, so that its cost is
-- proportional to current concurrency rather than to accumulated history — a
-- run that finished six months ago is not in the index at all.
CREATE INDEX runs_running_idx ON runs (status) WHERE status = 'RUNNING';

-- SPEC.md 7.2: release on shutdown (SPEC.md 8.7).
CREATE INDEX runs_owner_idx ON runs (owner_id);

-- -------------------------------------------------------------------- steps
-- SPEC.md 6.3.
CREATE TABLE steps (
    step_id       UUID        PRIMARY KEY,
    run_id        UUID        NOT NULL REFERENCES runs (run_id),
    seq           INT         NOT NULL,
    step_name     TEXT,
    status        TEXT        NOT NULL,
    decision      JSONB       NOT NULL,
    attempt_count INT         NOT NULL DEFAULT 0,
    output        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,

    CONSTRAINT steps_status CHECK (status IN ('RUNNING', 'DONE', 'DLQ', 'CANCELLED')),

    -- SPEC.md 3.3: seq starts at 1.
    CONSTRAINT steps_seq CHECK (seq >= 1),

    CONSTRAINT steps_attempt_count CHECK (attempt_count >= 0),

    -- SPEC.md 6.3: completed_at is "set exactly when status leaves RUNNING".
    CONSTRAINT steps_completed_at
        CHECK ((status = 'RUNNING') = (completed_at IS NULL))
);

-- SPEC.md 7.2: deriving last_step (SPEC.md 5.4) and assigning the next seq.
-- Unique, which is also SPEC.md 3.3's "unique per run".
CREATE UNIQUE INDEX steps_run_seq_idx ON steps (run_id, seq);

-- ----------------------------------------------------------------- attempts
-- SPEC.md 6.4.
CREATE TABLE attempts (
    attempt_id      UUID        PRIMARY KEY,
    step_id         UUID        NOT NULL REFERENCES steps (step_id),
    -- SPEC.md 6.4: denormalised, because the callback endpoint is addressed by
    -- attempt_id alone and must locate the run without a join.
    run_id          UUID        NOT NULL REFERENCES runs (run_id),
    attempt_no      INT         NOT NULL,
    status          TEXT        NOT NULL,
    connection_mode TEXT        NOT NULL,
    deadline_at     TIMESTAMPTZ NOT NULL,
    dispatched_by   TEXT        NOT NULL,
    output          JSONB,
    failure_reason  TEXT,
    error_text      TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,

    CONSTRAINT attempts_status CHECK (status IN ('RUNNING', 'DONE', 'FAILED')),

    CONSTRAINT attempts_connection_mode CHECK (connection_mode IN ('sync', 'async')),

    CONSTRAINT attempts_attempt_no CHECK (attempt_no >= 1),

    -- SPEC.md 5.3's enumeration.
    CONSTRAINT attempts_failure_reason_value CHECK (
        failure_reason IS NULL OR failure_reason IN (
            'worker_error', 'transport_error', 'invalid_response',
            'timeout', 'orphaned', 'cancelled')
    ),

    -- SPEC.md 6.4 invariants 1 and 2: FAILED implies a reason is present, and
    -- any other status implies it is absent. "A FAILED row with no reason is a
    -- dead end for the operator, and a reason attached to a DONE row is a
    -- contradiction that a later reader will trust."
    CONSTRAINT attempts_failure_reason
        CHECK ((status = 'FAILED') = (failure_reason IS NOT NULL)),

    -- SPEC.md 6.4 invariant 3.
    CONSTRAINT attempts_finished_at
        CHECK ((status <> 'RUNNING') = (finished_at IS NOT NULL)),

    -- SPEC.md 6.4: error_text is truncated to 4 KB. The orchestrator truncates
    -- before writing; this states the limit the column is under rather than
    -- doing the truncating.
    CONSTRAINT attempts_error_text_limit
        CHECK (error_text IS NULL OR octet_length(error_text) <= 4096)
);

-- SPEC.md 7.2: resolving the outstanding attempt (SPEC.md 4.2). Unique, which
-- is also SPEC.md 6.4's "1-based ordering within the step".
CREATE UNIQUE INDEX attempts_step_no_idx ON attempts (step_id, attempt_no);

-- SPEC.md 7.2: expiring overdue attempts. Partial, for the same reason as the
-- runs index above.
CREATE INDEX attempts_deadline_idx ON attempts (deadline_at) WHERE status = 'RUNNING';

-- -------------------------------------------------------- dead_letter_queue
-- SPEC.md 6.5. SPEC.md 6.7: append-only history. A row is never updated or
-- deleted, and one run may accumulate many rows across replay rounds.
CREATE TABLE dead_letter_queue (
    dlq_id        UUID        PRIMARY KEY,
    run_id        UUID        NOT NULL REFERENCES runs (run_id),
    -- SPEC.md 6.5: NULL for a planner-side entry. "step_id IS NULL already
    -- distinguishes the two sides, so no separate column records it."
    step_id       UUID        REFERENCES steps (step_id),
    reason        TEXT        NOT NULL,
    replay_round  INT         NOT NULL,
    attempt_count INT         NOT NULL,
    error_text    TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT dlq_reason CHECK (reason IN (
        'worker_budget_exhausted',
        'planner_unreachable',
        'planner_invalid_response',
        'planner_budget_exhausted',
        'planner_declared_fail')),

    -- SPEC.md 12.3: worker-side entries name a step, planner-side entries do
    -- not.
    CONSTRAINT dlq_side CHECK (
        (reason = 'worker_budget_exhausted') = (step_id IS NOT NULL)),

    CONSTRAINT dlq_counters CHECK (replay_round >= 0 AND attempt_count >= 0),

    -- SPEC.md 6.4: "one limit, both tables".
    CONSTRAINT dlq_error_text_limit CHECK (octet_length(error_text) <= 4096)
);

-- SPEC.md 7.2: GET /runs/{run_id}/dlq.
CREATE INDEX dlq_run_created_idx ON dead_letter_queue (run_id, created_at);

-- ------------------------------------------------------------ orchestrators
-- SPEC.md 6.6. One row per process boot; rows are never deleted by the system.
CREATE TABLE orchestrators (
    orchestrator_id TEXT        PRIMARY KEY,
    started_at      TIMESTAMPTZ NOT NULL,
    last_seen_at    TIMESTAMPTZ NOT NULL
);

COMMIT;
