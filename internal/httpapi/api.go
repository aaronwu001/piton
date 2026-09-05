// Package httpapi is the HTTP surface of SPEC.md 10.
//
// SPEC.md 18.1 fixes which of it milestone α implements: "POST /workflows,
// POST /workflows/{id}/runs, GET /runs/{run_id}, GET /runs/{run_id}/steps and
// GET /healthz. The remaining read endpoints land with ζ, the first milestone
// that has a planner able to call them." Nothing else is registered below —
// SPEC.md 10.2 states the complete read surface because it is a contract a
// planner author builds against, not because α owes all of it.
//
// SPEC.md 10.2 also settles who the read endpoints are for: "the planner's read
// access and the operator's read access are one surface, not two", because
// "they ask the same questions. Two surfaces would drift apart, as they did in
// the previous project."
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aaronwu001/piton/internal/engine"
	"github.com/aaronwu001/piton/internal/model"
	"github.com/aaronwu001/piton/internal/storage"
	"github.com/aaronwu001/piton/internal/validate"
)

// maxBodyBytes bounds a request body. It is a guard rail, not a rule: SPEC.md
// puts no limit on a workflow definition's size, and this one is far above any
// plausible one.
const maxBodyBytes = 8 << 20

// API serves SPEC.md 10's endpoints.
type API struct {
	store  storage.Store
	engine *engine.Engine
	logger *log.Logger
}

func New(store storage.Store, eng *engine.Engine, logger *log.Logger) *API {
	return &API{store: store, engine: eng, logger: logger}
}

// Handler registers exactly the endpoints milestone α implements.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("POST /workflows", a.createWorkflow)
	mux.HandleFunc("POST /workflows/{workflow_id}/runs", a.createRun)
	mux.HandleFunc("GET /runs/{run_id}", a.getRun)
	mux.HandleFunc("GET /runs/{run_id}/steps", a.getRunSteps)
	return mux
}

// ---------------------------------------------------------------------------
// SPEC.md 10.5 — error responses
// ---------------------------------------------------------------------------

// errorBody is SPEC.md 10.5's shape.
//
// SPEC.md 10.5: "a rejection states the actual current state, not merely that
// the request was refused", because "the operator's next action depends on what
// is true now. '409 Conflict' alone forces a second request to find out."
// Beyond `error` and `message`, a rejection "carries the identifier and current
// status of every entity the request named or would have touched, and omits
// only those that do not exist" — hence the omitempty on every remaining field.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`

	WorkflowID string `json:"workflow_id,omitempty"`
	RunID      string `json:"run_id,omitempty"`
	RunStatus  string `json:"run_status,omitempty"`
	StepID     string `json:"step_id,omitempty"`
	StepStatus string `json:"step_status,omitempty"`
}

// The `error` slugs. SPEC.md 10.5 requires them to be stable and machine
// readable, and pairs each with a code in its table: 400 for a malformed
// request or a violation of SPEC.md 16, 404 for no such entity, 409 for a state
// that forbids the operation, 503 for storage being unreachable.
const (
	slugInvalidRequest = "invalid_request"
	slugNotFound       = "not_found"
	slugUnavailable    = "storage_unavailable"
)

func (a *API) writeJSON(w http.ResponseWriter, code int, body any) {
	raw, err := json.Marshal(body)
	if err != nil {
		a.logf("cannot encode a response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(raw)
}

func (a *API) writeError(w http.ResponseWriter, code int, body errorBody) {
	a.writeJSON(w, code, body)
}

// writeStorageError maps a storage failure onto SPEC.md 10.5's table. Anything
// that is not "no such entity" is storage being unreachable as far as the
// caller is concerned — SPEC.md 10.5's 503 — and the diagnosis stays in the
// log rather than being handed to a client that cannot act on it.
func (a *API) writeStorageError(w http.ResponseWriter, what string, err error) {
	if errors.Is(err, storage.ErrNotFound) {
		a.writeError(w, http.StatusNotFound, errorBody{Error: slugNotFound, Message: what})
		return
	}
	a.logf("%s: %v", what, err)
	a.writeError(w, http.StatusServiceUnavailable, errorBody{
		Error:   slugUnavailable,
		Message: what + ": storage is unreachable",
	})
}

func (a *API) logf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
	}
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer r.Body.Close()
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return nil, false
	}
	return raw, true
}

// contextWithTimeout bounds one handler's work. The request's own context is
// the parent, so a client that hangs up stops the query it started.
func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

// ---------------------------------------------------------------------------
// SPEC.md 10.4 — operational
// ---------------------------------------------------------------------------

