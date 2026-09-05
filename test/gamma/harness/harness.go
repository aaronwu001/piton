// Package harness brings milestone gamma environment up and gives the
// automated suite the two interfaces SPEC.md makes authoritative: the HTTP API
// (SPEC.md 10) and database truth (SPEC.md 17.1).
//
// CLAUDE.md 5.5.4 requires the suite to reference the milestone own compose
// file rather than define an environment of its own, so that the owner hand-run
// demo (CLAUDE.md 4 step 5) and the suite exercise an identical definition.
// Every docker command below therefore runs demos/gamma/docker-compose.yml, and
// nothing in this package writes a compose file of its own or edits that one.
//
// WHY THIS DUPLICATES test/alpha/harness RATHER THAN IMPORTING IT
//
//	The two differ in exactly one thing that matters - which demo directory
//	they point at - so a shared package would have been possible. It was not
//	done, because alpha suite is the guard on a milestone the owner has
//	verified by hand, and refactoring it to serve gamma would put that guard at
//	risk for the convenience of a later milestone. SPEC.md 4.4 gives each
//	milestone its own directory for the same reason.
package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BaseURL is where the orchestrator is reachable from the host: the port
// demos/gamma/docker-compose.yml publishes.
const BaseURL = "http://localhost:8080"

// RunInput is the workflow input every leg submits. No worker sees it: SPEC.md
// 9.5 envelope carries params and inputs and no workflow input.
const RunInput = `{"text":"hello"}`

const (
	// upTimeout is generous because the first run of a group builds the
	// orchestrator image from source.
	upTimeout = 10 * time.Minute

	// commandTimeout bounds one short docker or psql invocation.
	commandTimeout = 2 * time.Minute

	// HealthTimeout bounds the wait for GET /healthz. There is no migration
	// service, so a 200 here is what proves the migrations finished.
	HealthTimeout = 4 * time.Minute

	// RunTimeout bounds how long one leg may take to reach a terminal state.
	// Every leg burns step_max_attempts against a worker that answers in
	// milliseconds or refuses instantly, and step_retry_delay_seconds is 0, so
	// this is a ceiling and not an expectation.
	RunTimeout = 2 * time.Minute
)

// moduleRoot walks up from this file to the directory holding go.mod.
func moduleRoot() (string, error) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("harness: cannot locate its own source file")
	}
	dir := filepath.Dir(self)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("harness: no go.mod found above %s", self)
		}
		dir = parent
	}
}

// DemoDir returns demos/gamma, the milestone own environment directory.
func DemoDir() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "demos", "gamma"), nil
}

