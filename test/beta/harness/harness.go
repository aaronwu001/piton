// Package harness brings milestone beta's environment up and gives the
// automated suite the interfaces SPEC.md makes authoritative: the HTTP API
// (SPEC.md 10) and database truth (SPEC.md 17.1) - plus the one thing no
// earlier milestone needed, the ability to kill and restart the orchestrator.
//
// CLAUDE.md 5.5.4 requires the suite to reference the milestone's own compose
// file rather than define an environment of its own, so every docker command
// below runs demos/beta/docker-compose.yml.
//
// WHY THIS DUPLICATES test/alpha/harness AND test/gamma/harness
//
//	See BACKLOG.md B18. Each milestone's suite is the guard on a milestone the
//	owner verified by hand, and refactoring an earlier one to serve a later one
//	puts that guard at risk for convenience. Beta's harness has diverged further
//	than gamma's did in any case: killing a process, restarting it, and waiting
//	for one orchestrator to take a run away from another are all new here.
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

// BaseURL is where the orchestrator is reachable from the host.
const BaseURL = "http://localhost:8080"

// RunInput is the workflow input every leg submits.
const RunInput = `{"text":"hello"}`

const (
	upTimeout      = 10 * time.Minute
	commandTimeout = 2 * time.Minute

	// HealthTimeout bounds the wait for GET /healthz, at first boot and after
	// every restart.
	HealthTimeout = 4 * time.Minute

	// RunTimeout bounds how long one leg may take to reach a terminal state.
	// Beta's legs contain deliberate sleeps and at least one lease expiry, so
	// this is larger than gamma's - but it is still a ceiling, not an
	// expectation.
	RunTimeout = 5 * time.Minute

	// Poll is how often the database is asked whether something has happened
	// yet. Beta waits on events - an attempt appearing, an owner changing -
	// rather than on wall-clock guesses, so this is the resolution of every
	// wait in the suite.
	Poll = 250 * time.Millisecond
)

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

// DemoDir returns demos/beta, the milestone's own environment directory.
func DemoDir() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "demos", "beta"), nil
}

// compose runs one docker compose command and returns its combined output and
// the process's exit code.
func compose(ctx context.Context, stdin string, args ...string) (string, int, error) {
	dir, err := DemoDir()
	if err != nil {
		return "", -1, err
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
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		code = -1
	}
	return out.String(), code, err
}

func composeRun(ctx context.Context, stdin string, args ...string) (string, error) {
	out, _, err := compose(ctx, stdin, args...)
	return out, err
}

// Up starts the group's environment from a clean database (CLAUDE.md 5.5.2).
func Up() error {
	ctx, cancel := context.WithTimeout(context.Background(), upTimeout)
	defer cancel()

	_, _ = composeRun(ctx, "", "down", "-v", "--remove-orphans")

	out, err := composeRun(ctx, "", "up", "-d", "--build", "--wait", "--wait-timeout", "240")
	if err != nil {
		logs, _ := composeRun(ctx, "", "logs", "--tail=60", "orchestrator")
		_, _ = composeRun(ctx, "", "down", "-v", "--remove-orphans")
		return fmt.Errorf("docker compose up failed: %w\n%s\n--- orchestrator logs ---\n%s", err, out, logs)
	}
	return nil
}

// Down tears the environment down, volume wipe included.
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
// only. SPEC.md 17.3 keeps error text in the database; logs are the second
// resort.
func OrchestratorLogs(lines int) string {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, _ := composeRun(ctx, "", "logs", fmt.Sprintf("--tail=%d", lines), "orchestrator")
	return out
}

// ---------------------------------------------------------------------------
// Killing and restarting - what beta needs and no earlier milestone did
// ---------------------------------------------------------------------------

// Kill stops the orchestrator with SIGKILL: no signal handler runs, no clean
// shutdown happens, and therefore SPEC.md 8.7's release does NOT happen. The
// run keeps its owner_id, and the only thing that can take it away is another
// orchestrator finding the owner no longer live (SPEC.md 8.5).
//
// This is the situation SPEC.md 13.1.1 guarantees is survived: "the
// orchestrator is killed at any instant".
func Kill() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := composeRun(ctx, "", "kill", "-s", "KILL", "orchestrator")
	if err != nil {
		return fmt.Errorf("docker compose kill failed: %w\n%s", err, out)
	}
	return nil
}