// healthz is SPEC.md 10.4: "liveness, including whether storage is reachable".
//
// In this milestone a 200 here also carries the weight SPEC.md 18.1 gives it:
// demos/alpha's environment has no migration service, so the orchestrator
// applies migrations at boot and binds its listener only afterwards, which
// makes a 200 the proof that "migrations run to completion before the
// orchestrator serves traffic".
func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 5*time.Second)
	defer cancel()

	if err := a.store.Ping(ctx); err != nil {
		a.logf("healthz: %v", err)
		a.writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable", "storage": "unreachable",
		})
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "storage": "reachable"})
}

// ---------------------------------------------------------------------------
// SPEC.md 10.1 — control
// ---------------------------------------------------------------------------

// createWorkflow is POST /workflows.
//
// Every refusal here is SPEC.md 16's, and SPEC.md 16 says when they happen:
// "before any run exists". SPEC.md 18.1 puts all of SPEC.md 16 — and therefore
// SPEC.md 9.4 and 9.8 in full for every element of planner_static_steps — in
// this milestone.
func (a *API) createWorkflow(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		a.writeError(w, http.StatusBadRequest, errorBody{
			Error: slugInvalidRequest, Message: "the request body could not be read"})
		return
	}

	wf, rej := validate.Workflow(body)
	if rej != nil {
		// SPEC.md 10.5: "a POST /workflows rejection has no run to describe".
		// There is no workflow to describe either — it was never created — so
		// the two mandatory fields are all this body can honestly carry.
		a.writeError(w, http.StatusBadRequest, errorBody{Error: rej.Slug, Message: rej.Message})
		return
	}

	wf.WorkflowID = model.NewID()
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	if err := a.store.CreateWorkflow(ctx, wf); err != nil {
		a.writeStorageError(w, "cannot create the workflow", err)
		return
	}

	a.writeJSON(w, http.StatusCreated, map[string]any{
		"workflow_id": wf.WorkflowID,
		"name":        wf.Name,
		"created_at":  wf.CreatedAt,
	})
}

// createRun is POST /workflows/{id}/runs.
//
// SPEC.md 5.1: RUNNING is the state a run is created in. owner_id is left NULL:
// SPEC.md 8.6's sweep is what claims it, and SPEC.md 8.5's claim is the only
// statement that may write ownership. The nudge afterwards asks the sweep to
// run now instead of at its next tick, which is latency and nothing else.
func (a *API) createRun(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("workflow_id")

	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()

	wf, err := a.store.GetWorkflow(ctx, workflowID)
	if err != nil {
		// SPEC.md 10.5: 404 is "no such entity". The workflow does not exist,
		// so it is one of the entities the rejection "omits" rather than
		// describes.
		a.writeStorageError(w, "no workflow with that workflow_id", err)
		return
	}

	body, ok := readBody(w, r)
	if !ok {
		a.writeError(w, http.StatusBadRequest, errorBody{
			Error: slugInvalidRequest, Message: "the request body could not be read",
			WorkflowID: wf.WorkflowID})
		return
	}

	input, rej := validate.RunInput(body)
	if rej != nil {
		a.writeError(w, http.StatusBadRequest, errorBody{
			Error: rej.Slug, Message: rej.Message, WorkflowID: wf.WorkflowID})
		return
	}

	run := &model.Run{
		RunID:      model.NewID(),
		WorkflowID: wf.WorkflowID,
		Status:     model.StatusRunning,
		Input:      input,
	}
	if err := a.store.CreateRun(ctx, run); err != nil {
		a.writeStorageError(w, "cannot create the run", err)
		return
	}
	if a.engine != nil {
		a.engine.Nudge()
	}

	a.writeJSON(w, http.StatusCreated, map[string]any{
		"run_id":      run.RunID,
		"workflow_id": run.WorkflowID,
		"status":      run.Status,
		"created_at":  run.CreatedAt,
	})
}

// ---------------------------------------------------------------------------
// SPEC.md 10.2 — reads
// ---------------------------------------------------------------------------

// stepSummary is a catalogue row. It carries output_bytes rather than the
// output itself, which is SPEC.md 10.2's two-layer split — "the catalogue is
// cheap and may be fetched whole, while outputs may be large and are fetched
// individually" — and the same shape SPEC.md 9.2 gives a planner's history.
type stepSummary struct {
	StepID       string          `json:"step_id"`
	Seq          int             `json:"seq"`
	StepName     *string         `json:"step_name"`
	Status       string          `json:"status"`
	Decision     json.RawMessage `json:"decision"`
	AttemptCount int             `json:"attempt_count"`
	OutputBytes  int             `json:"output_bytes"`
	CreatedAt    time.Time       `json:"created_at"`
	CompletedAt  *time.Time      `json:"completed_at"`

	// Attempts is populated only by GET /runs/{run_id}/steps, which SPEC.md
	// 10.2 defines as "the step catalogue, with attempt summaries".
	Attempts []attemptSummary `json:"attempts,omitempty"`
}