func composeRun(ctx context.Context, stdin string, args ...string) (string, error) {
	dir, err := DemoDir()
	if err != nil {
		return "", err
	}
	full := append([]string{"compose", "-f", filepath.Join(dir, "docker-compose.yml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	// The working directory matters as much as -f: the compose file mounts
	// ./piton.yaml, which resolves against it.
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	return out.String(), err
}

// Up starts the group environment from a clean database.
//
// CLAUDE.md 5.5.2: a group always begins from a clean database, so the volume
// wipe happens before the environment comes up and not only after it goes down.
func Up() error {
	ctx, cancel := context.WithTimeout(context.Background(), upTimeout)
	defer cancel()

	// Best effort: there is usually nothing to remove.
	_, _ = composeRun(ctx, "", "down", "-v", "--remove-orphans")

	out, err := composeRun(ctx, "", "up", "-d", "--build", "--wait", "--wait-timeout", "240")
	if err != nil {
		// Diagnosis first, then teardown. A half-started environment left
		// running holds the published port 8080 and would make the next group -
		// or the owner hand-run demo - fail for a reason that has nothing to do
		// with what it was testing.
		logs, _ := composeRun(ctx, "", "logs", "--tail=60", "orchestrator")
		_, _ = composeRun(ctx, "", "down", "-v", "--remove-orphans")
		return fmt.Errorf("docker compose up failed: %w\n%s\n--- orchestrator logs ---\n%s", err, out, logs)
	}
	return nil
}

// Down tears the group environment down, including the volume wipe CLAUDE.md
// 5.5.1 requires before the next group starts.
func Down() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := composeRun(ctx, "", "down", "-v", "--remove-orphans")
	if err != nil {
		return fmt.Errorf("docker compose down -v failed: %w\n%s", err, out)
	}
	return nil
}

// OrchestratorLogs returns the tail of the orchestrator log, for diagnosis
// only. SPEC.md 17.3 keeps error text in the database; the logs are the second
// resort, never the first.
func OrchestratorLogs(lines int) string {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, _ := composeRun(ctx, "", "logs", fmt.Sprintf("--tail=%d", lines), "orchestrator")
	return out
}

// PSQL runs one SQL statement against the demo database and returns its output,
// unaligned and with surrounding whitespace removed.
//
// It goes through "docker compose exec" rather than a Go driver because
// demos/gamma/docker-compose.yml deliberately publishes no host port for
// postgres, and CLAUDE.md 5.5.4 forbids the suite from defining an environment
// of its own to obtain one. This is the same access path SPEC.md 17.1 gives the
// operator, and the one demo.sh uses.
//
// SQL is fed on stdin, never with -c: psql substitutes :'name' only for input it
// reads through its normal lexer.
func PSQL(sql string, vars ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	args := []string{
		"exec", "-T", "postgres",
		"psql", "-U", "piton", "-d", "piton",
		"-v", "ON_ERROR_STOP=1", "-At",
	}
	for _, v := range vars {
		args = append(args, "-v", v)
	}
	out, err := composeRun(ctx, sql, args...)
	if err != nil {
		return "", fmt.Errorf("psql failed: %w\nquery: %s\noutput: %s", err, sql, out)
	}
	return strings.TrimSpace(out), nil
}

func do(method, path string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, BaseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}

// Post issues a JSON POST and returns the status code and body.
func Post(path string, body []byte) (int, []byte, error) { return do(http.MethodPost, path, body) }

// Get issues a GET and returns the status code and body.
func Get(path string) (int, []byte, error) { return do(http.MethodGet, path, nil) }

// WaitHealthy polls GET /healthz until it answers 200.
func WaitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		code, body, err := Get("/healthz")
		switch {
		case err != nil:
			last = err.Error()
		case code == http.StatusOK:
			return nil
		default:
			last = fmt.Sprintf("status %d: %s", code, strings.TrimSpace(string(body)))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("GET /healthz never answered 200 within %s (last: %s)", timeout, last)
		}
		time.Sleep(time.Second)
	}
}

// WorkflowJSON returns one of demos/gamma workflow files verbatim.
func WorkflowJSON(file string) ([]byte, error) {
	dir, err := DemoDir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, file))
}

// StaticSteps returns a workflow file planner_static_steps, each element left
// as raw JSON so it can be compared against steps.decision, which SPEC.md 6.3
// stores as "the StepSpec exactly as the planner returned it".
func StaticSteps(file string) ([]json.RawMessage, error) {
	raw, err := WorkflowJSON(file)
	if err != nil {
		return nil, err
	}
	var wf struct {
		PlannerStaticSteps []json.RawMessage `json:"planner_static_steps"`
	}
	if err := json.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("%s is not parseable: %w", file, err)
	}
	if len(wf.PlannerStaticSteps) == 0 {
		return nil, fmt.Errorf("%s declares no planner_static_steps", file)
	}
	return wf.PlannerStaticSteps, nil
}

// MaxAttempts returns a workflow file step_max_attempts, so that an assertion
// and the workflow it is about cannot drift apart. SPEC.md 11.1 makes it a
// TOTAL attempt count, not a retry count.
func MaxAttempts(file string) (int, error) {
	raw, err := WorkflowJSON(file)
	if err != nil {
		return 0, err
	}
	var wf struct {
		StepMaxAttempts *int `json:"step_max_attempts"`
	}
	if err := json.Unmarshal(raw, &wf); err != nil {
		return 0, fmt.Errorf("%s is not parseable: %w", file, err)
	}
	if wf.StepMaxAttempts == nil {
		// SPEC.md 11.1 default. Every gamma workflow states it explicitly, so
		// reaching this line means a file was edited; the default is returned
		// rather than an error because it is what the orchestrator would use.
		return 3, nil
	}
	return *wf.StepMaxAttempts, nil
}

