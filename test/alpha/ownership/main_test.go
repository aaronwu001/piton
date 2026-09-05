// Package ownership is milestone alpha's second test group: the coordination
// state the run leaves behind, and the heartbeat that keeps the orchestrator
// live.
//
// It is a separate group for the reason CLAUDE.md 5.5.3 gives: these tests read
// and assert global coordination state - the whole orchestrators table, and
// runs.owner_id - so a test running beside them would see a database that no
// single test put into that condition. "Exactly one orchestrator row" is only a
// meaningful assertion when nothing else is allowed to start a second one.
//
// CLAUDE.md 5.3 requires everything touching ownership to be tested against a
// real Postgres from docker compose, never a fake: the correctness argument of
// SPEC.md 8 rests on database semantics, and a mock cannot verify any of it.
// This group therefore uses demos/alpha/docker-compose.yml like the other one,
// and never a private environment (CLAUDE.md 5.5.4).
package ownership

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/aaronwu001/piton/test/alpha/harness"
)

var (
	runID       string
	finalStatus string

	// The two numbers that govern liveness, read from the file the
	// orchestrator itself boots on (demos/alpha/piton.yaml) so that an
	// assertion and the configuration it depends on cannot drift apart.
	heartbeatInterval int
	leaseTTL          int
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("alpha/ownership: skipped in -short mode; this group needs docker compose")
		os.Exit(0)
	}

	var err error
	if heartbeatInterval, err = harness.ConfigSeconds("heartbeat_interval_seconds"); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/ownership:", err)
		os.Exit(1)
	}
	if leaseTTL, err = harness.ConfigSeconds("lease_ttl_seconds"); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/ownership:", err)
		os.Exit(1)
	}

	if err = harness.Up(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/ownership: the environment did not come up:", err)
		os.Exit(1)
	}

	code := run(m)
	if err := harness.Down(); err != nil {
		fmt.Fprintln(os.Stderr, "alpha/ownership: teardown failed:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func run(m *testing.M) int {
	// This group needs a run that has reached a terminal state, because two of
	// its rules are about what a terminal run may no longer hold.
	_, id, status, err := harness.Seed()
	if err != nil {
		fmt.Fprintln(os.Stderr, "alpha/ownership: the fixture could not be created:", err)
		fmt.Fprintln(os.Stderr, "\n--- orchestrator logs ---")
		fmt.Fprintln(os.Stderr, harness.OrchestratorLogs(60))
		return 1
	}
	runID, finalStatus = id, status
	fmt.Printf("alpha/ownership: run_id=%s final=%s heartbeat=%ds lease_ttl=%ds\n",
		runID, finalStatus, heartbeatInterval, leaseTTL)
	return m.Run()
}

func rv() string { return "run=" + runID }
