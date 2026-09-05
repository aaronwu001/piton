// Package harness brings milestone alpha's environment up and gives the
// automated suite the two interfaces SPEC.md makes authoritative: the HTTP API
// (SPEC.md 10) and database truth (SPEC.md 17.1).
//
// CLAUDE.md 5.5.4 requires the suite to reference the milestone's own compose
// file rather than define an environment of its own, so that the owner's
// hand-run demo (CLAUDE.md 4 step 5) and the suite exercise an identical
// definition. Every docker command below therefore runs demos/alpha's
// docker-compose.yml, and nothing in this package writes a compose file of its
// own or edits that one.
//
// Nothing here was derived by reading an implementation: at the time it was
// written there is none (CLAUDE.md 4 step 3, R20-a). It is written against
// SPEC.md alone, which CLAUDE.md 5.1 makes the only legitimate source.
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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// BaseURL is where the orchestrator is reachable from the host: the port
// demos/alpha/docker-compose.yml publishes, which is what SPEC.md 18.1's
// "localhost:8080" refers to.
const BaseURL = "http://localhost:8080"

// RunInput is the workflow input SPEC.md 18.1's script submits.
//
// No worker ever sees it: SPEC.md 9.5's envelope carries params and inputs and
// no workflow input, and the static planner ignores SPEC.md 9.2's
// workflow_input by construction. It is asserted where it does live -
// runs.input, SPEC.md 6.2, "stored verbatim".
const RunInput = `{"text":"hello"}`

const (
	// upTimeout is generous because the first run of a group builds the
	// orchestrator image from source.
	upTimeout = 10 * time.Minute

	// commandTimeout bounds one short docker or psql invocation.
	commandTimeout = 2 * time.Minute

	// HealthTimeout bounds the wait for GET /healthz. SPEC.md 18.1 requires
	// migrations to run to completion before the orchestrator serves traffic,
	// and demos/alpha/docker-compose.yml has no migration service, so a 200
	// here is what proves they finished.
	HealthTimeout = 4 * time.Minute

	// RunTimeout bounds how long the alpha pipeline may take to reach a
	// terminal state. The echo worker answers in milliseconds; this is a
	// ceiling, not an expectation.
	RunTimeout = 2 * time.Minute
)

// moduleRoot walks up from this file to the directory holding go.mod.
//
// runtime.Caller reports the path this file had when it was compiled, which is
// the path it has when it runs: CLAUDE.md 8 requires the toolchain, the
// containers and the suite all to execute inside WSL, so there is no
// cross-machine build to invalidate it.
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

// DemoDir returns demos/alpha, the milestone's own environment directory.
func DemoDir() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "demos", "alpha"), nil
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

// Up starts the group's environment from a clean database.
//
// CLAUDE.md 5.5.2: a group always begins from a clean database, so the volume
// wipe happens before the environment comes up and not only after it goes down.
// A previous group that failed to tear itself down therefore cannot leak state
// into this one.
func Up() error {
	ctx, cancel := context.WithTimeout(context.Background(), upTimeout)
	defer cancel()

	// Best effort: there is usually nothing to remove.
	_, _ = composeRun(ctx, "", "down", "-v", "--remove-orphans")

	out, err := composeRun(ctx, "", "up", "-d", "--build", "--wait", "--wait-timeout", "240")
	if err != nil {
		// Diagnosis first, then teardown. A half-started environment left
		// running holds the published port 8080 and would make the next group
		// - or the owner's hand-run demo - fail for a reason that has nothing
		// to do with what it was testing. The logs are captured into the error
		// before the containers go away, so nothing is lost by removing them.
		logs, _ := composeRun(ctx, "", "logs", "--tail=60", "orchestrator")
		_, _ = composeRun(ctx, "", "down", "-v", "--remove-orphans")
		return fmt.Errorf("docker compose up failed: %w\n%s\n--- orchestrator logs ---\n%s", err, out, logs)
	}
	return nil
}

// Down tears the group's environment down, including the volume wipe CLAUDE.md
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

// OrchestratorLogs returns the tail of the orchestrator's log, for diagnosis
// only. SPEC.md 17.3 keeps error text in the database; the logs are the second
// resort, never the first.
func OrchestratorLogs(lines int) string {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, _ := composeRun(ctx, "", "logs", fmt.Sprintf("--tail=%d", lines), "orchestrator")
	return out
}

