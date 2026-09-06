package recovery

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aaronwu001/piton/test/beta/harness"
)

// ---------------------------------------------------------------------------
// Leg 1 - the orchestrator is killed at any instant (SPEC.md 13.1.1)
// ---------------------------------------------------------------------------

// TestOwnershipSurvivesSIGKILL is the observation that gives every other leg its
// meaning. SPEC.md 8.7 names exactly four writers of coordination metadata -
// claim, heartbeat, release, and any transition out of RUNNING - and all four
// live inside an orchestrator process. A SIGKILL runs none of them, so the run
// must still carry the dead process's owner_id.
//
// If this were NOT true, "recovery" would be indistinguishable from "the run was
// never owned", and SPEC.md 8.5's claim - which can only take a run from an
// owner that is not live - would have nothing to do.
func TestOwnershipSurvivesSIGKILL(t *testing.T) {
	if obs.ownerBeforeKill == "" {
		t.Fatal("SPEC.md 6.2: a RUNNING run being driven must carry a non-NULL owner_id; it was empty")
	}
	if obs.ownerAfterKill != obs.ownerBeforeKill {
		t.Errorf("SPEC.md 8.7: SIGKILL runs no shutdown path, so ownership must survive it.\n"+
			"  owner before the kill: %q\n  owner after the kill:  %q\n"+
			"  An empty value here would mean something released ownership that SPEC.md 8.7 does not permit to.",
			obs.ownerBeforeKill, obs.ownerAfterKill)
	}
}

// TestRunResumesAfterCrash is SPEC.md 13.1.1: "the orchestrator is killed at any
// instant. In-flight runs resume."
func TestRunResumesAfterCrash(t *testing.T) {
	if obs.crashFinal != "DONE" {
		t.Fatalf("SPEC.md 13.1.1: the run must resume and finish; it reached %q", obs.crashFinal)
	}
	harness.Bool(t, "SPEC.md 5.5 L3: a cleanly finished run is DONE with a DONE last step",
		"SELECT (SELECT status FROM steps WHERE run_id = :'run' ORDER BY seq DESC LIMIT 1) = 'DONE' "+
			"AND (SELECT status FROM runs WHERE run_id = :'run') = 'DONE';",
		harness.Var(obs.crashRun))

	// Every static step of the workflow ran, including the one after the crash.
	specs, err := harness.StaticSteps("workflow-survives-crash.json")
	if err != nil {
		t.Fatal(err)
	}
	harness.Bool(t,
		fmt.Sprintf("SPEC.md 13.1.1: the run continued past the crash - all %d static steps exist and are DONE",
			len(specs)),
		fmt.Sprintf("SELECT count(*) = %d AND bool_and(status = 'DONE') FROM steps WHERE run_id = :'run';",
			len(specs)),
		harness.Var(obs.crashRun))
}

// TestCompletedStepsNeverReRun is the second half of SPEC.md 13.1.1. The digest
// covers everything a re-run would disturb: the step's status, its
// attempt_count, its completed_at, the hash of its output, and the id and status
// of every attempt it owns.
//
// SPEC.md 13.2.1 permits duplicate execution of a step that was IN FLIGHT. It
// permits nothing at all for a step that had already completed.
func TestCompletedStepsNeverReRun(t *testing.T) {
	if obs.doneStepBefore == "" {
		t.Fatal("the fixture recorded no completed step before the crash; " +
			"SPEC.md 4.2 creates steps one at a time, so a second step's attempt implies the first is DONE")
	}
	if obs.doneStepAfter != obs.doneStepBefore {
		t.Errorf("SPEC.md 13.1.1: completed steps never re-run.\n"+
			"  before the crash: %s\n  after recovery:   %s\n"+
			"  (status|attempt_count|completed_at|md5(output)|attempt_id:status,...)",
			obs.doneStepBefore, obs.doneStepAfter)
	}
}

