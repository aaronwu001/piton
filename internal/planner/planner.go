// Package planner holds the built-in static planner of SPEC.md 6.1.
//
// SPEC.md 3.1: "the planner is a pure function: one request in, one decision
// out. It holds no state between calls." The static planner is that sentence
// taken literally — its decision is a function of the run's step count and the
// workflow's array, and of nothing else.
package planner

import (
	"encoding/json"
	"fmt"

	"github.com/aaronwu001/piton/internal/model"
)

// The three answers of SPEC.md 9.3, and the only three there are.
const (
	StatusContinue = "continue"
	StatusDone     = "done"
	StatusFail     = "fail"
)

// Decision is SPEC.md 9.3's response. SPEC.md 9.3: "StepSpec is one field of
// the response, not the response itself", because done and fail carry no step
// and "a response whose shape changes with its meaning is the kind of ambiguity
// that turned planner-response validation into the previous project's largest
// time sink".
type Decision struct {
	Status string

	// Step is the StepSpec bytes, present only when Status is continue. They
	// are carried unparsed so that SPEC.md 6.3 can store "the StepSpec exactly
	// as the planner returned it".
	Step json.RawMessage

	// Reason is present only when Status is fail (SPEC.md 9.3).
	Reason string
}

// Static is SPEC.md 6.1's built-in planner: "asked for a decision, it answers
// with planner_static_steps[n] where n is the number of steps the run already
// has, and answers done once n has reached the end of the array. It never
// answers fail."
//
// SPEC.md 12.1 states what follows and asks for it to be relied on rather than
// special-cased: "the static planner simply cannot fail at run time — SPEC.md
// 6.1 validates its steps at submission, and it holds no state and makes no
// network call, so planner_attempt_count never leaves 0". The error return
// below is therefore not a failure mode of the planner; it is the report of a
// workflow row that could not have been written by POST /workflows.
func Static(wf *model.Workflow, stepCount int) (*Decision, error) {
	if wf.PlannerType != model.PlannerStatic {
		return nil, fmt.Errorf("planner: workflow %s is not %q", wf.WorkflowID, model.PlannerStatic)
	}
	var steps []json.RawMessage
	if err := json.Unmarshal(wf.PlannerStaticSteps, &steps); err != nil {
		return nil, fmt.Errorf("planner: workflow %s has unreadable planner_static_steps: %w",
			wf.WorkflowID, err)
	}
	if stepCount < 0 || stepCount > len(steps) {
		return nil, fmt.Errorf("planner: workflow %s has %d static steps but the run has %d",
			wf.WorkflowID, len(steps), stepCount)
	}
	if stepCount == len(steps) {
		return &Decision{Status: StatusDone}, nil
	}
	return &Decision{Status: StatusContinue, Step: steps[stepCount]}, nil
}