// StopClean stops the orchestrator with SIGTERM and lets it shut down. SPEC.md
// 8.7: on a clean shutdown the orchestrator releases - `owner_id = NULL,
// claimed_at = NULL WHERE owner_id = :me` - which "makes failover immediate
// rather than lease_ttl later; correctness does not depend on it".
//
// It is the contrast that gives Kill its meaning: after a clean stop ownership
// is gone at once, after a kill it survives until the lease expires.
func StopClean() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := composeRun(ctx, "", "stop", "-t", "30", "orchestrator")
	if err != nil {
		return fmt.Errorf("docker compose stop failed: %w\n%s", err, out)
	}
	return nil
}

// Start brings the orchestrator back and waits for it to serve traffic again.
// SPEC.md 8.6: "startup recovery is not a separate code path - it is the first
// sweep."
func Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := composeRun(ctx, "", "start", "orchestrator")
	if err != nil {
		return fmt.Errorf("docker compose start failed: %w\n%s", err, out)
	}
	return WaitHealthy(HealthTimeout)
}

// BootWith runs a throwaway orchestrator against the named configuration file
// and returns its exit code and output, without touching the environment's own
// orchestrator container.
//
// It exists for SPEC.md 13.1.5 - "storage unreachable at startup. Fail fast:
// non-zero exit, and an error message that names storage as the cause" - which
// is the only one of SPEC.md 13.1's six situations that cannot be produced by
// killing something.
func BootWith(configPath string) (output string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, code, runErr := compose(ctx, "", "run", "--rm", "--no-deps", "--entrypoint",
		"/usr/local/bin/piton", "orchestrator", "--config", configPath)
	if code == -1 && runErr != nil {
		return out, code, fmt.Errorf("docker compose run could not be executed: %w\n%s", runErr, out)
	}
	return out, code, nil
}

// ---------------------------------------------------------------------------
// Database truth
// ---------------------------------------------------------------------------

// PSQL runs one SQL statement against the demo's database.
//
// It goes through "docker compose exec" because demos/beta/docker-compose.yml
// publishes no host port for postgres, and CLAUDE.md 5.5.4 forbids the suite
// from defining an environment of its own to obtain one. This is SPEC.md 17.1's
// access path, and the one demo.sh uses.
//
// SQL is fed on stdin, never with -c: psql substitutes :'name' only for input
// it reads through its normal lexer.
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

// Await polls a SQL predicate until it is true, and reports what it last saw if
// it never became true.
//
// Every wait in beta is written this way rather than as a sleep. SPEC.md 13.3
// makes a timeout a LOWER bound - "somewhere in [deadline, deadline +
// sweep_interval]" when the owner is dead - so a test that slept for a
// calculated duration and then asserted would be testing a schedule SPEC.md
// deliberately does not promise. Waiting for the state itself asserts only what
// is guaranteed, and a slow machine makes the suite slower rather than red.
func Await(what, sql string, timeout time.Duration, vars ...string) error {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		out, err := PSQL(sql, vars...)
		if err != nil {
			last = err.Error()
		} else {
			last = out
			if out == "t" {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s (last: %q)\n  query: %s",
				timeout, what, last, sql)
		}
		time.Sleep(Poll)
	}
}

// ---------------------------------------------------------------------------
// The HTTP API
// ---------------------------------------------------------------------------

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

// WorkflowJSON returns one of demos/beta's workflow files verbatim.
func WorkflowJSON(file string) ([]byte, error) {
	dir, err := DemoDir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, file))
}

// StaticSteps returns a workflow file's planner_static_steps.
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
	return wf.PlannerStaticSteps, nil
}

// MaxAttempts returns a workflow file's step_max_attempts (SPEC.md 11.1: a
// TOTAL attempt count, not a retry count), read from the file so that an
// assertion and the workflow it is about cannot drift apart.
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
		return 3, nil // SPEC.md 11.1's default
	}
	return *wf.StepMaxAttempts, nil
}

// LeaseTTL reads lease_ttl_seconds out of demos/beta/piton.yaml - the file the
// orchestrator itself boots on - so that a wait for a takeover and the
// configuration governing it cannot drift apart. SPEC.md 8.7: an orchestrator
// is live iff last_seen_at > now() - lease_ttl.
func LeaseTTL() (time.Duration, error) {
	n, err := configSeconds("lease_ttl_seconds")
	return time.Duration(n) * time.Second, err
}

// SweepInterval reads sweep_interval_seconds out of the same file. SPEC.md 13.3
// makes it the width of the uncertainty window in which an attempt with no live
// owner is declared failed.
func SweepInterval() (time.Duration, error) {
	n, err := configSeconds("sweep_interval_seconds")
	return time.Duration(n) * time.Second, err
}