// PSQL runs one SQL statement against the demo's database and returns its
// output, unaligned and with surrounding whitespace removed.
//
// It goes through "docker compose exec" rather than a Go driver because
// demos/alpha/docker-compose.yml deliberately publishes no host port for
// postgres - a fixed one collides with any Postgres the operator already runs -
// and CLAUDE.md 5.5.4 forbids the suite from defining an environment of its own
// to obtain one. This is the same access path SPEC.md 17.1 gives the operator,
// and the one demo.sh uses.
//
// SQL is fed on stdin, never with -c: psql substitutes :'name' only for input
// it reads through its normal lexer, and with -c the placeholder would reach
// the server untouched and every query using one would fail to parse.
//
// Each vars entry is a psql variable in name=value form, referenced in SQL as
// :'name'. Values must not contain a single quote; nothing this suite passes
// does, and neither a UUID nor a compacted JSON document can.
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

// WorkflowJSON returns demos/alpha/workflow.json verbatim: the workflow
// definition SPEC.md 18.1's script posts.
//
// The validation group builds its rejected bodies by mutating this one, so
// every "this must be a 400" case differs from an accepted workflow in exactly
// the one way the rule under test names.
func WorkflowJSON() ([]byte, error) {
	dir, err := DemoDir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "workflow.json"))
}

// CreateWorkflow posts demos/alpha/workflow.json to POST /workflows and returns
// the workflow_id (SPEC.md 10.1, 18.1).
func CreateWorkflow() (string, error) {
	raw, err := WorkflowJSON()
	if err != nil {
		return "", err
	}
	code, body, err := Post("/workflows", raw)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return "", fmt.Errorf("POST /workflows returned %d: %s\n"+
			"SPEC.md 16 lists every reason this is a 400; the body says which", code, body)
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

// StartRun posts SPEC.md 10.1's run-creation body and returns the run_id.
//
// overrides is sent empty: SPEC.md 11.2 makes any non-empty value a 400 until
// milestone eta, and the field exists now only because the request shape is a
// contract other people build against.
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
// a run's status in GET /runs/{run_id}, so polling the API here would mean
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

// Seed performs SPEC.md 18.1's two control calls and waits for the run to reach
// a terminal state. Both groups need the same fixture: one workflow, one run,
// run to completion.
func Seed() (workflowID, runID, finalStatus string, err error) {
	if err = WaitHealthy(HealthTimeout); err != nil {
		return "", "", "", err
	}
	if workflowID, err = CreateWorkflow(); err != nil {
		return "", "", "", err
	}
	if runID, err = StartRun(workflowID); err != nil {
		return workflowID, "", "", err
	}
	finalStatus, err = WaitTerminal(runID, RunTimeout)
	return workflowID, runID, finalStatus, err
}

// StaticSteps returns demos/alpha/workflow.json's planner_static_steps, each
// element left as raw JSON so it can be compared against steps.decision, which
// SPEC.md 6.3 stores as "the StepSpec exactly as the planner returned it".
func StaticSteps() ([]json.RawMessage, error) {
	dir, err := DemoDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "workflow.json"))
	if err != nil {
		return nil, err
	}
	var wf struct {
		PlannerStaticSteps []json.RawMessage `json:"planner_static_steps"`
	}
	if err := json.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("workflow.json is not parseable: %w", err)
	}
	if len(wf.PlannerStaticSteps) == 0 {
		return nil, errors.New("workflow.json declares no planner_static_steps")
	}
	return wf.PlannerStaticSteps, nil
}

// ConfigSeconds reads one integer-valued key out of demos/alpha/piton.yaml.
//
// The values that govern ownership are read from the file the orchestrator
// itself boots on, so an assertion and the configuration it depends on cannot
// drift apart. A regexp rather than a YAML parser, because go.mod declares no
// dependencies and one scalar does not justify the first one.
func ConfigSeconds(key string) (int, error) {
	dir, err := DemoDir()
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "piton.yaml"))
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:[ \t]*([0-9]+)`)
	m := re.FindSubmatch(raw)
	if m == nil {
		return 0, fmt.Errorf("harness: %s not found in piton.yaml", key)
	}
	return strconv.Atoi(string(m[1]))
}
