-- The inverse of 000001_initial_schema.up.sql.
--
-- Dropped in reverse dependency order rather than with CASCADE, so that a
-- forgotten reference fails here instead of taking an unnamed object with it.

BEGIN;

DROP TABLE IF EXISTS orchestrators;
DROP TABLE IF EXISTS dead_letter_queue;
DROP TABLE IF EXISTS attempts;
DROP TABLE IF EXISTS steps;
DROP TABLE IF EXISTS runs;
DROP TABLE IF EXISTS workflows;

COMMIT;
