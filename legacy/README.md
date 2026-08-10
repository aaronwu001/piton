# legacy/ — VOID

Everything in this directory belonged to **StateFlow**, the project Piton replaces. None of it is
authoritative. None of it describes Piton.

**Read it for "how they thought". Never for "so we do the same."**

If a rule matters, it is in `SPEC.md`. If you find something here that looks load-bearing and has
no counterpart in `SPEC.md`, that is a question for the owner — not a licence to implement it.

## Why these documents were abandoned

The technical content was not the defect. The structure was.

- **Four separate registries** answered "is X guaranteed?" — whitepaper §18 (Temporary Design
  Registry), `BEHAVIOR_MATRIX` §K (explicit non-guarantees), `BEHAVIOR_MATRIX` §M (rulings), and
  `BACKLOG.md` — plus §21 (Roadmap). One question, four possible answers.
- **The API surface was stated in full in four places** (whitepaper §12–13, `CLAUDE-old.md`,
  `README.md`, `USER_MANUAL.md`), and they had drifted apart.
- Documents declaring themselves authoritative contradicted each other — `CLAUDE-old.md` and
  `BEHAVIOR_MATRIX` disagreed on where `retry_limit` lived.
- `WHITEPAPER_V1_1_PATCHES.md` was a patch list that was never merged.
- Every document pointed at `spec/BEHAVIOR_MATRIX.md` as the authority; that path did not exist.

Piton's document set is built to make this impossible: one authority (`SPEC.md`), and everything
else explicitly forbidden from being cited as justification.

## Contents

| File | What it was |
|---|---|
| `StateFlow_Whitepaper_v1_0.md` | Self-declared authoritative design; lagged the implementation in roughly a dozen places |
| `WHITEPAPER_V1_1_PATCHES.md` | Patches to the above; never merged |
| `BEHAVIOR_MATRIX.md` | ~150 assertion rows, self-declared to win all conflicts |
| `StateFlow_Rules_Consolidation_v3_EN.md` | Former authority, demoted to reference, then stale |
| `CLAUDE-old.md` | Development discipline for the old project |
| `BACKLOG.md` | The old backlog |
| `OPERATIONAL_FACTS.md` | How to run the old system; environment-specific |
| `USER_MANUAL.md` | User-facing documentation |
| `README.md` | Public README |
| `HANDOFF_23AB.md` | A session handoff template, with its placeholders never filled in |

The old implementation itself lives at `~/Projects/StateFlow` and is reference-only under the rules
in `CLAUDE.md § 7`.