// TestTheInFlightAttemptIsOrphanedAndReplaced covers three ratified rules that
// meet on one row.
//
//	SPEC.md 8.6, claim time: a sync attempt "may be expired immediately,
//	regardless of deadline_at. Its HTTP connection died with its previous owner,
//	so no report can ever arrive."
//	SPEC.md 5.3: orphaned is "timeout, where the attempt's dispatching
//	orchestrator was not live when the attempt was expired".
//	SPEC.md 6.4: dispatched_by is the orchestrator_id that dispatched it - so the
//	replacement attempt must name a different process from the one that died.
func TestTheInFlightAttemptIsOrphanedAndReplaced(t *testing.T) {
	v := harness.Var(obs.crashRun)

	harness.Bool(t, "SPEC.md 8.6: the attempt that was in flight at the crash was expired",
		"SELECT count(*) >= 2 FROM attempts a JOIN steps s ON s.step_id = a.step_id "+
			"WHERE a.run_id = :'run' AND s.step_name = 'during-the-crash';", v)

	harness.Bool(t, "SPEC.md 5.3: its failure_reason is orphaned - the dispatching orchestrator was not live",
		"SELECT a.status = 'FAILED' AND a.failure_reason = 'orphaned' "+
			"FROM attempts a JOIN steps s ON s.step_id = a.step_id "+
			"WHERE a.run_id = :'run' AND s.step_name = 'during-the-crash' AND a.attempt_no = 1;", v)

	// SPEC.md 5.3 stresses that timeout and orphaned are decided by whether the
	// dispatcher was live, not by the clock alone. The attempt was killed far
	// inside step_timeout_seconds, so plain timeout would be the wrong label.
	harness.Bool(t, "SPEC.md 8.6: it was expired immediately, well inside its deadline",
		"SELECT a.finished_at < a.deadline_at "+
			"FROM attempts a JOIN steps s ON s.step_id = a.step_id "+
			"WHERE a.run_id = :'run' AND s.step_name = 'during-the-crash' AND a.attempt_no = 1;", v)

	dispatcher, err := harness.PSQL(
		"SELECT a.dispatched_by FROM attempts a JOIN steps s ON s.step_id = a.step_id "+
			"WHERE a.run_id = :'run' AND s.step_name = 'during-the-crash' AND a.attempt_no = 2;", v)
	if err != nil {
		t.Fatal(err)
	}
	if dispatcher == "" {
		t.Fatal("SPEC.md 13.1.1: no second attempt was dispatched, so the run did not resume")
	}
	if dispatcher == obs.dispatcherBeforeKill {
		t.Errorf("SPEC.md 6.4: the replacement attempt names the process that died.\n"+
			"  attempt 1 dispatched_by: %s\n  attempt 2 dispatched_by: %s\n"+
			"  A restarted orchestrator takes a fresh orchestrator_id, so these must differ.",
			obs.dispatcherBeforeKill, dispatcher)
	}
}

// ---------------------------------------------------------------------------
// Leg 2 - recovery never auto-replays a DLQ'd run (SPEC.md 13.2.7)
// ---------------------------------------------------------------------------

// TestDLQContentUntouchedByRecovery is the half of beta that could not be shown
// until gamma existed - which is why SPEC.md 18 orders gamma before beta.
//
// The DLQ'd run was created before the crash of leg 1 and digested either side
// of it: the run row, every step, every attempt, and every dead-letter entry.
// SPEC.md 13.2.7 makes reaching DLQ a decision a human must undo, and SPEC.md
// 8.6 is the mechanism - the sweep filters on status = 'RUNNING', so a DLQ run
// is never claimed and no driver ever looks at it again.
func TestDLQContentUntouchedByRecovery(t *testing.T) {
	if obs.dlqDigestBefore == "" {
		t.Fatal("the fixture recorded no digest before the crash")
	}
	if obs.dlqDigestAfter != obs.dlqDigestBefore {
		t.Errorf("SPEC.md 13.2.7: recovery never auto-replays a DLQ'd run, so nothing about it may change.\n"+
			"  digest before the crash: %s\n  digest after recovery:   %s",
			obs.dlqDigestBefore, obs.dlqDigestAfter)
	}

	v := harness.Var(obs.dlqRun)
	harness.Bool(t, "SPEC.md 5.5 L4: the run is still in worker-side DLQ",
		"SELECT status = 'DLQ' FROM runs WHERE run_id = :'run';", v)
	harness.Bool(t, "SPEC.md 8.6: a DLQ run is never claimed, so it holds no owner",
		"SELECT owner_id IS NULL AND claimed_at IS NULL FROM runs WHERE run_id = :'run';", v)
	harness.Bool(t, "SPEC.md 14 is milestone delta: recovery did not replay it",
		"SELECT replay_count = 0 FROM runs WHERE run_id = :'run';", v)
	harness.Bool(t, "SPEC.md 12.4: still exactly one dead-letter entry - recovery added none",
		"SELECT count(*) = 1 FROM dead_letter_queue WHERE run_id = :'run';", v)
}