// CreateWorkflow posts one of demos/gamma workflow files to POST /workflows and
// returns the workflow_id (SPEC.md 10.1).
func CreateWorkflow(file string) (string, error) {
	raw, err := WorkflowJSON(file)
	if err != nil {
		return "", err
	}
	code, body, err := Post("/workflows", raw)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return "", fmt.Errorf("POST /workflows returned %d for %s: %s\n"+
			"SPEC.md 16 lists every reason this is a 400; the body says which", code, file, body)
	}
	var out struct {
		WorkflowID string `json:"workflow_id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("POST /workflows returned unparseable JSON: %w: %s", err, body)
	}
	if out.WorkflowID == "" {
		return "", fmt.Errorf("POST /workflows returned no workflow_id: %s", body)
	}
	return out.WorkflowID, nil
}

// StartRun posts SPEC.md 10.1 run-creation body and returns the run_id.
//
// overrides is sent empty: SPEC.md 11.2 makes any non-empty value a 400 until
// milestone eta.
func StartRun(workflowID string) (string, error) {
	body := []byte(fmt.Sprintf(`{"input":%s,"overrides":{}}`, RunInput))
	code, resp, err := Post("/workflows/"+workflowID+"/runs", body)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return "", fmt.Errorf("POST /workflows/{id}/runs returned %d: %s", code, resp)
	}
	var out struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", fmt.Errorf("POST /workflows/{id}/runs returned unparseable JSON: %w: %s", err, resp)
	}
	if out.RunID == "" {
		return "", fmt.Errorf("POST /workflows/{id}/runs returned no run_id: %s", resp)
	}
	return out.RunID, nil
}

// WaitTerminal polls runs.status until the run leaves RUNNING, and returns the
// state it reached.
//
// The wait polls the DATABASE, not the API: SPEC.md 17.1 makes database truth
// the interface, and SPEC.md 10.2 does not fix the JSON field name that carries
// a run status in GET /runs/{run_id}, so polling the API here would mean
// inventing a wire contract no ruling covers. runs.status is specified, in
// SPEC.md 5.1 and 6.2.
func WaitTerminal(runID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		status, err := PSQL("SELECT status FROM runs WHERE run_id = :'run';", "run="+runID)
		if err != nil {
			return "", err
		}
		// SPEC.md 5.1: DONE and CANCELLED are terminal, and DLQ is left only by
		// an explicit replay or cancel. All three end this wait.
		switch status {
		case "DONE", "DLQ", "CANCELLED":
			return status, nil
		}
		if time.Now().After(deadline) {
			return status, fmt.Errorf("run %s was still %q after %s", runID, status, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Leg is one of SPEC.md 18.2 four scenarios: a workflow submitted, a run
// started from it, and the terminal state that run reached.
type Leg struct {
	// Name is how a failure names this leg, in SPEC.md 18.2 vocabulary.
	Name string
	// File is the workflow file in demos/gamma that produced it.
	File string
	// WorkflowID and RunID are what the two control calls returned.
	WorkflowID string
	RunID      string
	// Final is the state the run reached. It is asserted, never assumed.
	Final string
}

// Run submits one leg workflow, starts a run, and waits for it to reach a
// terminal state.
func (l *Leg) Run() error {
	var err error
	if l.WorkflowID, err = CreateWorkflow(l.File); err != nil {
		return fmt.Errorf("%s: %w", l.Name, err)
	}
	if l.RunID, err = StartRun(l.WorkflowID); err != nil {
		return fmt.Errorf("%s: %w", l.Name, err)
	}
	if l.Final, err = WaitTerminal(l.RunID, RunTimeout); err != nil {
		return fmt.Errorf("%s: %w", l.Name, err)
	}
	return nil
}

// Var is the psql variable binding for this leg run, for use with PSQL and the
// assertion helpers.
func (l *Leg) Var() string { return "run=" + l.RunID }
