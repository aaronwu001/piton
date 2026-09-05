// Package happypath is milestone alpha's first test group: the scenario
// SPEC.md 18.1 requires, asserted against database truth.
//
// It is one group in the sense of CLAUDE.md 5.5: one docker-compose environment
// is brought up for this package, every test in it runs against that
// environment, and it is torn down with a volume wipe before the next group
// starts. Groups never share a live environment (5.5.2), which the suite
// enforces by running one package at a time - see test/alpha/run.sh.
//
// The fixture is created once, in TestMain, because SPEC.md 18.1's scenario is
// a single run: one workflow, one run, walked to a terminal state. Each test
// then asserts one rule about the state that run left behind.
package happypath

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/aaronwu001/piton/test/alpha/harness"
)

var (
	// workflowID and runID are the fixture every test in this group reads.
	workflowID string
	runID      string

	// finalStatus is the state the run reached. It is asserted, not assumed:
	// see TestRunReachedDone.
	finalStatus string

	// specs is demos/alpha/workflow.json's planner_static_steps, and n is how
	// many there are. Both are read from the file rather than written as a
	// literal, so the workflow and its assertions cannot drift apart.
	specs []json.RawMessage
	n     int
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("alpha/happypath: skipped in -short mode; this group needs docker compose")
		os.Exit(0)
	}

	var err error
	if specs, err = harness.StaticSteps(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/happypath:", err)
		os.Exit(1)
	}
	n = len(specs)

	if err = harness.Up(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/happypath: the environment did not come up:", err)
		fmt.Fprintln(os.Stderr, "\nIf the orchestrator is the service that failed, the likely reason is")
		fmt.Fprintln(os.Stderr, "simply that milestone alpha is not implemented yet: this suite is written")
		fmt.Fprintln(os.Stderr, "before the code (CLAUDE.md 4 step 3).")
		os.Exit(1)
	}

	code := run(m)
	if err := harness.Down(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/happypath: teardown failed:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// run seeds the fixture and runs the tests, so that TestMain's teardown happens
// through one return path rather than being duplicated on every failure.
func run(m *testing.M) int {
	var err error
	workflowID, runID, finalStatus, err = harness.Seed()
	if err != nil {
		fmt.Fprintln(os.Stderr, "alpha/happypath: the fixture could not be created:", err)
		fmt.Fprintln(os.Stderr, "\n--- orchestrator logs ---")
		fmt.Fprintln(os.Stderr, harness.OrchestratorLogs(60))
		return 1
	}
	fmt.Printf("alpha/happypath: workflow_id=%s run_id=%s final=%s\n", workflowID, runID, finalStatus)
	return m.Run()
}

// rv is the psql variable every query in this group binds: the fixture's run.
func rv() string { return "run=" + runID }