// ---------------------------------------------------------------------------
// Leg 3 - a clean shutdown releases (SPEC.md 8.7)
// ---------------------------------------------------------------------------

// TestCleanShutdownReleasesOwnership is the contrast that makes leg 1 legible.
// SPEC.md 8.7: "on a clean shutdown the orchestrator releases ... This is an
// optimisation that makes failover immediate rather than lease_ttl later;
// correctness does not depend on it."
//
// The read happens after the process has exited and before anything is started
// again, so no wait is involved: either the shutdown released, or it did not.
func TestCleanShutdownReleasesOwnership(t *testing.T) {
	if obs.ownerAfterCleanStop != "" {
		t.Errorf("SPEC.md 8.7: a clean shutdown releases ownership, so owner_id must already be NULL "+
			"once the process has exited; it still held %q.\n"+
			"  Compare leg 1, where a SIGKILL correctly left ownership in place.",
			obs.ownerAfterCleanStop)
	}
	if obs.cleanFinal != "DONE" {
		t.Errorf("SPEC.md 13.1.1: the released run must be picked up again and finish; it reached %q",
			obs.cleanFinal)
	}
}

// ---------------------------------------------------------------------------
// Leg 4 - a crash loop converges (SPEC.md 13.1.4)
// ---------------------------------------------------------------------------

// TestCrashLoopConvergesToDLQ is SPEC.md 13.1.4 - "a crash loop. Each pass burns
// one unit of budget, so every in-flight step converges to DLQ (12.2)" - and
// SPEC.md 12.2's stronger claim that "unbounded retry is structurally
// impossible. Every dispatch increments a persisted counter BEFORE the work
// begins, so no crash afterwards - including one during recovery - can undo it."
//
// The step is killed on every attempt it is ever given. If a crash did not burn
// budget, this loop would not terminate.
func TestCrashLoopConvergesToDLQ(t *testing.T) {
	if obs.loopKills != maxAttempts {
		t.Fatalf("the fixture performed %d kills; the workflow's step_max_attempts is %d",
			obs.loopKills, maxAttempts)
	}
	if obs.loopFinal != "DLQ" {
		t.Fatalf("SPEC.md 13.1.4: a crash loop must converge to DLQ; the run reached %q", obs.loopFinal)
	}
	v := harness.Var(obs.loopRun)

	harness.Bool(t,
		fmt.Sprintf("SPEC.md 12.2: exactly %d attempts were dispatched - one per unit of budget, "+
			"burned at dispatch and never refunded by a crash", maxAttempts),
		fmt.Sprintf("SELECT count(*) = %d AND bool_and(status = 'FAILED') FROM attempts WHERE run_id = :'run';",
			maxAttempts),
		v)
	harness.Bool(t, "SPEC.md 5.3: every one of them is orphaned - each was expired with its dispatcher dead",
		"SELECT bool_and(failure_reason = 'orphaned') FROM attempts WHERE run_id = :'run';", v)
	harness.Bool(t,
		fmt.Sprintf("SPEC.md 12.2: the step is DLQ with attempt_count = %d", maxAttempts),
		fmt.Sprintf("SELECT status = 'DLQ' AND attempt_count = %d FROM steps WHERE run_id = :'run';",
			maxAttempts),
		v)
	harness.Bool(t, "SPEC.md 12.3 L4: one worker-side dead-letter entry naming the step",
		"SELECT count(*) = 1 FROM dead_letter_queue "+
			"WHERE run_id = :'run' AND reason = 'worker_budget_exhausted' AND step_id IS NOT NULL;", v)
	harness.Bool(t, "SPEC.md 8.7: the transition out of RUNNING released ownership in the same transaction",
		"SELECT owner_id IS NULL AND claimed_at IS NULL FROM runs WHERE run_id = :'run';", v)
}

