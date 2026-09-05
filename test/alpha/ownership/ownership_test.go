package ownership

import (
	"fmt"
	"testing"
	"time"

	"github.com/aaronwu001/piton/test/alpha/harness"
)

// ---------------------------------------------------------------------------
// The orchestrator's own row
// ---------------------------------------------------------------------------

// SPEC.md 18.1: "exactly one row, recently seen". SPEC.md 6.6 explains why the
// count is exactly one here and not merely at least one: rows accumulate at one
// row per process boot and are never deleted, and this group boots the
// orchestrator once against a database CLAUDE.md 5.5.2 guarantees is clean.
func TestExactlyOneOrchestratorRow(t *testing.T) {
	harness.Bool(t, "SPEC.md 18.1, 6.6: exactly one orchestrator row, this group having booted one process",
		"SELECT count(*) = 1 FROM orchestrators;")
	harness.Bool(t, "SPEC.md 6.6: both timestamps are present on that row",
		`SELECT count(*) = 1 FROM orchestrators
           WHERE started_at IS NOT NULL AND last_seen_at IS NOT NULL;`)
}

// SPEC.md 4.3 and 8.7 define liveness identically and mechanically: an
// orchestrator is live iff last_seen_at > now() - lease_ttl. The TTL comes from
// demos/alpha/piton.yaml, the file the orchestrator booted on, so this
// assertion cannot disagree with the process it is asserting about.
func TestOrchestratorIsLive(t *testing.T) {
	harness.Bool(t,
		fmt.Sprintf("SPEC.md 4.3, 8.7: the orchestrator is live - last_seen_at within lease_ttl (%ds)", leaseTTL),
		fmt.Sprintf(`SELECT count(*) = 1 FROM orchestrators
                       WHERE last_seen_at > now() - interval '%d seconds';`, leaseTTL))
}

// SPEC.md 8.7: the orchestrator writes one heartbeat row every heartbeat
// interval, and last_seen_at is "the only column a heartbeat touches"
// (SPEC.md 6.6).
//
// This is the one assertion in the suite that must spend real time, because the
// property being asserted is that something happens repeatedly rather than
// once. A single reading cannot distinguish a heartbeat from a value written at
// boot and never touched again - and that difference is exactly what liveness
// means: SPEC.md 4.3 calls renewal "a liveness signal, not a progress signal",
// and the run this group created has long since finished, so nothing but the
// heartbeat itself can move the column.
//
// The wait is one interval plus half of one. A tighter margin would be
// measuring the scheduler rather than the rule.
func TestHeartbeatAdvancesLastSeenAt(t *testing.T) {
	before := harness.Scalar(t, "SELECT last_seen_at FROM orchestrators;")
	if before == "" {
		t.Fatal("SPEC.md 6.6: no orchestrator row to observe")
	}

	wait := time.Duration(heartbeatInterval)*time.Second + time.Duration(heartbeatInterval)*time.Second/2
	t.Logf("waiting %s for at least one heartbeat (heartbeat_interval_seconds = %d)", wait, heartbeatInterval)
	time.Sleep(wait)

	harness.Bool(t, "SPEC.md 8.7: last_seen_at advanced, so the process is heartbeating and not merely registered",
		"SELECT last_seen_at > (:'before')::timestamptz FROM orchestrators;",
		"before="+before)
	harness.Bool(t, "SPEC.md 6.6: the heartbeat touched last_seen_at and did not add a row",
		"SELECT count(*) = 1 FROM orchestrators;")
}

// ---------------------------------------------------------------------------
// What a terminal run may no longer hold
// ---------------------------------------------------------------------------

// SPEC.md 6.2's runs invariants: "owner_id and claimed_at are non-NULL only
// while status = 'RUNNING', and are always written and cleared as a pair".
//
// SPEC.md 8.7 names the mechanism that makes this true rather than merely
// asserted. Coordination metadata is written in exactly four places, the fourth
// being any transition of a run out of RUNNING, which clears the pair in the
// same transaction as the status change. This run's transition was to DONE; the
// same rule covers DLQ and CANCELLED, which alpha does not demonstrate.
func TestTerminalRunHoldsNoCoordinationMetadata(t *testing.T) {
	if finalStatus == "RUNNING" || finalStatus == "" {
		t.Fatalf("the fixture run is %q; this rule is about a run that has left RUNNING", finalStatus)
	}
	harness.Bool(t, "SPEC.md 6.2, 8.7: a run that has left RUNNING holds no owner_id",
		"SELECT owner_id IS NULL FROM runs WHERE run_id = :'run';", rv())
	harness.Bool(t, "SPEC.md 6.2, 8.7: it holds no claimed_at either - the pair is cleared together",
		"SELECT claimed_at IS NULL FROM runs WHERE run_id = :'run';", rv())
	harness.Bool(t, "SPEC.md 6.2, 8.7: no run in this database violates the invariant",
		`SELECT count(*) = 0 FROM runs
           WHERE status <> 'RUNNING'
             AND (owner_id IS NOT NULL OR claimed_at IS NOT NULL);`)
}

// SPEC.md 6.4: attempts.dispatched_by is "the orchestrator_id that dispatched
// it". With one live orchestrator, every attempt must carry that one id - and
// the column must not be a placeholder, which is what this catches.
func TestEveryAttemptWasDispatchedByTheLiveOrchestrator(t *testing.T) {
	total := harness.Int(t, "SELECT count(*) FROM attempts WHERE run_id = :'run';", rv())
	if total == 0 {
		t.Fatal("SPEC.md 18.1: the run produced no attempts at all")
	}
	harness.Bool(t, "SPEC.md 6.4: every attempt names the one orchestrator that dispatched it",
		fmt.Sprintf(`SELECT count(*) FILTER (
                            WHERE dispatched_by = (SELECT orchestrator_id FROM orchestrators)) = %d
                       FROM attempts WHERE run_id = :'run';`, total), rv())
}
