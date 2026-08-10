# Fencing and CAS — the same primitive, two predicates

*Explanatory. `SPEC.md § 8` is the authority.*

This note exists because "fencing" and "CAS" sound like two mechanisms, and reading the rules as if
they were two mechanisms makes the design harder than it is.

---

## The behaviour, stated plainly first

A driver checks whether the run is still its own. If it is not, it walks away — aborts the
transaction, stops the goroutine, dispatches nothing further, tells nobody.

That is the whole of it. Earlier drafts wrote this as "every transaction **asserts** it still holds
the run", which reads as *claiming* possession rather than *testing* it. It is a test.

---

## Why the check cannot be a separate step

The obvious implementation has a hole in it:

```sql
SELECT owner_id FROM runs WHERE run_id = :rid;   -- (1) still mine, good
                                                 -- (2) ← someone else claims it here
UPDATE steps SET status = 'DONE' WHERE ...;      -- (3) no longer entitled, but the write lands
```

This is TOCTOU — time-of-check to time-of-use. **A check passing does not mean it is still true
when you write.**

So the requirement is not "check". It is: *check and write, with no window in which the answer can
change.* There are two ways to get that.

**(A) Put the condition inside the writing statement.**

```sql
UPDATE steps SET status = 'DONE', output = :out
WHERE step_id = :sid
  AND EXISTS (SELECT 1 FROM runs WHERE run_id = :rid AND owner_id = :me);
```

No window. But a business transaction touches `steps`, `attempts` and `runs`, so the predicate has
to be repeated in every statement — noisy, and **one forgotten statement silently breaks the whole
guarantee**.

**(B) Take a row lock once, at the top of the transaction.** This is what Piton does.

```sql
BEGIN;
SELECT 1 FROM runs WHERE run_id = :rid AND owner_id = :me FOR UPDATE;
-- 0 rows  ⇒ walk away (ROLLBACK, the goroutine ends)
-- 1 row   ⇒ that row is now locked; a concurrent claim blocks until we commit or abort
...  write as many tables as the transaction needs, all under the same protection  ...
COMMIT;
```

**One statement protects the whole transaction**, and the protection comes from the database rather
than from the author remembering to repeat something.

---

## The two terms

**CAS (compare-and-set)** is the general primitive: a write whose effect is conditional on the
current state matching an expectation, evaluated atomically with the write. In SQL that is
`UPDATE … WHERE <expected state>`, and *zero rows affected* means the expectation was wrong.

**Fencing** is *a specific use* of that primitive, where the condition is about **ownership**. The
name comes from the distributed-systems "fencing token": the job of a fence is to keep a superseded
process out.

**So fencing is CAS.** Same mechanism, different predicate. Piton has exactly two predicates:

| | **Ownership fence** | **Attempt CAS** |
|---|---|---|
| Question it asks | "Am I still this run's owner?" | "Has this attempt's outcome not been written yet?" |
| Row it tests | the `runs` row | the `attempts` row |
| SQL | `WHERE run_id = :rid AND owner_id = :me` | `WHERE attempt_id = :aid AND status = 'RUNNING'` |
| What it prevents | two drivers making **decisions** about one run | one attempt's outcome being written twice, including by a late report |
| Meaning of zero rows | I am out — abort, stop, dispatch nothing | someone already recorded this — discard the report, reply 409 |
| Who must satisfy it | every write that constitutes a **decision** | every write of an attempt outcome, owner or not |

---

## Why there is one exception, and why it is safe

In async mode a worker POSTs its result to a callback URL. With more than one orchestrator replica
behind a load balancer, that POST can land on a replica that does not own the run.

If the ownership fence applied without exception, that replica would refuse the callback — and a
result the worker actually computed would be thrown away, the attempt would time out, and the work
would be done a second time for nothing.

So: **an async callback arriving at a non-owner checks only the second predicate, not the first.**

The line is not arbitrary. A report is a **fact about work that already happened**; a decision is
about **what the run should do next**. Facts may be recorded by whoever receives them; decisions
belong to the owner. And the exception is safe in a way you can check mechanically: the only row it
can touch is one that already admits exactly one winner.

It is also kept as small as it can be — the callback writes the `attempts` row and nothing else.
Whether a failed step retries or goes to the dead-letter queue depends on a budget check, and a
budget check is a decision, so the owner does it on its next poll of the attempt row.

---

## Why the owner polls a row instead of waiting on a channel

The previous project pushed callback results into an in-memory channel held by the driver
goroutine. On the wrong replica there is no channel, so the result was simply lost — the failure was
built into the structure.

Once the outcome is a row guarded by CAS, any replica can accept any callback and the database
arbitrates. The driver awaiting an async result therefore polls the attempt row rather than blocking
on a channel. That single change deletes the channel registry, the rule that a firing timer must
scrub that registry, the process-level enforcement of "single writer", and the wrong-replica loss —
all at once.

This is `SPEC.md § 1` doing its job: the database is the coordination mechanism, and the in-memory
structure was only ever a cache.
