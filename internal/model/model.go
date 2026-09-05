// Package model holds the entities of SPEC.md 3.2 and the enumerated values of
// SPEC.md 5. It carries no behaviour beyond what a value's own definition
// implies, and it knows nothing about storage or HTTP.
//
// Every JSON-valued field is []byte, not a parsed structure. SPEC.md 7.1 makes
// that a requirement of the storage interface — "opaque []byte for every JSON
// document" — and the same bytes travel through this package unchanged, which
// is what lets SPEC.md 6.3 store "the StepSpec exactly as the planner returned
// it" and SPEC.md 6.2 store the run's input "verbatim".
package model

import "time"

// Run states (SPEC.md 5.1), step states (SPEC.md 5.2) and attempt states
// (SPEC.md 5.3). They are plain strings because they are plain strings in the
// database: SPEC.md 6.2, 6.3 and 6.4 give every status column type TEXT.
const (
	StatusRunning   = "RUNNING"
	StatusDone      = "DONE"
	StatusDLQ       = "DLQ"
	StatusCancelled = "CANCELLED"
	StatusFailed    = "FAILED"
)

// Failure reasons (SPEC.md 5.3). Each is "a diagnostic label, not a distinct
// mechanism"; every value below burns one unit of budget except Cancelled.
const (
	FailureWorkerError     = "worker_error"
	FailureTransportError  = "transport_error"
	FailureInvalidResponse = "invalid_response"
	FailureTimeout         = "timeout"
	FailureOrphaned        = "orphaned"
	FailureCancelled       = "cancelled"
)

// Dead-letter reasons (SPEC.md 6.5).
const (
	DLQWorkerBudgetExhausted  = "worker_budget_exhausted"
	DLQPlannerUnreachable     = "planner_unreachable"
	DLQPlannerInvalidResponse = "planner_invalid_response"
	DLQPlannerBudgetExhausted = "planner_budget_exhausted"
	DLQPlannerDeclaredFail    = "planner_declared_fail"
)

// Planner types (SPEC.md 6.1).
const (
	PlannerStatic = "static"
	PlannerHTTP   = "http"
)

// Connection modes and dispatch styles (SPEC.md 9.4).
const (
	ConnectionSync  = "sync"
	ConnectionAsync = "async"

	DispatchEnvelope = "envelope"
	DispatchRaw      = "raw"
)

// ErrorTextLimit is SPEC.md 6.4's 4 KB cap on diagnostic text — "one limit,
// both tables", attempts.error_text and dead_letter_queue.error_text. SPEC.md
// 6.4 also fixes who applies it: "the orchestrator truncates before writing;
// it is never the backend's job".
const ErrorTextLimit = 4 << 10

// TruncateError applies that cap. It cuts on a byte boundary rather than a
// rune boundary only when the cap falls mid-rune; the column is diagnostic
// text read by a human, and a trailing partial rune is not worth a second rule.
func TruncateError(s string) string {
	if len(s) <= ErrorTextLimit {
		return s
	}
	cut := ErrorTextLimit
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut]
}

// Workflow is SPEC.md 6.1: a definition, "a template and is never executed
// itself".
type Workflow struct {
	WorkflowID string
	Name       string

	PlannerType string
	// PlannerURL is present iff PlannerType is http; PlannerStaticSteps is
	// present iff it is static (SPEC.md 6.1's invariant).
	PlannerURL         string
	PlannerStaticSteps []byte

	StepTimeoutSeconds    int
	StepMaxAttempts       int
	StepRetryDelaySeconds int
	PlannerTimeoutSeconds int
	PlannerMaxAttempts    int

	CreatedAt time.Time
}

// Run is SPEC.md 6.2: one execution of a workflow, "the unit of history and the
// unit of ownership".
type Run struct {
	RunID      string
	WorkflowID string
	Status     string
	Input      []byte

	PlannerAttemptCount int
	ReplayCount         int
	LastPlannerError    *string

	// OwnerID and ClaimedAt are coordination metadata (SPEC.md 3.4), non-NULL
	// only while Status is RUNNING and always written and cleared as a pair
	// (SPEC.md 6.2, 8.7).
	OwnerID   *string
	ClaimedAt *time.Time

	CreatedAt time.Time
}

// Step is SPEC.md 6.3: one decided unit of work at a fixed position seq.
type Step struct {
	StepID   string
	RunID    string
	Seq      int
	StepName *string
	Status   string

	// Decision is the StepSpec exactly as the planner returned it (SPEC.md
	// 6.3), which is why it never leaves this package as a parsed struct and
	// is re-serialised nowhere.
	Decision []byte

	AttemptCount int
	Output       []byte

	CreatedAt   time.Time
	CompletedAt *time.Time
}

// Attempt is SPEC.md 6.4: one execution of a step, one dispatch and one
// outcome.
type Attempt struct {
	AttemptID      string
	StepID         string
	RunID          string
	AttemptNo      int
	Status         string
	ConnectionMode string
	DeadlineAt     time.Time
	DispatchedBy   string
	Output         []byte
	FailureReason  *string
	ErrorText      *string
	StartedAt      time.Time
	FinishedAt     *time.Time
}

// DeadLetterEntry is SPEC.md 6.5: an append-only historical record that a run
// stopped because a budget was exhausted or the planner refused to continue.
type DeadLetterEntry struct {
	DLQID        string
	RunID        string
	StepID       *string
	Reason       string
	ReplayRound  int
	AttemptCount int
	ErrorText    string
	CreatedAt    time.Time
}

// Orchestrator is SPEC.md 6.6: one row per process boot, whose last_seen_at is
// "the only column a heartbeat touches".
type Orchestrator struct {
	OrchestratorID string
	StartedAt      time.Time
	LastSeenAt     time.Time
}
