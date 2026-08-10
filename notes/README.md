# notes/

**These documents explain. They never govern.**

`SPEC.md` says what the system must do. These notes say why, what the alternatives were, and how to
think about a mechanism the first time you meet it. The split is by **voice**:

| Goes in `SPEC.md` | Goes in `notes/` |
|---|---|
| Which fields exist, their types, whether they are required | Why a field exists, and what was eliminated to arrive at it |
| Which fields exist in which mode | A worked example of one message travelling end to end |
| The rule, plus a one-line `Why:` | The long derivation behind that one line |
| The state table | A walkthrough of how a run moves through it |

The test: **a field table is a MUST — SPEC. A walkthrough that aids understanding — notes.**

## The rule that makes this safe

**Nothing in `notes/` may ever be cited as justification for a behaviour.** If a behaviour is real,
it is in `SPEC.md`. If you find yourself reaching for a note to settle an argument about what the
system does, the note is not the answer — the missing SPEC rule is, and it has to be ruled on.

This is the same rule that governs `GRILLING_LOG.md`, and it exists because the previous project
died of having four documents that could each answer the same question differently.

## Contents

| File | What it explains |
|---|---|
| `concurrency-control.md` | What fencing and CAS are, why they are the same primitive, and why the check cannot be a separate step |