// ---------------------------------------------------------------------------
// Leg 5 - storage unreachable at startup (SPEC.md 13.1.5)
// ---------------------------------------------------------------------------

// TestStorageUnreachableAtStartupFailsFast is SPEC.md 13.1.5: "fail fast:
// non-zero exit, and an error message that names storage as the cause".
//
// SPEC.md fixes the two observable properties and not the wording, so the
// assertion checks the exit code exactly and accepts any of the words that name
// storage as the cause. A fixed string would be inventing a contract no ruling
// covers (CLAUDE.md 9).
func TestStorageUnreachableAtStartupFailsFast(t *testing.T) {
	if obs.badBootExit == 0 {
		t.Errorf("SPEC.md 13.1.5: an orchestrator that cannot reach storage must exit non-zero; it exited 0.\n"+
			"  output:\n%s", obs.badBootOutput)
	}
	lower := strings.ToLower(obs.badBootOutput)
	named := false
	for _, word := range []string{"storage", "postgres", "database", "dsn"} {
		if strings.Contains(lower, word) {
			named = true
			break
		}
	}
	if !named {
		t.Errorf("SPEC.md 13.1.5: the error message must name storage as the cause.\n"+
			"  output:\n%s", obs.badBootOutput)
	}
}

// ---------------------------------------------------------------------------
// Across the legs
// ---------------------------------------------------------------------------

// TestEveryTerminalRunReleasedItsOwnership is SPEC.md 6.2's invariant - owner_id
// and claimed_at are non-NULL only while status = 'RUNNING' - which SPEC.md 8.7
// makes true "rather than merely asserted" by clearing them in the same
// transaction as the status change. Beta is where it is worth re-checking: five
// runs, three restarts of the process, and two different ways of stopping it.
func TestEveryTerminalRunReleasedItsOwnership(t *testing.T) {
	harness.Bool(t, "SPEC.md 6.2, 8.7: no run outside RUNNING holds owner_id or claimed_at",
		"SELECT count(*) = 0 FROM runs WHERE status <> 'RUNNING' "+
			"AND (owner_id IS NOT NULL OR claimed_at IS NOT NULL);")
}

// TestOrchestratorsRecordsEveryProcessAndOneIsLive covers SPEC.md 4.3 and 8.7.
// Each boot takes a fresh orchestrator_id, so the table accumulates a row per
// process - and exactly one of them is live, by the definition SPEC.md 8.7
// gives: last_seen_at > now() - lease_ttl.
//
// The dead rows are not a defect: BACKLOG.md B9 records that cleaning them up is
// pure disk housekeeping, and SPEC.md 5.3 needs them, because "was its owner
// alive?" is answered against this table long after the process is gone.
func TestOrchestratorsRecordsEveryProcessAndOneIsLive(t *testing.T) {
	ttl, err := harness.LeaseTTL()
	if err != nil {
		t.Fatal(err)
	}
	harness.Bool(t, "SPEC.md 4.3: every process that ran left a row - beta restarted the orchestrator several times",
		"SELECT count(*) > 1 FROM orchestrators;")
	harness.Bool(t,
		fmt.Sprintf("SPEC.md 8.7: exactly one orchestrator is live (last_seen_at > now() - %s)", ttl),
		fmt.Sprintf("SELECT count(*) = 1 FROM orchestrators WHERE last_seen_at > now() - interval '%d seconds';",
			int(ttl.Seconds())))
}

// TestWhatBetaDoesNotDemonstrate asserts the absence of what belongs to other
// milestones, where that absence is observable.
func TestWhatBetaDoesNotDemonstrate(t *testing.T) {
	harness.Bool(t, "SPEC.md 14 is milestone delta: recovery replayed nothing",
		"SELECT bool_and(replay_count = 0) FROM runs;")
	harness.Bool(t, "SPEC.md 15 is milestone iota: nothing was cancelled",
		"SELECT count(*) = 0 FROM runs WHERE status = 'CANCELLED';")
	harness.Bool(t, "SPEC.md 12.1: the static planner cannot fail, so no planner budget was consumed",
		"SELECT bool_and(planner_attempt_count = 0) FROM runs;")
	harness.Bool(t, "SPEC.md 9.7: sync + envelope is still the only combination in use",
		"SELECT bool_and(connection_mode = 'sync') FROM attempts;")
}
