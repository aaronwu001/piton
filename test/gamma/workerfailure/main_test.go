// Package workerfailure is milestone gamma test group: the four legs SPEC.md
// 18.2 requires, asserted against database truth.
//
// It is one group in the sense of CLAUDE.md 5.5: one docker-compose environment
// is brought up for this package, every test in it runs against that
// environment, and it is torn down with a volume wipe before the next group
// starts.
//
// WHY ALL FOUR LEGS ARE ONE GROUP AND NOT FOUR
//
//	CLAUDE.md 5.5.1 brings up one environment "per test file (or per milestone
//	scenario)", and SPEC.md 18.2 is one scenario: four runs against one
//	environment, which is what its demo.sh row requires of the demo too. Nothing
//	here touches global coordination state - no test reads the whole
//	orchestrators table, and no test asserts anything about runs it did not
//	start - so CLAUDE.md 5.5.3, which is what forces alpha ownership group to
//	stand alone, does not apply. The one place a test does speak about every run
//	at once is TestImpossibleCombination, and it asserts an invariant that must
//	hold no matter which runs exist.
//
// The fixture is created once, in TestMain, because the legs are independent by
// construction: each has its own workflow, its own run, and a worker whose
// counter is keyed on (run_id, step_id).
package workerfailure

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/aaronwu001/piton/test/gamma/harness"
)

// The four legs of SPEC.md 18.2, in the order that section lists them.
var (
	// legRetry is leg 1: the worker fails twice and succeeds on the third and
	// last attempt the budget allows.
	legRetry = &harness.Leg{Name: "leg 1 (recovering retry)", File: "workflow-retry.json"}

	// The three legs that reach SPEC.md 12.3 L4, one per way a worker fails.
	// Reason is the SPEC.md 5.3 value every attempt of that leg must carry.
	dlqLegs = []struct {
		Leg    *harness.Leg
		Reason string
	}{
		{&harness.Leg{Name: "leg 2 (worker reports failure)", File: "workflow-worker-error.json"}, "worker_error"},
		{&harness.Leg{Name: "leg 3 (worker replies HTTP 500)", File: "workflow-http-500.json"}, "transport_error"},
		{&harness.Leg{Name: "leg 4 (nothing listening)", File: "workflow-unreachable.json"}, "transport_error"},
	}
)

// maxAttempts is step_max_attempts, read from the workflow files rather than
// written as a literal, so an assertion and the workflow it is about cannot
// drift apart. SPEC.md 11.1 makes it a TOTAL attempt count, not a retry count.
var maxAttempts int

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("gamma/workerfailure: skipped in -short mode; this group needs docker compose")
		os.Exit(0)
	}

	var err error
	if maxAttempts, err = harness.MaxAttempts(legRetry.File); err != nil {
		fmt.Fprintln(os.Stderr, "gamma/workerfailure:", err)
		os.Exit(1)
	}
	// Every leg must agree on the budget, or the assertions below would be
	// comparing different runs against one number.
	for _, l := range dlqLegs {
		n, err := harness.MaxAttempts(l.Leg.File)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gamma/workerfailure:", err)
			os.Exit(1)
		}
		if n != maxAttempts {
			fmt.Fprintf(os.Stderr, "gamma/workerfailure: %s declares step_max_attempts %d, %s declares %d\n",
				l.Leg.File, n, legRetry.File, maxAttempts)
			os.Exit(1)
		}
	}

	if err = harness.Up(); err != nil {
		fmt.Fprintln(os.Stderr, "gamma/workerfailure: the environment did not come up:", err)
		os.Exit(1)
	}

	code := run(m)
	if err := harness.Down(); err != nil {
		fmt.Fprintln(os.Stderr, "gamma/workerfailure: teardown failed:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// run seeds the fixture and runs the tests, so that TestMain teardown happens
// through one return path rather than being duplicated on every failure.
func run(m *testing.M) int {
	if err := harness.WaitHealthy(harness.HealthTimeout); err != nil {
		fmt.Fprintln(os.Stderr, "gamma/workerfailure:", err)
		fmt.Fprintln(os.Stderr, "\n--- orchestrator logs ---")
		fmt.Fprintln(os.Stderr, harness.OrchestratorLogs(60))
		return 1
	}

	// The legs run one after another rather than concurrently. SPEC.md 4.4
	// deployment shape is one orchestrator, and running four runs at once would
	// make a failure ambiguous between the rule under test and contention no
	// milestone before beta has anything to say about.
	all := []*harness.Leg{legRetry}
	for _, l := range dlqLegs {
		all = append(all, l.Leg)
	}
	for _, leg := range all {
		if err := leg.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "gamma/workerfailure: the fixture could not be created:", err)
			fmt.Fprintln(os.Stderr, "\n--- orchestrator logs ---")
			fmt.Fprintln(os.Stderr, harness.OrchestratorLogs(60))
			return 1
		}
		fmt.Printf("gamma/workerfailure: %-32s run_id=%s final=%s\n", leg.Name, leg.RunID, leg.Final)
	}
	return m.Run()
}
