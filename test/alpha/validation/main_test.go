// Package validation is milestone alpha's third test group: everything
// SPEC.md 16 refuses at submission time.
//
// SPEC.md 18.1 says alpha implements all of section 16, and section 9.4 and
// section 9.8 in full for every element of planner_static_steps. Section 6.1
// gives the reason it cannot be deferred: a malformed static plan discovered at
// run time leaves a run that can neither progress nor fail - it is not the
// planner refusing, and no attempt exists to burn budget - so it would sit
// RUNNING forever, reclaimed and re-failed by every sweep. Validating at
// submission makes that state unreachable, and these tests are what hold that
// door shut.
//
// The group starts no run. Every test here is one POST and one status code, so
// it is much the fastest of the three groups; it is a separate group only
// because CLAUDE.md 5.5.1 makes a group one compose environment and a Go
// package the unit that can own a TestMain.
package validation

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/aaronwu001/piton/test/alpha/harness"
)

// validWorkflowID is a workflow the orchestrator accepted, needed by the tests
// that reject a run-creation body rather than a workflow definition. Creating
// it is also this group's positive control: if demos/alpha/workflow.json were
// itself rejected, every "must be 400" assertion below would pass for the wrong
// reason.
var validWorkflowID string

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("alpha/validation: skipped in -short mode; this group needs docker compose")
		os.Exit(0)
	}

	if err := harness.Up(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/validation: the environment did not come up:", err)
		os.Exit(1)
	}

	code := run(m)
	if err := harness.Down(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/validation: teardown failed:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func run(m *testing.M) int {
	if err := harness.WaitHealthy(harness.HealthTimeout); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/validation:", err)
		fmt.Fprintln(os.Stderr, "\n--- orchestrator logs ---")
		fmt.Fprintln(os.Stderr, harness.OrchestratorLogs(60))
		return 1
	}

	id, err := harness.CreateWorkflow()
	if err != nil {
		fmt.Fprintln(os.Stderr, "alpha/validation: the demo's own workflow.json was not accepted:", err)
		fmt.Fprintln(os.Stderr, "Every rejection assertion in this group would otherwise pass for the wrong reason.")
		return 1
	}
	validWorkflowID = id
	fmt.Printf("alpha/validation: control workflow_id=%s\n", validWorkflowID)
	return m.Run()
}