// TakeoverBudget is how long a takeover may take before the suite calls it a
// failure: the lease must expire, then a sweep must run, and then the claim
// happens. The multiplier is slack for a loaded machine, not a promise - it
// only ever makes the suite wait longer before failing.
func TakeoverBudget() (time.Duration, error) {
	ttl, err := LeaseTTL()
	if err != nil {
		return 0, err
	}
	sweep, err := SweepInterval()
	if err != nil {
		return 0, err
	}
	return 4*(ttl+sweep) + 30*time.Second, nil
}

func configSeconds(key string) (int, error) {
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

// CreateWorkflow posts one of demos/beta's workflow files (SPEC.md 10.1).
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
		return "", fmt.Errorf("POST /workflows returned %d for %s: %s", code, file, body)
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

// Begin submits a workflow and starts a run from it, returning the run_id.
func Begin(file string) (string, error) {
	wf, err := CreateWorkflow(file)
	if err != nil {
		return "", err
	}
	return StartRun(wf)
}

// WaitTerminal polls runs.status until the run leaves RUNNING.
//
// It polls the DATABASE, not the API: SPEC.md 17.1 makes database truth the
// interface, and SPEC.md 10.2 does not fix the JSON field name carrying a run's
// status, so polling the API would mean inventing a contract no ruling covers.
func WaitTerminal(runID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		status, err := PSQL("SELECT status FROM runs WHERE run_id = :'run';", "run="+runID)
		if err == nil {
			switch status {
			case "DONE", "DLQ", "CANCELLED":
				return status, nil
			}
			if time.Now().After(deadline) {
				return status, fmt.Errorf("run %s was still %q after %s", runID, status, timeout)
			}
		} else if time.Now().After(deadline) {
			return "", err
		}
		time.Sleep(Poll)
	}
}

// WaitAttemptRunning waits until the named step of a run has a RUNNING attempt,
// and returns the orchestrator_id that dispatched it.
//
// Beta's legs kill the orchestrator "mid-run", and this is what makes that
// precise: the kill happens once an attempt is on the wire, not after a sleep
// long enough that it probably is.
func WaitAttemptRunning(runID, stepName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	const q = `SELECT a.dispatched_by FROM attempts a JOIN steps s ON s.step_id = a.step_id
                WHERE a.run_id = :'run' AND s.step_name = :'step' AND a.status = 'RUNNING'
                ORDER BY a.attempt_no DESC LIMIT 1;`
	for {
		out, err := PSQL(q, "run="+runID, "step="+stepName)
		if err == nil && out != "" {
			return out, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("step %q of run %s never had a RUNNING attempt within %s",
				stepName, runID, timeout)
		}
		time.Sleep(Poll)
	}
}

// WaitAttemptNoRunning waits until a SPECIFIC attempt number of a step is
// RUNNING, and returns the orchestrator_id that dispatched it.
//
// The distinction from WaitAttemptRunning is not pedantry, and getting it wrong
// silently weakens a test rather than failing it. After a SIGKILL the killed
// attempt is still RUNNING in the database - nobody expired it, because the
// process that would have died with it - so "is an attempt RUNNING?" is true
// again the instant the next orchestrator boots, long before it has claimed
// anything (SPEC.md 8.5 makes it wait for the dead owner's lease to expire).
// A loop that killed on that signal would kill a process that had not yet done
// anything, and would burn no budget at all.
func WaitAttemptNoRunning(runID, stepName string, attemptNo int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	const q = `SELECT a.dispatched_by FROM attempts a JOIN steps s ON s.step_id = a.step_id
                WHERE a.run_id = :'run' AND s.step_name = :'step'
                  AND a.attempt_no = :attempt AND a.status = 'RUNNING';`
	for {
		out, err := PSQL(q, "run="+runID, "step="+stepName, fmt.Sprintf("attempt=%d", attemptNo))
		if err == nil && out != "" {
			return out, nil
		}
		if time.Now().After(deadline) {
			state, _ := PSQL(`SELECT coalesce(string_agg(a.attempt_no || ':' || a.status, ',' ORDER BY a.attempt_no), '<none>')
                                FROM attempts a JOIN steps s ON s.step_id = a.step_id
                               WHERE a.run_id = :'run' AND s.step_name = :'step';`,
				"run="+runID, "step="+stepName)
			return "", fmt.Errorf("attempt %d of step %q (run %s) was never RUNNING within %s; attempts are: %s",
				attemptNo, stepName, runID, timeout, state)
		}
		time.Sleep(Poll)
	}
}

// Var is the psql variable binding for a run.
func Var(runID string) string { return "run=" + runID }