type attemptSummary struct {
	AttemptID      string     `json:"attempt_id"`
	AttemptNo      int        `json:"attempt_no"`
	Status         string     `json:"status"`
	ConnectionMode string     `json:"connection_mode"`
	DeadlineAt     time.Time  `json:"deadline_at"`
	DispatchedBy   string     `json:"dispatched_by"`
	FailureReason  *string    `json:"failure_reason"`
	ErrorText      *string    `json:"error_text"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

type runBody struct {
	RunID      string          `json:"run_id"`
	WorkflowID string          `json:"workflow_id"`
	Status     string          `json:"status"`
	Input      json.RawMessage `json:"input"`

	PlannerAttemptCount int     `json:"planner_attempt_count"`
	ReplayCount         int     `json:"replay_count"`
	LastPlannerError    *string `json:"last_planner_error"`

	OwnerID   *string    `json:"owner_id"`
	ClaimedAt *time.Time `json:"claimed_at"`
	CreatedAt time.Time  `json:"created_at"`

	Steps []stepSummary `json:"steps"`
}

func summariseStep(st *model.Step) stepSummary {
	return stepSummary{
		StepID:       st.StepID,
		Seq:          st.Seq,
		StepName:     st.StepName,
		Status:       st.Status,
		Decision:     json.RawMessage(st.Decision),
		AttemptCount: st.AttemptCount,
		OutputBytes:  len(st.Output),
		CreatedAt:    st.CreatedAt,
		CompletedAt:  st.CompletedAt,
	}
}

// getRun is GET /runs/{run_id}: "one run, with a summary of its steps".
func (a *API) getRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")

	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()

	run, err := a.store.GetRun(ctx, runID)
	if err != nil {
		a.writeStorageError(w, "no run with that run_id", err)
		return
	}
	steps, err := a.store.ListSteps(ctx, runID)
	if err != nil {
		a.writeStorageError(w, "cannot read the run's steps", err)
		return
	}

	body := runBody{
		RunID:               run.RunID,
		WorkflowID:          run.WorkflowID,
		Status:              run.Status,
		Input:               json.RawMessage(run.Input),
		PlannerAttemptCount: run.PlannerAttemptCount,
		ReplayCount:         run.ReplayCount,
		LastPlannerError:    run.LastPlannerError,
		OwnerID:             run.OwnerID,
		ClaimedAt:           run.ClaimedAt,
		CreatedAt:           run.CreatedAt,
		Steps:               make([]stepSummary, 0, len(steps)),
	}
	for _, st := range steps {
		body.Steps = append(body.Steps, summariseStep(st))
	}
	a.writeJSON(w, http.StatusOK, body)
}

// getRunSteps is GET /runs/{run_id}/steps: "the step catalogue, with attempt
// summaries".
func (a *API) getRunSteps(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("run_id")

	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()

	// The run is read first so that a request for a run that does not exist is
	// SPEC.md 10.5's 404 rather than an empty catalogue, which would say "this
	// run has no steps" about a run that is not there.
	if _, err := a.store.GetRun(ctx, runID); err != nil {
		a.writeStorageError(w, "no run with that run_id", err)
		return
	}
	steps, err := a.store.ListSteps(ctx, runID)
	if err != nil {
		a.writeStorageError(w, "cannot read the run's steps", err)
		return
	}
	attempts, err := a.store.ListAttempts(ctx, runID)
	if err != nil {
		a.writeStorageError(w, "cannot read the run's attempts", err)
		return
	}

	byStep := make(map[string][]attemptSummary, len(steps))
	for _, at := range attempts {
		byStep[at.StepID] = append(byStep[at.StepID], attemptSummary{
			AttemptID:      at.AttemptID,
			AttemptNo:      at.AttemptNo,
			Status:         at.Status,
			ConnectionMode: at.ConnectionMode,
			DeadlineAt:     at.DeadlineAt,
			DispatchedBy:   at.DispatchedBy,
			FailureReason:  at.FailureReason,
			ErrorText:      at.ErrorText,
			StartedAt:      at.StartedAt,
			FinishedAt:     at.FinishedAt,
		})
	}

	out := make([]stepSummary, 0, len(steps))
	for _, st := range steps {
		summary := summariseStep(st)
		summary.Attempts = byStep[st.StepID]
		out = append(out, summary)
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"run_id": runID, "steps": out})
}
